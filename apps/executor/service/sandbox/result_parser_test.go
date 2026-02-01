package sandbox

import (
	"testing"

	"github.com/antinvestor/builder/internal/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTestResultParser_ParseGoTestOutput(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		exitCode    int
		wantTotal   int
		wantPassed  int
		wantFailed  int
		wantSkipped int
		wantSuccess bool
		wantCases   int
	}{
		{
			name: "all tests pass",
			output: `=== RUN   TestAdd
--- PASS: TestAdd (0.00s)
=== RUN   TestSubtract
--- PASS: TestSubtract (0.01s)
PASS
ok  	github.com/example/pkg	0.123s
coverage: 85.5% of statements`,
			exitCode:    0,
			wantTotal:   1,
			wantPassed:  1,
			wantFailed:  0,
			wantSuccess: true,
			wantCases:   2,
		},
		{
			name: "some tests fail",
			output: `=== RUN   TestAdd
--- PASS: TestAdd (0.00s)
=== RUN   TestSubtract
    subtract_test.go:15: expected 5, got 4
--- FAIL: TestSubtract (0.01s)
FAIL
FAIL	github.com/example/pkg	0.123s`,
			exitCode:    1,
			wantTotal:   1,
			wantPassed:  0,
			wantFailed:  1,
			wantSuccess: false,
			wantCases:   2,
		},
		{
			name: "multiple packages",
			output: `ok  	github.com/example/pkg1	0.100s
ok  	github.com/example/pkg2	0.200s
FAIL	github.com/example/pkg3	0.300s`,
			exitCode:    1,
			wantTotal:   3,
			wantPassed:  2,
			wantFailed:  1,
			wantSuccess: false,
			wantCases:   0, // No individual test cases parsed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewTestResultParser()
			result := parser.ParseResults(tt.output, tt.exitCode, "go")

			assert.Equal(t, tt.wantTotal, result.TotalTests, "total tests")
			assert.Equal(t, tt.wantPassed, result.PassedTests, "passed tests")
			assert.Equal(t, tt.wantFailed, result.FailedTests, "failed tests")
			assert.Equal(t, tt.wantSuccess, result.Success, "success")
			assert.Len(t, result.TestCases, tt.wantCases, "test cases")
		})
	}
}

func TestTestResultParser_ParsePytestOutput(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		exitCode    int
		wantTotal   int
		wantPassed  int
		wantFailed  int
		wantSkipped int
		wantSuccess bool
		wantCases   int
	}{
		{
			name: "all tests pass",
			output: `test_module.py::test_add PASSED
test_module.py::test_subtract PASSED
=========================== 2 passed in 0.12s ===========================`,
			exitCode:    0,
			wantTotal:   2,
			wantPassed:  2,
			wantFailed:  0,
			wantSkipped: 0,
			wantSuccess: true,
			wantCases:   2,
		},
		{
			name: "mixed results",
			output: `test_module.py::test_add PASSED
test_module.py::test_subtract FAILED
test_module.py::test_multiply SKIPPED
=========================== 1 passed, 1 failed, 1 skipped in 0.23s ===========================`,
			exitCode:    1,
			wantTotal:   3,
			wantPassed:  1,
			wantFailed:  1,
			wantSkipped: 1,
			wantSuccess: false,
			wantCases:   3,
		},
		{
			name: "with coverage",
			output: `test_module.py::test_example PASSED
=========================== 1 passed in 0.05s ===========================

----------- coverage: platform linux, python 3.12 -----------
Name                      Stmts   Miss  Cover
---------------------------------------------
module.py                    20      3    85%
---------------------------------------------
TOTAL                        20      3    85%`,
			exitCode:    0,
			wantTotal:   1,
			wantPassed:  1,
			wantFailed:  0,
			wantSuccess: true,
			wantCases:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewTestResultParser()
			result := parser.ParseResults(tt.output, tt.exitCode, "python")

			assert.Equal(t, tt.wantTotal, result.TotalTests, "total tests")
			assert.Equal(t, tt.wantPassed, result.PassedTests, "passed tests")
			assert.Equal(t, tt.wantFailed, result.FailedTests, "failed tests")
			assert.Equal(t, tt.wantSkipped, result.SkippedTests, "skipped tests")
			assert.Equal(t, tt.wantSuccess, result.Success, "success")
			assert.Len(t, result.TestCases, tt.wantCases, "test cases")
		})
	}
}

func TestTestResultParser_ParseJestOutput(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		exitCode    int
		wantTotal   int
		wantPassed  int
		wantFailed  int
		wantSuccess bool
		wantCases   int
	}{
		{
			name: "all tests pass",
			output: ` PASS  src/utils.test.js
  ✓ adds numbers correctly (5 ms)
  ✓ subtracts numbers correctly (2 ms)

Tests: 2 passed, 2 total`,
			exitCode:    0,
			wantTotal:   2,
			wantPassed:  2,
			wantFailed:  0,
			wantSuccess: true,
			wantCases:   2,
		},
		{
			name: "mixed results",
			output: ` FAIL  src/utils.test.js
  ✓ adds numbers correctly (5 ms)
  ✕ subtracts numbers correctly (3 ms)

Tests: 1 passed, 1 failed, 2 total`,
			exitCode:    1,
			wantTotal:   2,
			wantPassed:  1,
			wantFailed:  1,
			wantSuccess: false,
			wantCases:   2,
		},
		{
			name: "with coverage",
			output: ` PASS  src/utils.test.js
  ✓ test example (2 ms)

Tests: 1 passed, 1 total
----------|---------|----------|---------|---------|
File      | % Stmts | % Branch | % Funcs | % Lines |
----------|---------|----------|---------|---------|
All files |   87.5  |    75    |   100   |   87.5  |
----------|---------|----------|---------|---------|`,
			exitCode:    0,
			wantTotal:   1,
			wantPassed:  1,
			wantFailed:  0,
			wantSuccess: true,
			wantCases:   1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewTestResultParser()
			result := parser.ParseResults(tt.output, tt.exitCode, "node")

			assert.Equal(t, tt.wantTotal, result.TotalTests, "total tests")
			assert.Equal(t, tt.wantPassed, result.PassedTests, "passed tests")
			assert.Equal(t, tt.wantFailed, result.FailedTests, "failed tests")
			assert.Equal(t, tt.wantSuccess, result.Success, "success")
			assert.Len(t, result.TestCases, tt.wantCases, "test cases")
		})
	}
}

func TestTestResultParser_ParseMavenOutput(t *testing.T) {
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
			name: "all tests pass",
			output: `[INFO] -------------------------------------------------------
[INFO]  T E S T S
[INFO] -------------------------------------------------------
[INFO] Running com.example.AppTest
[INFO] Tests run: 5, Failures: 0, Errors: 0, Skipped: 0
[INFO] BUILD SUCCESS`,
			exitCode:    0,
			wantTotal:   5,
			wantPassed:  5,
			wantFailed:  0,
			wantSkipped: 0,
			wantSuccess: true,
		},
		{
			name: "some failures",
			output: `[INFO] -------------------------------------------------------
[INFO]  T E S T S
[INFO] -------------------------------------------------------
[INFO] Running com.example.AppTest
[ERROR] Tests run: 10, Failures: 2, Errors: 1, Skipped: 1
[INFO] BUILD FAILURE`,
			exitCode:    1,
			wantTotal:   10,
			wantPassed:  6,
			wantFailed:  3,
			wantSkipped: 1,
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewTestResultParser()
			result := parser.ParseResults(tt.output, tt.exitCode, "java")

			assert.Equal(t, tt.wantTotal, result.TotalTests, "total tests")
			assert.Equal(t, tt.wantPassed, result.PassedTests, "passed tests")
			assert.Equal(t, tt.wantFailed, result.FailedTests, "failed tests")
			assert.Equal(t, tt.wantSkipped, result.SkippedTests, "skipped tests")
			assert.Equal(t, tt.wantSuccess, result.Success, "success")
		})
	}
}

func TestTestResultParser_ParseCargoOutput(t *testing.T) {
	tests := []struct {
		name        string
		output      string
		exitCode    int
		wantTotal   int
		wantPassed  int
		wantFailed  int
		wantSkipped int
		wantSuccess bool
		wantCases   int
	}{
		{
			name: "all tests pass",
			output: `running 3 tests
test tests::test_add ... ok
test tests::test_subtract ... ok
test tests::test_multiply ... ok

test result: ok. 3 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out`,
			exitCode:    0,
			wantTotal:   3,
			wantPassed:  3,
			wantFailed:  0,
			wantSkipped: 0,
			wantSuccess: true,
			wantCases:   3,
		},
		{
			name: "mixed results",
			output: `running 3 tests
test tests::test_add ... ok
test tests::test_subtract ... FAILED
test tests::test_ignored ... ignored

test result: FAILED. 1 passed; 1 failed; 1 ignored; 0 measured; 0 filtered out`,
			exitCode:    1,
			wantTotal:   3,
			wantPassed:  1,
			wantFailed:  1,
			wantSkipped: 1,
			wantSuccess: false,
			wantCases:   3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewTestResultParser()
			result := parser.ParseResults(tt.output, tt.exitCode, "rust")

			assert.Equal(t, tt.wantTotal, result.TotalTests, "total tests")
			assert.Equal(t, tt.wantPassed, result.PassedTests, "passed tests")
			assert.Equal(t, tt.wantFailed, result.FailedTests, "failed tests")
			assert.Equal(t, tt.wantSkipped, result.SkippedTests, "skipped tests")
			assert.Equal(t, tt.wantSuccess, result.Success, "success")
			assert.Len(t, result.TestCases, tt.wantCases, "test cases")
		})
	}
}

func TestTestResultParser_ParseGenericOutput(t *testing.T) {
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
			name:        "success with no recognizable output",
			output:      "All tests completed successfully!",
			exitCode:    0,
			wantTotal:   1,
			wantPassed:  1,
			wantFailed:  0,
			wantSuccess: true,
		},
		{
			name:        "failure with no recognizable output",
			output:      "Test execution failed with error",
			exitCode:    1,
			wantTotal:   1,
			wantPassed:  0,
			wantFailed:  1,
			wantSuccess: false,
		},
		{
			name:        "generic pass/fail keywords",
			output:      "Test1: passed\nTest2: passed\nTest3: failed",
			exitCode:    1,
			wantTotal:   3,
			wantPassed:  2,
			wantFailed:  1,
			wantSuccess: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewTestResultParser()
			result := parser.ParseResults(tt.output, tt.exitCode, "unknown")

			assert.Equal(t, tt.wantTotal, result.TotalTests, "total tests")
			assert.Equal(t, tt.wantPassed, result.PassedTests, "passed tests")
			assert.Equal(t, tt.wantFailed, result.FailedTests, "failed tests")
			assert.Equal(t, tt.wantSuccess, result.Success, "success")
		})
	}
}

func TestTestResultParser_TestCaseDetails(t *testing.T) {
	parser := NewTestResultParser()

	t.Run("go test case with failure message", func(t *testing.T) {
		// In Go test output, error lines appear before the FAIL line
		// The extractGoTestFailure function captures lines between the test sections
		output := `=== RUN   TestDivide
--- FAIL: TestDivide (0.00s)
    math_test.go:25: division by zero
    math_test.go:26: expected error but got nil
=== RUN   TestOther
--- PASS: TestOther (0.00s)
FAIL
FAIL	github.com/example/math	0.010s`

		result := parser.ParseResults(output, 1, "go")

		require.Len(t, result.TestCases, 2)

		// Find the TestDivide case
		var failedTest *events.TestCaseResult
		for i := range result.TestCases {
			if result.TestCases[i].Name == "TestDivide" {
				failedTest = &result.TestCases[i]
				break
			}
		}
		require.NotNil(t, failedTest, "TestDivide should be in results")
		assert.Equal(t, string(events.TestStatusFailed), failedTest.Status)
		assert.Contains(t, failedTest.Error, "division by zero")
	})

	t.Run("go test case with duration", func(t *testing.T) {
		output := `=== RUN   TestSlow
--- PASS: TestSlow (2.50s)
PASS
ok  	github.com/example/slow	2.510s`

		result := parser.ParseResults(output, 0, "go")

		require.Len(t, result.TestCases, 1)
		tc := result.TestCases[0]
		assert.Equal(t, "TestSlow", tc.Name)
		assert.Equal(t, string(events.TestStatusPassed), tc.Status)
		assert.Equal(t, int64(2500), tc.DurationMs) // 2.5s = 2500ms
	})
}

func TestTestResultParser_CoverageExtraction(t *testing.T) {
	parser := NewTestResultParser()

	t.Run("go coverage", func(t *testing.T) {
		output := `=== RUN   TestExample
--- PASS: TestExample (0.00s)
PASS
coverage: 85.5% of statements
ok  	github.com/example/pkg	0.010s`

		result := parser.ParseResults(output, 0, "go")
		assert.Equal(t, 85.5, result.Coverage)
	})

	t.Run("pytest coverage", func(t *testing.T) {
		output := `test_module.py::test_example PASSED
=========================== 1 passed in 0.05s ===========================

----------- coverage: platform linux -----------
Name                      Stmts   Miss  Cover
---------------------------------------------
module.py                    20      3    85%
---------------------------------------------
TOTAL                        20      3    85%`

		result := parser.ParseResults(output, 0, "python")
		assert.Equal(t, float64(85), result.Coverage)
	})

	t.Run("jest coverage", func(t *testing.T) {
		output := ` PASS  src/test.js
  ✓ example (1 ms)

Tests: 1 passed, 1 total
----------|---------|----------|---------|---------|
File      | % Stmts | % Branch | % Funcs | % Lines |
----------|---------|----------|---------|---------|
All files |   92.5  |    80    |   100   |   92.5  |
----------|---------|----------|---------|---------|`

		result := parser.ParseResults(output, 0, "javascript")
		assert.Equal(t, 92.5, result.Coverage)
	})
}

func TestTestResultParser_LanguageAliases(t *testing.T) {
	parser := NewTestResultParser()

	// All these should use Jest parsing
	languages := []string{"node", "javascript", "typescript"}

	output := ` PASS  test.js
  ✓ example (1 ms)

Tests: 1 passed, 1 total`

	for _, lang := range languages {
		t.Run(lang, func(t *testing.T) {
			result := parser.ParseResults(output, 0, lang)
			assert.Equal(t, 1, result.TotalTests)
			assert.Equal(t, 1, result.PassedTests)
		})
	}
}
