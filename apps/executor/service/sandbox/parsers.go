package sandbox

import (
	"bufio"
	"encoding/json"
	"encoding/xml"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/antinvestor/builder/internal/events"
)

// Language constants for supported test frameworks.
const (
	LanguageGo     = "go"
	LanguagePython = "python"
	LanguageNode   = "node"
	LanguageJava   = "java"
)

// Test status constants.
const (
	statusPassed  = "passed"
	statusFailed  = "failed"
	statusSkipped = "skipped"
)

// Time conversion constants.
const (
	msPerSecond     = 1000
	percentMultiple = 100
)

// Pre-compiled regular expressions for parsing test output.
// These are compiled once at package initialization for better performance.
var (
	// Go test output patterns.
	goTestPassRe = regexp.MustCompile(`--- PASS: (\S+) \(([0-9.]+)s\)`)
	goTestFailRe = regexp.MustCompile(`--- FAIL: (\S+) \(([0-9.]+)s\)`)
	goTestSkipRe = regexp.MustCompile(`--- SKIP: (\S+) \(([0-9.]+)s\)`)
	goCoverageRe = regexp.MustCompile(`coverage:\s*([0-9.]+)%`)

	// Pytest output patterns.
	pytestSummaryRe = regexp.MustCompile(
		`(\d+) passed(?:, (\d+) failed)?(?:, (\d+) skipped)?(?:, (\d+) error)? in ([0-9.]+)s`,
	)
	pytestPassedRe   = regexp.MustCompile(`PASSED\s+(\S+)`)
	pytestFailedRe   = regexp.MustCompile(`FAILED\s+(\S+)`)
	pytestSkippedRe  = regexp.MustCompile(`SKIPPED\s+(\S+)`)
	pytestCoverageRe = regexp.MustCompile(`TOTAL\s+\d+\s+\d+\s+(\d+)%`)

	// Jest output patterns.
	jestSummaryRe = regexp.MustCompile(
		`Tests:\s*(?:(\d+) passed)?(?:,\s*)?(?:(\d+) failed)?(?:,\s*)?(?:(\d+) skipped)?(?:,\s*)?(\d+) total`,
	)
	jestTimeRe     = regexp.MustCompile(`Time:\s*([0-9.]+)s`)
	jestCoverageRe = regexp.MustCompile(`All files\s*\|\s*([0-9.]+)`)
)

// TestResultParser parses test output from various frameworks.
type TestResultParser struct {
	coverageThreshold float64
}

// NewTestResultParser creates a new test result parser.
func NewTestResultParser(coverageThreshold float64) *TestResultParser {
	return &TestResultParser{
		coverageThreshold: coverageThreshold,
	}
}

// ParseTestOutput parses test output based on language.
func (p *TestResultParser) ParseTestOutput(
	language string,
	output string,
	exitCode int,
) (*events.TestResult, error) {
	var result *events.TestResult

	switch strings.ToLower(language) {
	case LanguageGo:
		result = p.parseGoTestOutput(output)
	case LanguagePython:
		result = p.parsePytestOutput(output)
	case LanguageNode:
		result = p.parseJestOutput(output)
	case LanguageJava:
		result = p.parseJUnitXML(output)
	default:
		// Fallback to generic parsing
		result = p.parseGenericOutput(output, exitCode)
	}

	// Override success based on exit code if not already failed
	if exitCode != 0 && result.Success {
		result.Success = false
	}

	return result, nil
}

// =============================================================================
// Go Test Parser
// =============================================================================

// goTestEvent represents a single event from go test -json output.
type goTestEvent struct {
	Time    time.Time `json:"Time"`
	Action  string    `json:"Action"`
	Package string    `json:"Package"`
	Test    string    `json:"Test"`
	Output  string    `json:"Output"`
	Elapsed float64   `json:"Elapsed"`
}

func (p *TestResultParser) parseGoTestOutput(output string) *events.TestResult {
	result := &events.TestResult{
		TestCases: []events.TestCaseResult{},
	}

	testResults := make(map[string]*events.TestCaseResult)
	var totalDuration float64

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()

		// Try to parse as JSON first (go test -json output)
		var event goTestEvent
		if err := json.Unmarshal([]byte(line), &event); err == nil {
			p.processGoTestEvent(&event, testResults, &totalDuration)
			continue
		}

		// Fallback: try to parse traditional go test output
		p.parseGoTestLine(line, result)
	}

	// Aggregate results from JSON parsing
	for _, tc := range testResults {
		result.TestCases = append(result.TestCases, *tc)
		result.TotalTests++
		switch tc.Status {
		case statusPassed:
			result.PassedTests++
		case statusFailed:
			result.FailedTests++
		case statusSkipped:
			result.SkippedTests++
		}
	}

	// Parse coverage from output
	coverage := p.parseGoCoverage(output)
	result.Coverage = coverage

	result.DurationMs = int64(totalDuration * msPerSecond)
	result.Success = result.FailedTests == 0

	return result
}

func (p *TestResultParser) processGoTestEvent(
	event *goTestEvent,
	results map[string]*events.TestCaseResult,
	totalDuration *float64,
) {
	if event.Test == "" {
		// Package-level event
		if event.Action == "pass" || event.Action == "fail" {
			*totalDuration += event.Elapsed
		}
		return
	}

	key := event.Package + "/" + event.Test
	tc, exists := results[key]
	if !exists {
		tc = &events.TestCaseResult{
			Name:  event.Test,
			Suite: event.Package,
		}
		results[key] = tc
	}

	switch event.Action {
	case "pass":
		tc.Status = statusPassed
		tc.DurationMs = int64(event.Elapsed * msPerSecond)
	case "fail":
		tc.Status = statusFailed
		tc.DurationMs = int64(event.Elapsed * msPerSecond)
	case "skip":
		tc.Status = statusSkipped
	case "output":
		if tc.Status == statusFailed && event.Output != "" {
			tc.Output += event.Output
		}
	}
}

func (p *TestResultParser) parseGoTestLine(line string, result *events.TestResult) {
	// Parse traditional go test output format
	// --- PASS: TestName (0.00s)
	// --- FAIL: TestName (0.00s)
	// --- SKIP: TestName (0.00s)
	if match := goTestPassRe.FindStringSubmatch(line); match != nil {
		duration, _ := strconv.ParseFloat(match[2], 64)
		result.TestCases = append(result.TestCases, events.TestCaseResult{
			Name:       match[1],
			Status:     statusPassed,
			DurationMs: int64(duration * msPerSecond),
		})
		result.TotalTests++
		result.PassedTests++
		return
	}

	if matchFail := goTestFailRe.FindStringSubmatch(line); matchFail != nil {
		duration, _ := strconv.ParseFloat(matchFail[2], 64)
		result.TestCases = append(result.TestCases, events.TestCaseResult{
			Name:       matchFail[1],
			Status:     statusFailed,
			DurationMs: int64(duration * msPerSecond),
		})
		result.TotalTests++
		result.FailedTests++
		return
	}

	if matchSkip := goTestSkipRe.FindStringSubmatch(line); matchSkip != nil {
		duration, _ := strconv.ParseFloat(matchSkip[2], 64)
		result.TestCases = append(result.TestCases, events.TestCaseResult{
			Name:       matchSkip[1],
			Status:     statusSkipped,
			DurationMs: int64(duration * msPerSecond),
		})
		result.TotalTests++
		result.SkippedTests++
	}
}

func (p *TestResultParser) parseGoCoverage(output string) float64 {
	// Parse coverage from go test output
	// coverage: 75.5% of statements
	if match := goCoverageRe.FindStringSubmatch(output); match != nil {
		coverage, err := strconv.ParseFloat(match[1], 64)
		if err == nil {
			return coverage
		}
	}
	return 0
}

// =============================================================================
// Python pytest Parser
// =============================================================================

func (p *TestResultParser) parsePytestOutput(output string) *events.TestResult {
	result := &events.TestResult{
		TestCases: []events.TestCaseResult{},
	}

	// Parse summary line: "5 passed, 2 failed, 1 skipped in 1.23s"
	if match := pytestSummaryRe.FindStringSubmatch(output); match != nil {
		result.PassedTests, _ = strconv.Atoi(match[1])
		if match[2] != "" {
			result.FailedTests, _ = strconv.Atoi(match[2])
		}
		if match[3] != "" {
			result.SkippedTests, _ = strconv.Atoi(match[3])
		}
		duration, _ := strconv.ParseFloat(match[5], 64)
		result.DurationMs = int64(duration * msPerSecond)
	}

	// Parse individual test results
	// PASSED tests/test_foo.py::test_bar
	// FAILED tests/test_foo.py::test_baz
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if matchPass := pytestPassedRe.FindStringSubmatch(line); matchPass != nil {
			result.TestCases = append(result.TestCases, events.TestCaseResult{
				Name:   matchPass[1],
				Status: statusPassed,
			})
		} else if matchFail := pytestFailedRe.FindStringSubmatch(line); matchFail != nil {
			result.TestCases = append(result.TestCases, events.TestCaseResult{
				Name:   matchFail[1],
				Status: statusFailed,
			})
		} else if matchSkip := pytestSkippedRe.FindStringSubmatch(line); matchSkip != nil {
			result.TestCases = append(result.TestCases, events.TestCaseResult{
				Name:   matchSkip[1],
				Status: statusSkipped,
			})
		}
	}

	result.TotalTests = result.PassedTests + result.FailedTests + result.SkippedTests
	result.Success = result.FailedTests == 0

	// Parse coverage
	result.Coverage = p.parsePythonCoverage(output)

	return result
}

func (p *TestResultParser) parsePythonCoverage(output string) float64 {
	// Parse coverage from pytest-cov output
	// TOTAL                                                  123     45    63%
	if match := pytestCoverageRe.FindStringSubmatch(output); match != nil {
		coverage, err := strconv.ParseFloat(match[1], 64)
		if err == nil {
			return coverage
		}
	}
	return 0
}

// =============================================================================
// Node.js Jest Parser
// =============================================================================

// jestResult represents Jest JSON output.
type jestResult struct {
	NumTotalTests   int                         `json:"numTotalTests"`
	NumPassedTests  int                         `json:"numPassedTests"`
	NumFailedTests  int                         `json:"numFailedTests"`
	NumPendingTests int                         `json:"numPendingTests"` // skipped
	Success         bool                        `json:"success"`
	TestResults     []jestTestFile              `json:"testResults"`
	CoverageMap     map[string]jestFileCoverage `json:"coverageMap,omitempty"`
}

type jestTestFile struct {
	Name             string          `json:"name"`
	AssertionResults []jestAssertion `json:"assertionResults"`
}

type jestAssertion struct {
	AncestorTitles  []string `json:"ancestorTitles"`
	FullName        string   `json:"fullName"`
	Status          string   `json:"status"` // passed, failed, pending
	Title           string   `json:"title"`
	Duration        int64    `json:"duration"`
	FailureMessages []string `json:"failureMessages"`
}

type jestFileCoverage struct {
	Path string           `json:"path"`
	S    map[string]int   `json:"s"` // statement coverage
	B    map[string][]int `json:"b"` // branch coverage
	F    map[string]int   `json:"f"` // function coverage
}

func (p *TestResultParser) parseJestOutput(output string) *events.TestResult {
	result := &events.TestResult{
		TestCases: []events.TestCaseResult{},
	}

	// Try to parse JSON output
	var jestRes jestResult
	if err := json.Unmarshal([]byte(output), &jestRes); err == nil {
		result.TotalTests = jestRes.NumTotalTests
		result.PassedTests = jestRes.NumPassedTests
		result.FailedTests = jestRes.NumFailedTests
		result.SkippedTests = jestRes.NumPendingTests
		result.Success = jestRes.Success

		// Extract individual test cases
		for _, file := range jestRes.TestResults {
			for _, assertion := range file.AssertionResults {
				tc := events.TestCaseResult{
					Name:       assertion.FullName,
					Suite:      strings.Join(assertion.AncestorTitles, " > "),
					DurationMs: assertion.Duration,
				}
				switch assertion.Status {
				case statusPassed:
					tc.Status = statusPassed
				case statusFailed:
					tc.Status = statusFailed
					if len(assertion.FailureMessages) > 0 {
						tc.Error = strings.Join(assertion.FailureMessages, "\n")
					}
				case "pending":
					tc.Status = statusSkipped
				default:
					tc.Status = assertion.Status
				}
				result.TestCases = append(result.TestCases, tc)
			}
		}

		// Calculate coverage if available
		result.Coverage = p.calculateJestCoverage(&jestRes)

		return result
	}

	// Fallback to text parsing
	return p.parseJestTextOutput(output)
}

func (p *TestResultParser) parseJestTextOutput(output string) *events.TestResult {
	result := &events.TestResult{
		TestCases: []events.TestCaseResult{},
	}

	// Parse summary: Tests: 3 passed, 2 failed, 5 total
	if match := jestSummaryRe.FindStringSubmatch(output); match != nil {
		if match[1] != "" {
			result.PassedTests, _ = strconv.Atoi(match[1])
		}
		if match[2] != "" {
			result.FailedTests, _ = strconv.Atoi(match[2])
		}
		if match[3] != "" {
			result.SkippedTests, _ = strconv.Atoi(match[3])
		}
		if match[4] != "" {
			result.TotalTests, _ = strconv.Atoi(match[4])
		}
	}

	// Parse time: Time: 1.234s
	if match := jestTimeRe.FindStringSubmatch(output); match != nil {
		duration, _ := strconv.ParseFloat(match[1], 64)
		result.DurationMs = int64(duration * msPerSecond)
	}

	// Parse coverage: All files | 75.5 | ...
	if match := jestCoverageRe.FindStringSubmatch(output); match != nil {
		result.Coverage, _ = strconv.ParseFloat(match[1], 64)
	}

	result.Success = result.FailedTests == 0
	return result
}

func (p *TestResultParser) calculateJestCoverage(jestRes *jestResult) float64 {
	if len(jestRes.CoverageMap) == 0 {
		return 0
	}

	var totalStatements, coveredStatements int
	for _, file := range jestRes.CoverageMap {
		for _, count := range file.S {
			totalStatements++
			if count > 0 {
				coveredStatements++
			}
		}
	}

	if totalStatements == 0 {
		return 0
	}
	return float64(coveredStatements) / float64(totalStatements) * percentMultiple
}

// =============================================================================
// Java JUnit XML Parser
// =============================================================================

// junitTestSuites represents the root element of JUnit XML.
type junitTestSuites struct {
	XMLName    xml.Name         `xml:"testsuites"`
	TestSuites []junitTestSuite `xml:"testsuite"`
}

// junitTestSuite represents a single test suite.
type junitTestSuite struct {
	XMLName   xml.Name        `xml:"testsuite"`
	Name      string          `xml:"name,attr"`
	Tests     int             `xml:"tests,attr"`
	Errors    int             `xml:"errors,attr"`
	Failures  int             `xml:"failures,attr"`
	Skipped   int             `xml:"skipped,attr"`
	Time      float64         `xml:"time,attr"`
	TestCases []junitTestCase `xml:"testcase"`
}

// junitTestCase represents a single test case.
type junitTestCase struct {
	XMLName   xml.Name      `xml:"testcase"`
	Name      string        `xml:"name,attr"`
	ClassName string        `xml:"classname,attr"`
	Time      float64       `xml:"time,attr"`
	Failure   *junitFailure `xml:"failure"`
	Error     *junitError   `xml:"error"`
	Skipped   *junitSkipped `xml:"skipped"`
}

type junitFailure struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Content string `xml:",chardata"`
}

type junitError struct {
	Message string `xml:"message,attr"`
	Type    string `xml:"type,attr"`
	Content string `xml:",chardata"`
}

type junitSkipped struct {
	Message string `xml:"message,attr"`
}

func (p *TestResultParser) parseJUnitXML(output string) *events.TestResult {
	result := &events.TestResult{
		TestCases: []events.TestCaseResult{},
	}

	// Try parsing as testsuites (multiple suites)
	var suites junitTestSuites
	if err := xml.Unmarshal([]byte(output), &suites); err == nil && len(suites.TestSuites) > 0 {
		for _, suite := range suites.TestSuites {
			p.processJUnitSuite(&suite, result)
		}
		result.Success = result.FailedTests == 0
		return result
	}

	// Try parsing as single testsuite
	var suite junitTestSuite
	if err := xml.Unmarshal([]byte(output), &suite); err == nil {
		p.processJUnitSuite(&suite, result)
		result.Success = result.FailedTests == 0
		return result
	}

	// Fallback to generic parsing
	return p.parseGenericOutput(output, 1) // Assume failure if can't parse
}

func (p *TestResultParser) processJUnitSuite(suite *junitTestSuite, result *events.TestResult) {
	result.DurationMs += int64(suite.Time * msPerSecond)

	for _, tc := range suite.TestCases {
		testCase := events.TestCaseResult{
			Name:       tc.Name,
			Suite:      tc.ClassName,
			DurationMs: int64(tc.Time * msPerSecond),
		}

		switch {
		case tc.Skipped != nil:
			testCase.Status = statusSkipped
			result.SkippedTests++
		case tc.Failure != nil:
			testCase.Status = statusFailed
			testCase.Error = tc.Failure.Message
			if tc.Failure.Content != "" {
				testCase.Output = tc.Failure.Content
			}
			result.FailedTests++
		case tc.Error != nil:
			testCase.Status = statusFailed
			testCase.Error = tc.Error.Message
			if tc.Error.Content != "" {
				testCase.Output = tc.Error.Content
			}
			result.FailedTests++
		default:
			testCase.Status = statusPassed
			result.PassedTests++
		}

		result.TestCases = append(result.TestCases, testCase)
		result.TotalTests++
	}
}

// =============================================================================
// Generic Parser (Fallback)
// =============================================================================

func (p *TestResultParser) parseGenericOutput(output string, exitCode int) *events.TestResult {
	result := &events.TestResult{
		TotalTests: 1,
		Success:    exitCode == 0,
		TestCases:  []events.TestCaseResult{},
	}

	if exitCode == 0 {
		result.PassedTests = 1
	} else {
		result.FailedTests = 1
		result.TestCases = append(result.TestCases, events.TestCaseResult{
			Name:   "test_run",
			Status: statusFailed,
			Output: output,
		})
	}

	return result
}

// =============================================================================
// Coverage Validation
// =============================================================================

// ValidateCoverage checks if coverage meets the threshold.
func (p *TestResultParser) ValidateCoverage(coverage float64) bool {
	return coverage >= p.coverageThreshold
}

// CoverageStatus returns a description of the coverage status.
func (p *TestResultParser) CoverageStatus(coverage float64) string {
	if coverage >= p.coverageThreshold {
		return statusPassed
	}
	return statusFailed
}
