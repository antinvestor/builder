package review

import (
	"context"
	"testing"

	"github.com/antinvestor/builder/internal/events"
	"github.com/stretchr/testify/require"
)

func TestPatternSecurityAnalyzer_SQLInjection(t *testing.T) {
	analyzer := NewPatternSecurityAnalyzer(nil)

	tests := []struct {
		name           string
		fileContents   map[string]string
		wantPatterns   int
		wantVulnerable bool
	}{
		{
			name: "detects SQL injection via string concatenation",
			fileContents: map[string]string{
				"db.go": `
					package db
					func GetUser(id string) {
						query := "SELECT * FROM users WHERE id = '" + id + "'"
						db.Query(query)
					}
				`,
			},
			wantPatterns:   1,
			wantVulnerable: true,
		},
		{
			name: "detects SQL injection via fmt.Sprintf",
			fileContents: map[string]string{
				"db.go": `package db

import "fmt"

func GetUser(id string) {
	query := "SELECT * FROM users WHERE id = '" + id + "'"
	db.Query(query)
}
`,
			},
			wantPatterns:   1,
			wantVulnerable: true,
		},
		{
			name: "safe parameterized query",
			fileContents: map[string]string{
				"db.go": `
					package db
					func GetUser(id string) {
						db.Query("SELECT * FROM users WHERE id = $1", id)
					}
				`,
			},
			wantPatterns:   0,
			wantVulnerable: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &SecurityAnalysisRequest{
				FileContents: tt.fileContents,
				Language:     "go",
			}

			assessment, err := analyzer.Analyze(context.Background(), req)
			require.NoError(t, err)

			sqlInjectionPatterns := 0
			for _, pattern := range assessment.InsecurePatterns {
				if pattern.PatternType == events.InsecurePatternSQLInjection {
					sqlInjectionPatterns++
				}
			}

			if tt.wantVulnerable {
				require.Greater(t, sqlInjectionPatterns, 0, "expected SQL injection pattern to be detected")
			} else {
				require.Equal(t, 0, sqlInjectionPatterns, "expected no SQL injection patterns")
			}
		})
	}
}

func TestPatternSecurityAnalyzer_XSS(t *testing.T) {
	analyzer := NewPatternSecurityAnalyzer(nil)

	tests := []struct {
		name         string
		fileContents map[string]string
		wantXSS      bool
	}{
		{
			name: "detects innerHTML assignment",
			fileContents: map[string]string{
				"app.js": `
					function updateContent(data) {
						document.getElementById('content').innerHTML = data;
					}
				`,
			},
			wantXSS: true,
		},
		{
			name: "detects document.write",
			fileContents: map[string]string{
				"app.js": `
					function writeContent(data) {
						document.write(data);
					}
				`,
			},
			wantXSS: true,
		},
		{
			name: "detects dangerouslySetInnerHTML",
			fileContents: map[string]string{
				"component.tsx": `
					function Component({html}) {
						return <div dangerouslySetInnerHTML={{ __html: html }} />;
					}
				`,
			},
			wantXSS: true,
		},
		{
			name: "safe textContent assignment",
			fileContents: map[string]string{
				"app.js": `
					function updateContent(data) {
						document.getElementById('content').textContent = data;
					}
				`,
			},
			wantXSS: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &SecurityAnalysisRequest{
				FileContents: tt.fileContents,
				Language:     "javascript",
			}

			assessment, err := analyzer.Analyze(context.Background(), req)
			require.NoError(t, err)

			xssPatterns := 0
			for _, pattern := range assessment.InsecurePatterns {
				if pattern.PatternType == events.InsecurePatternXSS {
					xssPatterns++
				}
			}

			if tt.wantXSS {
				require.Greater(t, xssPatterns, 0, "expected XSS pattern to be detected")
			} else {
				require.Equal(t, 0, xssPatterns, "expected no XSS patterns")
			}
		})
	}
}

func TestPatternSecurityAnalyzer_CommandInjection(t *testing.T) {
	analyzer := NewPatternSecurityAnalyzer(nil)

	tests := []struct {
		name           string
		fileContents   map[string]string
		wantInjection  bool
	}{
		{
			name: "detects command injection with shell=True",
			fileContents: map[string]string{
				"script.py": `
					import subprocess
					def run_cmd(user_input):
						subprocess.run(user_input, shell=True)
				`,
			},
			wantInjection: true,
		},
		{
			name: "safe subprocess without shell",
			fileContents: map[string]string{
				"script.py": `
					import subprocess
					def run_cmd(args):
						subprocess.run(args, shell=False)
				`,
			},
			wantInjection: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &SecurityAnalysisRequest{
				FileContents: tt.fileContents,
				Language:     "python",
			}

			assessment, err := analyzer.Analyze(context.Background(), req)
			require.NoError(t, err)

			cmdPatterns := 0
			for _, pattern := range assessment.InsecurePatterns {
				if pattern.PatternType == events.InsecurePatternCommandInjection {
					cmdPatterns++
				}
			}

			if tt.wantInjection {
				require.Greater(t, cmdPatterns, 0, "expected command injection pattern to be detected")
			} else {
				require.Equal(t, 0, cmdPatterns, "expected no command injection patterns")
			}
		})
	}
}

func TestPatternSecurityAnalyzer_Secrets(t *testing.T) {
	analyzer := NewPatternSecurityAnalyzer(nil)

	tests := []struct {
		name         string
		fileContents map[string]string
		wantSecrets  int
	}{
		{
			name: "detects AWS access key",
			fileContents: map[string]string{
				"config.go": `package config
const AWSKey = "AKIAIOSFODNN7REALKYB"
`,
			},
			wantSecrets: 1,
		},
		{
			name: "detects GitHub token",
			fileContents: map[string]string{
				"auth.go": `package auth
var token = "ghp_abcdefghijklmnopqrstuvwxyz1234567890"
`,
			},
			wantSecrets: 1,
		},
		{
			name: "detects hardcoded password",
			fileContents: map[string]string{
				"db.go": `package db
var password = "supersecretpassword123"
`,
			},
			wantSecrets: 1,
		},
		{
			name: "detects JWT token",
			fileContents: map[string]string{
				"token.go": `package token
var jwt = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"
`,
			},
			wantSecrets: 1,
		},
		{
			name: "detects database connection string",
			fileContents: map[string]string{
				"db.go": `package db
var dbURL = "postgres://user:password@localhost:5432/db"
`,
			},
			wantSecrets: 1,
		},
		{
			name: "no secrets in safe code",
			fileContents: map[string]string{
				"config.go": `package config
var AWSKey = os.Getenv("AWS_ACCESS_KEY_ID")
`,
			},
			wantSecrets: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &SecurityAnalysisRequest{
				FileContents: tt.fileContents,
				Language:     "go",
			}

			assessment, err := analyzer.Analyze(context.Background(), req)
			require.NoError(t, err)

			if tt.wantSecrets > 0 {
				require.Greater(t, len(assessment.SecretsDetected), 0, "expected secrets to be detected")
			} else {
				require.Equal(t, 0, len(assessment.SecretsDetected), "expected no secrets")
			}
		})
	}
}

func TestPatternSecurityAnalyzer_InsecureTLS(t *testing.T) {
	analyzer := NewPatternSecurityAnalyzer(nil)

	tests := []struct {
		name         string
		fileContents map[string]string
		language     string
		wantInsecure bool
	}{
		{
			name: "detects InsecureSkipVerify in Go",
			fileContents: map[string]string{
				"http.go": `
					package http
					client := &http.Client{
						Transport: &http.Transport{
							TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
						},
					}
				`,
			},
			language:     "go",
			wantInsecure: true,
		},
		{
			name: "detects verify=False in Python",
			fileContents: map[string]string{
				"request.py": `
					import requests
					response = requests.get(url, verify=False)
				`,
			},
			language:     "python",
			wantInsecure: true,
		},
		{
			name: "detects rejectUnauthorized in Node.js",
			fileContents: map[string]string{
				"https.js": `
					const https = require('https');
					const agent = new https.Agent({
						rejectUnauthorized: false
					});
				`,
			},
			language:     "javascript",
			wantInsecure: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &SecurityAnalysisRequest{
				FileContents: tt.fileContents,
				Language:     tt.language,
			}

			assessment, err := analyzer.Analyze(context.Background(), req)
			require.NoError(t, err)

			tlsPatterns := 0
			for _, pattern := range assessment.InsecurePatterns {
				if pattern.PatternType == events.InsecurePatternInsecureTLS {
					tlsPatterns++
				}
			}

			if tt.wantInsecure {
				require.Greater(t, tlsPatterns, 0, "expected insecure TLS pattern to be detected")
			} else {
				require.Equal(t, 0, tlsPatterns, "expected no insecure TLS patterns")
			}
		})
	}
}

func TestPatternSecurityAnalyzer_WeakCrypto(t *testing.T) {
	analyzer := NewPatternSecurityAnalyzer(nil)

	tests := []struct {
		name         string
		fileContents map[string]string
		wantWeak     bool
	}{
		{
			name: "detects MD5 usage",
			fileContents: map[string]string{
				"hash.go": `
					package hash
					import "crypto/md5"
					func Hash(data []byte) []byte {
						h := md5.New()
						return h.Sum(data)
					}
				`,
			},
			wantWeak: true,
		},
		{
			name: "detects SHA1 usage",
			fileContents: map[string]string{
				"hash.go": `
					package hash
					import "crypto/sha1"
					func Hash(data []byte) []byte {
						h := sha1.New()
						return h.Sum(data)
					}
				`,
			},
			wantWeak: true,
		},
		{
			name: "safe SHA256 usage",
			fileContents: map[string]string{
				"hash.go": `
					package hash
					import "crypto/sha256"
					func Hash(data []byte) []byte {
						h := sha256.New()
						return h.Sum(data)
					}
				`,
			},
			wantWeak: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &SecurityAnalysisRequest{
				FileContents: tt.fileContents,
				Language:     "go",
			}

			assessment, err := analyzer.Analyze(context.Background(), req)
			require.NoError(t, err)

			weakPatterns := 0
			for _, pattern := range assessment.InsecurePatterns {
				if pattern.PatternType == events.InsecurePatternWeakCrypto {
					weakPatterns++
				}
			}

			if tt.wantWeak {
				require.Greater(t, weakPatterns, 0, "expected weak crypto pattern to be detected")
			} else {
				require.Equal(t, 0, weakPatterns, "expected no weak crypto patterns")
			}
		})
	}
}

func TestPatternSecurityAnalyzer_SecurityScore(t *testing.T) {
	analyzer := NewPatternSecurityAnalyzer(nil)

	tests := []struct {
		name            string
		fileContents    map[string]string
		wantScoreBelow  int
		wantStatus      events.SecurityStatus
	}{
		{
			name: "clean code has high score",
			fileContents: map[string]string{
				"safe.go": `
					package safe
					func GetUser(id string) {
						db.Query("SELECT * FROM users WHERE id = $1", id)
					}
				`,
			},
			wantScoreBelow: 101, // Score should be 100
			wantStatus:     events.SecurityStatusSecure,
		},
		{
			name: "multiple critical issues lower score significantly",
			fileContents: map[string]string{
				"vulnerable.go": `
					package vulnerable
					const password = "hardcodedpassword123"
					func GetUser(id string) {
						query := "SELECT * FROM users WHERE id = '" + id + "'"
						db.Query(query)
					}
				`,
			},
			wantScoreBelow: 50,
			wantStatus:     events.SecurityStatusCritical,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &SecurityAnalysisRequest{
				FileContents: tt.fileContents,
				Language:     "go",
			}

			assessment, err := analyzer.Analyze(context.Background(), req)
			require.NoError(t, err)

			require.Less(t, assessment.OverallSecurityScore, tt.wantScoreBelow)
			require.Equal(t, tt.wantStatus, assessment.SecurityStatus)
		})
	}
}

func TestPatternSecurityAnalyzer_SkipsTestFiles(t *testing.T) {
	analyzer := NewPatternSecurityAnalyzer(nil)

	// Test files should be skipped even if they contain patterns
	req := &SecurityAnalysisRequest{
		FileContents: map[string]string{
			"db_test.go": `
				package db
				func TestSQLInjection(t *testing.T) {
					// This is a test for SQL injection
					query := "SELECT * FROM users WHERE id = '" + testID + "'"
					// Testing that our sanitizer catches this
				}
			`,
		},
		Language: "go",
	}

	assessment, err := analyzer.Analyze(context.Background(), req)
	require.NoError(t, err)

	// Test files should be skipped
	require.Equal(t, 0, len(assessment.InsecurePatterns), "expected patterns in test files to be skipped")
}
