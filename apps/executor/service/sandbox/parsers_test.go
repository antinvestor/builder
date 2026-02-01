package sandbox_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/antinvestor/builder/apps/executor/service/sandbox"
)

func TestParseGoTestOutput(t *testing.T) {
	parser := sandbox.NewTestResultParser(70.0)

	tests := []struct {
		name         string
		output       string
		exitCode     int
		wantTotal    int
		wantPassed   int
		wantFailed   int
		wantSkipped  int
		wantSuccess  bool
		wantCoverage float64
	}{
		{
			name: "go test -json output with passing tests",
			output: `{"Time":"2024-01-15T10:00:00.000Z","Action":"run","Package":"example/pkg","Test":"TestOne"}
{"Time":"2024-01-15T10:00:00.100Z","Action":"pass","Package":"example/pkg","Test":"TestOne","Elapsed":0.1}
{"Time":"2024-01-15T10:00:00.200Z","Action":"run","Package":"example/pkg","Test":"TestTwo"}
{"Time":"2024-01-15T10:00:00.300Z","Action":"pass","Package":"example/pkg","Test":"TestTwo","Elapsed":0.1}
{"Time":"2024-01-15T10:00:00.400Z","Action":"pass","Package":"example/pkg","Elapsed":0.4}`,
			exitCode:    0,
			wantTotal:   2,
			wantPassed:  2,
			wantFailed:  0,
			wantSkipped: 0,
			wantSuccess: true,
		},
		{
			name: "go test -json output with failing test",
			output: `{"Time":"2024-01-15T10:00:00.000Z","Action":"run","Package":"example/pkg","Test":"TestOne"}
{"Time":"2024-01-15T10:00:00.100Z","Action":"fail","Package":"example/pkg","Test":"TestOne","Elapsed":0.1}
{"Time":"2024-01-15T10:00:00.200Z","Action":"fail","Package":"example/pkg","Elapsed":0.2}`,
			exitCode:    1,
			wantTotal:   1,
			wantPassed:  0,
			wantFailed:  1,
			wantSkipped: 0,
			wantSuccess: false,
		},
		{
			name: "traditional go test output",
			output: `=== RUN   TestOne
--- PASS: TestOne (0.10s)
=== RUN   TestTwo
--- FAIL: TestTwo (0.20s)
=== RUN   TestThree
--- SKIP: TestThree (0.00s)
FAIL
coverage: 75.5% of statements`,
			exitCode:     1,
			wantTotal:    3,
			wantPassed:   1,
			wantFailed:   1,
			wantSkipped:  1,
			wantSuccess:  false,
			wantCoverage: 75.5,
		},
		{
			name:         "go test with coverage only",
			output:       `ok  	example/pkg	0.123s	coverage: 82.3% of statements`,
			exitCode:     0,
			wantCoverage: 82.3,
			wantSuccess:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseTestOutput(sandbox.LanguageGo, tt.output, tt.exitCode)
			require.NoError(t, err)

			if tt.wantTotal > 0 {
				assert.Equal(t, tt.wantTotal, result.TotalTests, "TotalTests")
				assert.Equal(t, tt.wantPassed, result.PassedTests, "PassedTests")
				assert.Equal(t, tt.wantFailed, result.FailedTests, "FailedTests")
				assert.Equal(t, tt.wantSkipped, result.SkippedTests, "SkippedTests")
			}
			assert.Equal(t, tt.wantSuccess, result.Success, "Success")
			if tt.wantCoverage > 0 {
				assert.InDelta(t, tt.wantCoverage, result.Coverage, 0.1, "Coverage")
			}
		})
	}
}

func TestParsePytestOutput(t *testing.T) {
	parser := sandbox.NewTestResultParser(70.0)

	tests := []struct {
		name         string
		output       string
		exitCode     int
		wantTotal    int
		wantPassed   int
		wantFailed   int
		wantSkipped  int
		wantSuccess  bool
		wantCoverage float64
	}{
		{
			name: "pytest summary with all passed",
			output: `PASSED tests/test_one.py::test_a
PASSED tests/test_one.py::test_b
5 passed in 1.23s`,
			exitCode:    0,
			wantTotal:   5,
			wantPassed:  5,
			wantFailed:  0,
			wantSkipped: 0,
			wantSuccess: true,
		},
		{
			name: "pytest with failures",
			output: `PASSED tests/test_one.py::test_a
FAILED tests/test_one.py::test_b
3 passed, 2 failed in 2.45s`,
			exitCode:    1,
			wantTotal:   5,
			wantPassed:  3,
			wantFailed:  2,
			wantSkipped: 0,
			wantSuccess: false,
		},
		{
			name: "pytest with coverage",
			output: `5 passed in 1.23s
---------- coverage: platform linux, python 3.9 ----------
Name                 Stmts   Miss  Cover
TOTAL                  100     25    75%`,
			exitCode:     0,
			wantTotal:    5,
			wantPassed:   5,
			wantCoverage: 75.0,
			wantSuccess:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseTestOutput(sandbox.LanguagePython, tt.output, tt.exitCode)
			require.NoError(t, err)

			assert.Equal(t, tt.wantTotal, result.TotalTests, "TotalTests")
			assert.Equal(t, tt.wantPassed, result.PassedTests, "PassedTests")
			assert.Equal(t, tt.wantFailed, result.FailedTests, "FailedTests")
			assert.Equal(t, tt.wantSkipped, result.SkippedTests, "SkippedTests")
			assert.Equal(t, tt.wantSuccess, result.Success, "Success")
			if tt.wantCoverage > 0 {
				assert.InDelta(t, tt.wantCoverage, result.Coverage, 0.1, "Coverage")
			}
		})
	}
}

func TestParseJestOutput(t *testing.T) {
	parser := sandbox.NewTestResultParser(70.0)

	tests := []struct {
		name        string
		output      string
		exitCode    int
		wantTotal   int
		wantPassed  int
		wantFailed  int
		wantSuccess bool
	}{
		{
			name: "jest JSON output",
			output: `{
				"numTotalTests": 5,
				"numPassedTests": 4,
				"numFailedTests": 1,
				"numPendingTests": 0,
				"success": false,
				"testResults": []
			}`,
			exitCode:    1,
			wantTotal:   5,
			wantPassed:  4,
			wantFailed:  1,
			wantSuccess: false,
		},
		{
			name: "jest text output",
			output: `PASS src/test.spec.js
Tests: 3 passed, 2 failed, 5 total
Time: 1.234s`,
			exitCode:    1,
			wantTotal:   5,
			wantPassed:  3,
			wantFailed:  2,
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseTestOutput(sandbox.LanguageNode, tt.output, tt.exitCode)
			require.NoError(t, err)

			assert.Equal(t, tt.wantTotal, result.TotalTests, "TotalTests")
			assert.Equal(t, tt.wantPassed, result.PassedTests, "PassedTests")
			assert.Equal(t, tt.wantFailed, result.FailedTests, "FailedTests")
			assert.Equal(t, tt.wantSuccess, result.Success, "Success")
		})
	}
}

func TestParseJUnitXML(t *testing.T) {
	parser := sandbox.NewTestResultParser(70.0)

	tests := []struct {
		name        string
		output      string
		exitCode    int
		wantTotal   int
		wantPassed  int
		wantFailed  int
		wantSkipped int
		wantSuccess bool
	}{
		{
			name: "JUnit XML with passing tests",
			output: `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="TestSuite" tests="3" time="1.5">
  <testcase name="testOne" classname="com.example.Test" time="0.5"/>
  <testcase name="testTwo" classname="com.example.Test" time="0.5"/>
  <testcase name="testThree" classname="com.example.Test" time="0.5"/>
</testsuite>`,
			exitCode:    0,
			wantTotal:   3,
			wantPassed:  3,
			wantFailed:  0,
			wantSkipped: 0,
			wantSuccess: true,
		},
		{
			name: "JUnit XML with failures",
			output: `<?xml version="1.0" encoding="UTF-8"?>
<testsuite name="TestSuite" tests="3" failures="1" time="1.5">
  <testcase name="testOne" classname="com.example.Test" time="0.5"/>
  <testcase name="testTwo" classname="com.example.Test" time="0.5">
    <failure message="assertion failed">Expected true but got false</failure>
  </testcase>
  <testcase name="testThree" classname="com.example.Test" time="0.5">
    <skipped/>
  </testcase>
</testsuite>`,
			exitCode:    1,
			wantTotal:   3,
			wantPassed:  1,
			wantFailed:  1,
			wantSkipped: 1,
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parser.ParseTestOutput(sandbox.LanguageJava, tt.output, tt.exitCode)
			require.NoError(t, err)

			assert.Equal(t, tt.wantTotal, result.TotalTests, "TotalTests")
			assert.Equal(t, tt.wantPassed, result.PassedTests, "PassedTests")
			assert.Equal(t, tt.wantFailed, result.FailedTests, "FailedTests")
			assert.Equal(t, tt.wantSkipped, result.SkippedTests, "SkippedTests")
			assert.Equal(t, tt.wantSuccess, result.Success, "Success")
		})
	}
}

func TestCoverageValidation(t *testing.T) {
	parser := sandbox.NewTestResultParser(70.0)

	tests := []struct {
		coverage float64
		wantPass bool
	}{
		{coverage: 75.0, wantPass: true},
		{coverage: 70.0, wantPass: true},
		{coverage: 69.9, wantPass: false},
		{coverage: 50.0, wantPass: false},
		{coverage: 100.0, wantPass: true},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			assert.Equal(t, tt.wantPass, parser.ValidateCoverage(tt.coverage))
		})
	}
}

func TestExitCodeOverridesSuccess(t *testing.T) {
	parser := sandbox.NewTestResultParser(70.0)

	// Even if parsing shows success, exit code 1 should mark as failed
	result, err := parser.ParseTestOutput("unknown", "some output", 1)
	require.NoError(t, err)
	assert.False(t, result.Success)
}
