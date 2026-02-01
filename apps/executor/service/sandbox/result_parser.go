package sandbox

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/antinvestor/builder/internal/events"
)

// TestResultParser parses test output from various test frameworks.
type TestResultParser struct{}

// NewTestResultParser creates a new result parser.
func NewTestResultParser() *TestResultParser {
	return &TestResultParser{}
}

// ParseResults parses test output and exit code into structured results.
func (p *TestResultParser) ParseResults(output string, exitCode int, language string) *events.TestResult {
	result := &events.TestResult{
		Success:    exitCode == 0,
		DurationMs: 0,
		TestCases:  []events.TestCaseResult{},
	}

	// Parse based on language/framework
	switch strings.ToLower(language) {
	case "go":
		p.parseGoTestOutput(output, result)
	case "python":
		p.parsePytestOutput(output, result)
	case "node", "javascript", "typescript":
		p.parseJestOutput(output, result)
	case "java":
		p.parseMavenOutput(output, result)
	case "rust":
		p.parseCargoOutput(output, result)
	default:
		// Generic parsing
		p.parseGenericOutput(output, exitCode, result)
	}

	// Ensure counts are consistent
	if result.TotalTests == 0 && exitCode == 0 {
		result.TotalTests = 1
		result.PassedTests = 1
	}

	return result
}

// parseGoTestOutput parses output from `go test`.
func (p *TestResultParser) parseGoTestOutput(output string, result *events.TestResult) {
	// Parse summary line: ok/FAIL package_name duration
	// Example: "ok  	github.com/pkg/name	0.123s"
	// Example: "FAIL	github.com/pkg/name	0.456s"

	passPattern := regexp.MustCompile(`(?m)^ok\s+\S+\s+([\d.]+)s`)
	failPattern := regexp.MustCompile(`(?m)^FAIL\s+\S+\s+([\d.]+)s`)

	passMatches := passPattern.FindAllStringSubmatch(output, -1)
	failMatches := failPattern.FindAllStringSubmatch(output, -1)

	result.PassedTests = len(passMatches)
	result.FailedTests = len(failMatches)
	result.TotalTests = result.PassedTests + result.FailedTests

	// Parse individual test cases
	// "--- PASS: TestName (0.00s)"
	// "--- FAIL: TestName (0.00s)"
	testPassPattern := regexp.MustCompile(`(?m)^--- PASS: (\S+)\s+\(([\d.]+)s\)`)
	testFailPattern := regexp.MustCompile(`(?m)^--- FAIL: (\S+)\s+\(([\d.]+)s\)`)

	for _, match := range testPassPattern.FindAllStringSubmatch(output, -1) {
		duration := parseDuration(match[2])
		result.TestCases = append(result.TestCases, events.TestCaseResult{
			Name:       match[1],
			Status:     string(events.TestStatusPassed),
			DurationMs: duration,
		})
	}

	for _, match := range testFailPattern.FindAllStringSubmatch(output, -1) {
		duration := parseDuration(match[2])

		// Try to extract failure message
		failMsg := extractGoTestFailure(output, match[1])

		result.TestCases = append(result.TestCases, events.TestCaseResult{
			Name:       match[1],
			Status:     string(events.TestStatusFailed),
			DurationMs: duration,
			Error:      failMsg,
		})
	}

	// Parse coverage if present
	// "coverage: 85.5% of statements"
	coveragePattern := regexp.MustCompile(`coverage:\s+([\d.]+)%`)
	if matches := coveragePattern.FindStringSubmatch(output); len(matches) >= 2 {
		if coverage, err := strconv.ParseFloat(matches[1], 64); err == nil {
			result.Coverage = coverage
		}
	}
}

// parsePytestOutput parses output from pytest.
func (p *TestResultParser) parsePytestOutput(output string, result *events.TestResult) {
	// Parse summary: "5 passed, 2 failed, 1 skipped in 1.23s"
	summaryPattern := regexp.MustCompile(`(\d+)\s+passed(?:.*?(\d+)\s+failed)?(?:.*?(\d+)\s+skipped)?.*?in\s+([\d.]+)s`)
	if matches := summaryPattern.FindStringSubmatch(output); len(matches) >= 2 {
		result.PassedTests = parseInt(matches[1])
		if len(matches) >= 3 && matches[2] != "" {
			result.FailedTests = parseInt(matches[2])
		}
		if len(matches) >= 4 && matches[3] != "" {
			result.SkippedTests = parseInt(matches[3])
		}
		if len(matches) >= 5 {
			result.DurationMs = parseDuration(matches[4])
		}
		result.TotalTests = result.PassedTests + result.FailedTests + result.SkippedTests
	}

	// Parse individual test cases
	// "test_file.py::test_name PASSED"
	// "test_file.py::test_name FAILED"
	testPattern := regexp.MustCompile(`(?m)^(\S+::\S+)\s+(PASSED|FAILED|SKIPPED|ERROR)`)
	for _, match := range testPattern.FindAllStringSubmatch(output, -1) {
		status := string(events.TestStatusPassed)
		switch match[2] {
		case "FAILED", "ERROR":
			status = string(events.TestStatusFailed)
		case "SKIPPED":
			status = string(events.TestStatusSkipped)
		}

		result.TestCases = append(result.TestCases, events.TestCaseResult{
			Name:   match[1],
			Status: status,
		})
	}

	// Parse coverage if present
	// "TOTAL ... 85%"
	coveragePattern := regexp.MustCompile(`TOTAL\s+\d+\s+\d+\s+(\d+)%`)
	if matches := coveragePattern.FindStringSubmatch(output); len(matches) >= 2 {
		if coverage, err := strconv.ParseFloat(matches[1], 64); err == nil {
			result.Coverage = coverage
		}
	}
}

// parseJestOutput parses output from Jest (Node.js).
func (p *TestResultParser) parseJestOutput(output string, result *events.TestResult) {
	// Parse summary: "Tests: 5 passed, 2 failed, 7 total"
	summaryPattern := regexp.MustCompile(`Tests:\s+(\d+)\s+passed(?:,\s+(\d+)\s+failed)?(?:,\s+(\d+)\s+skipped)?`)
	if matches := summaryPattern.FindStringSubmatch(output); len(matches) >= 2 {
		result.PassedTests = parseInt(matches[1])
		if len(matches) >= 3 && matches[2] != "" {
			result.FailedTests = parseInt(matches[2])
		}
		if len(matches) >= 4 && matches[3] != "" {
			result.SkippedTests = parseInt(matches[3])
		}
		result.TotalTests = result.PassedTests + result.FailedTests + result.SkippedTests
	}

	// Parse individual test results
	// "✓ test name (123 ms)"
	// "✕ test name (123 ms)"
	passPattern := regexp.MustCompile(`(?m)^\s*[✓✔]\s+(.+?)\s+\((\d+)\s*m?s?\)`)
	failPattern := regexp.MustCompile(`(?m)^\s*[✕✗]\s+(.+?)\s+\((\d+)\s*m?s?\)`)

	for _, match := range passPattern.FindAllStringSubmatch(output, -1) {
		duration := parseInt(match[2])
		result.TestCases = append(result.TestCases, events.TestCaseResult{
			Name:       match[1],
			Status:     string(events.TestStatusPassed),
			DurationMs: int64(duration),
		})
	}

	for _, match := range failPattern.FindAllStringSubmatch(output, -1) {
		duration := parseInt(match[2])
		result.TestCases = append(result.TestCases, events.TestCaseResult{
			Name:       match[1],
			Status:     string(events.TestStatusFailed),
			DurationMs: int64(duration),
		})
	}

	// Parse coverage: "All files | 85.5 | 70 | 85.5 | 85.5 |"
	coveragePattern := regexp.MustCompile(`All files\s*\|\s*([\d.]+)`)
	if matches := coveragePattern.FindStringSubmatch(output); len(matches) >= 2 {
		if coverage, err := strconv.ParseFloat(matches[1], 64); err == nil {
			result.Coverage = coverage
		}
	}
}

// parseMavenOutput parses output from Maven.
func (p *TestResultParser) parseMavenOutput(output string, result *events.TestResult) {
	// Parse summary: "Tests run: 10, Failures: 2, Errors: 0, Skipped: 1"
	summaryPattern := regexp.MustCompile(
		`Tests run:\s*(\d+),\s*Failures:\s*(\d+),\s*Errors:\s*(\d+),\s*Skipped:\s*(\d+)`,
	)
	if matches := summaryPattern.FindStringSubmatch(output); len(matches) >= 5 {
		total := parseInt(matches[1])
		failures := parseInt(matches[2])
		errors := parseInt(matches[3])
		skipped := parseInt(matches[4])

		result.TotalTests = total
		result.FailedTests = failures + errors
		result.SkippedTests = skipped
		result.PassedTests = total - result.FailedTests - skipped
	}
}

// parseCargoOutput parses output from Cargo (Rust).
func (p *TestResultParser) parseCargoOutput(output string, result *events.TestResult) {
	// Parse summary: "test result: ok. 5 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out"
	summaryPattern := regexp.MustCompile(`test result:.*?(\d+)\s+passed;\s+(\d+)\s+failed;\s+(\d+)\s+ignored`)
	if matches := summaryPattern.FindStringSubmatch(output); len(matches) >= 4 {
		result.PassedTests = parseInt(matches[1])
		result.FailedTests = parseInt(matches[2])
		result.SkippedTests = parseInt(matches[3])
		result.TotalTests = result.PassedTests + result.FailedTests + result.SkippedTests
	}

	// Parse individual tests
	// "test tests::test_name ... ok"
	// "test tests::test_name ... FAILED"
	cargoTestPattern := regexp.MustCompile(`(?m)^test\s+(\S+)\s+\.\.\.\s+(ok|FAILED|ignored)`)
	for _, match := range cargoTestPattern.FindAllStringSubmatch(output, -1) {
		status := string(events.TestStatusPassed)
		switch match[2] {
		case "FAILED":
			status = string(events.TestStatusFailed)
		case "ignored":
			status = string(events.TestStatusSkipped)
		}

		result.TestCases = append(result.TestCases, events.TestCaseResult{
			Name:   match[1],
			Status: status,
		})
	}
}

// parseGenericOutput provides generic parsing for unknown formats.
func (p *TestResultParser) parseGenericOutput(output string, exitCode int, result *events.TestResult) {
	// Look for common patterns

	// Count "pass", "fail", "error", "skip" keywords
	lowerOutput := strings.ToLower(output)

	passCount := strings.Count(lowerOutput, "passed")
	if passCount == 0 {
		passCount = strings.Count(lowerOutput, "pass:")
	}
	if passCount == 0 {
		passCount = strings.Count(lowerOutput, " ok")
	}

	failCount := strings.Count(lowerOutput, "failed")
	if failCount == 0 {
		failCount = strings.Count(lowerOutput, "failure")
	}
	if failCount == 0 {
		failCount = strings.Count(lowerOutput, " fail ")
	}

	skipCount := strings.Count(lowerOutput, "skipped")
	if skipCount == 0 {
		skipCount = strings.Count(lowerOutput, "skip:")
	}

	// If we found something, use it
	if passCount > 0 || failCount > 0 {
		result.PassedTests = passCount
		result.FailedTests = failCount
		result.SkippedTests = skipCount
		result.TotalTests = passCount + failCount + skipCount
	} else {
		// Default based on exit code
		if exitCode == 0 {
			result.TotalTests = 1
			result.PassedTests = 1
		} else {
			result.TotalTests = 1
			result.FailedTests = 1
		}
	}
}

// Helper functions

func parseInt(s string) int {
	if s == "" {
		return 0
	}
	n, _ := strconv.Atoi(s)
	return n
}

func parseDuration(s string) int64 {
	if s == "" {
		return 0
	}
	f, _ := strconv.ParseFloat(s, 64)
	return int64(f * 1000) // Convert to milliseconds
}

func extractGoTestFailure(output string, testName string) string {
	// Try to find the failure message after the test name
	lines := strings.Split(output, "\n")
	inTest := false
	var failureLines []string

	for _, line := range lines {
		if strings.Contains(line, "--- FAIL: "+testName) {
			inTest = true
			continue
		}
		if inTest {
			if strings.HasPrefix(line, "---") || strings.HasPrefix(line, "===") {
				break
			}
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				failureLines = append(failureLines, trimmed)
			}
			if len(failureLines) >= 5 {
				break
			}
		}
	}

	return strings.Join(failureLines, "\n")
}
