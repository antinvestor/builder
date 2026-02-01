package review

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"

	appconfig "github.com/antinvestor/builder/apps/reviewer/config"
	"github.com/antinvestor/builder/internal/events"
	"github.com/pitabwire/util"
)

// securityPattern defines a pattern to detect security issues.
type securityPattern struct {
	Name        string
	Pattern     *regexp.Regexp
	PatternType events.InsecurePatternType
	Severity    events.VulnerabilitySeverity
	CWE         string
	OWASPID     string
	Description string
	Remediation string
	Languages   []string // Empty means all languages
}

// secretPattern defines a pattern to detect secrets.
type secretPattern struct {
	Name        string
	Pattern     *regexp.Regexp
	SecretType  string
	Description string
}

// PatternSecurityAnalyzer implements SecurityAnalyzer using regex patterns.
type PatternSecurityAnalyzer struct {
	cfg              *appconfig.ReviewerConfig
	securityPatterns []securityPattern
	secretPatterns   []secretPattern
}

// NewPatternSecurityAnalyzer creates a new pattern-based security analyzer.
func NewPatternSecurityAnalyzer(cfg *appconfig.ReviewerConfig) *PatternSecurityAnalyzer {
	return &PatternSecurityAnalyzer{
		cfg:              cfg,
		securityPatterns: initSecurityPatterns(),
		secretPatterns:   initSecretPatterns(),
	}
}

// initSecurityPatterns initializes security vulnerability patterns.
func initSecurityPatterns() []securityPattern {
	return []securityPattern{
		// SQL Injection patterns
		{
			Name:        "SQL Injection - String Concatenation",
			Pattern:     regexp.MustCompile(`(?i)(SELECT|INSERT|UPDATE|DELETE|DROP|CREATE|ALTER)\s+.*\+\s*("|')?\s*\w+`),
			PatternType: events.InsecurePatternSQLInjection,
			Severity:    events.VulnerabilitySeverityCritical,
			CWE:         "CWE-89",
			OWASPID:     "A03:2021",
			Description: "SQL query built with string concatenation is vulnerable to SQL injection",
			Remediation: "Use parameterized queries or prepared statements instead of string concatenation",
		},
		{
			Name:        "SQL Injection - fmt.Sprintf",
			Pattern:     regexp.MustCompile(`(?i)(db\.|sql\.).*fmt\.Sprintf.*SELECT|INSERT|UPDATE|DELETE`),
			PatternType: events.InsecurePatternSQLInjection,
			Severity:    events.VulnerabilitySeverityCritical,
			CWE:         "CWE-89",
			OWASPID:     "A03:2021",
			Description: "SQL query built with fmt.Sprintf may be vulnerable to SQL injection",
			Remediation: "Use parameterized queries with placeholders instead of fmt.Sprintf",
			Languages:   []string{"go"},
		},
		{
			Name:        "SQL Injection - f-string",
			Pattern:     regexp.MustCompile(`(?i)execute\s*\(\s*f["'].*SELECT|INSERT|UPDATE|DELETE`),
			PatternType: events.InsecurePatternSQLInjection,
			Severity:    events.VulnerabilitySeverityCritical,
			CWE:         "CWE-89",
			OWASPID:     "A03:2021",
			Description: "SQL query built with f-string is vulnerable to SQL injection",
			Remediation: "Use parameterized queries with placeholders",
			Languages:   []string{"python"},
		},
		// XSS patterns
		{
			Name:        "XSS - innerHTML",
			Pattern:     regexp.MustCompile(`\.innerHTML\s*=\s*[^"']+`),
			PatternType: events.InsecurePatternXSS,
			Severity:    events.VulnerabilitySeverityHigh,
			CWE:         "CWE-79",
			OWASPID:     "A03:2021",
			Description: "Setting innerHTML with dynamic content can lead to XSS",
			Remediation: "Use textContent or sanitize HTML before insertion",
			Languages:   []string{"javascript", "typescript"},
		},
		{
			Name:        "XSS - document.write",
			Pattern:     regexp.MustCompile(`document\.write\s*\(`),
			PatternType: events.InsecurePatternXSS,
			Severity:    events.VulnerabilitySeverityHigh,
			CWE:         "CWE-79",
			OWASPID:     "A03:2021",
			Description: "document.write with user input can lead to XSS",
			Remediation: "Use DOM manipulation methods instead of document.write",
			Languages:   []string{"javascript", "typescript"},
		},
		{
			Name:        "XSS - dangerouslySetInnerHTML",
			Pattern:     regexp.MustCompile(`dangerouslySetInnerHTML\s*=\s*\{`),
			PatternType: events.InsecurePatternXSS,
			Severity:    events.VulnerabilitySeverityMedium,
			CWE:         "CWE-79",
			OWASPID:     "A03:2021",
			Description: "dangerouslySetInnerHTML can lead to XSS if content is not sanitized",
			Remediation: "Sanitize HTML content using a library like DOMPurify before rendering",
			Languages:   []string{"javascript", "typescript"},
		},
		// Command Injection patterns
		{
			Name:        "Command Injection - exec",
			Pattern:     regexp.MustCompile(`(?i)(exec|system|popen|subprocess\.call|subprocess\.run|os\.system|shell_exec)\s*\([^)]*\+`),
			PatternType: events.InsecurePatternCommandInjection,
			Severity:    events.VulnerabilitySeverityCritical,
			CWE:         "CWE-78",
			OWASPID:     "A03:2021",
			Description: "Command execution with dynamic input is vulnerable to command injection",
			Remediation: "Use safe command execution with argument arrays, avoid shell interpretation",
		},
		{
			Name:        "Command Injection - shell=True",
			Pattern:     regexp.MustCompile(`subprocess\.(run|call|Popen)\s*\([^)]*shell\s*=\s*True`),
			PatternType: events.InsecurePatternCommandInjection,
			Severity:    events.VulnerabilitySeverityHigh,
			CWE:         "CWE-78",
			OWASPID:     "A03:2021",
			Description: "Using shell=True with dynamic input enables command injection",
			Remediation: "Set shell=False and pass command as a list",
			Languages:   []string{"python"},
		},
		// Path Traversal patterns
		{
			Name:        "Path Traversal - User Input",
			Pattern:     regexp.MustCompile(`(?i)(os\.Open|ioutil\.ReadFile|os\.ReadFile|open\(|fopen|file_get_contents)\s*\([^)]*(\+|fmt\.Sprintf|f"|%s)`),
			PatternType: events.InsecurePatternPathTraversal,
			Severity:    events.VulnerabilitySeverityHigh,
			CWE:         "CWE-22",
			OWASPID:     "A01:2021",
			Description: "File path built from user input without sanitization",
			Remediation: "Validate and sanitize file paths, use filepath.Clean and verify paths are within allowed directories",
		},
		{
			Name:        "Path Traversal - Direct Join",
			Pattern:     regexp.MustCompile(`filepath\.Join\s*\([^)]*req\.|r\.|request\.|params\.|query\.`),
			PatternType: events.InsecurePatternPathTraversal,
			Severity:    events.VulnerabilitySeverityMedium,
			CWE:         "CWE-22",
			OWASPID:     "A01:2021",
			Description: "File path joined with user input may allow directory traversal",
			Remediation: "Validate that the resulting path is within the allowed directory",
			Languages:   []string{"go"},
		},
		// SSRF patterns
		{
			Name:        "SSRF - User-Controlled URL",
			Pattern:     regexp.MustCompile(`(?i)(http\.Get|http\.Post|requests\.get|requests\.post|fetch|axios|urllib\.request)\s*\([^)]*(\+|%s|f"|\${)`),
			PatternType: events.InsecurePatternSSRF,
			Severity:    events.VulnerabilitySeverityHigh,
			CWE:         "CWE-918",
			OWASPID:     "A10:2021",
			Description: "HTTP request URL built from user input may allow SSRF",
			Remediation: "Validate and whitelist allowed URLs/domains, block internal addresses",
		},
		// Open Redirect patterns
		{
			Name:        "Open Redirect - User URL",
			Pattern:     regexp.MustCompile(`(?i)(redirect|location\.href|window\.location|http\.Redirect)\s*[=(]\s*[^"']+(\+|req\.|request\.)`),
			PatternType: events.InsecurePatternOpenRedirect,
			Severity:    events.VulnerabilitySeverityMedium,
			CWE:         "CWE-601",
			OWASPID:     "A01:2021",
			Description: "Redirect URL from user input may allow open redirect attacks",
			Remediation: "Validate redirect URLs against a whitelist of allowed domains",
		},
		// Hardcoded Credentials patterns
		{
			Name:        "Hardcoded Password",
			Pattern:     regexp.MustCompile(`(?i)(password|passwd|pwd|secret)\s*[:=]\s*["'][^"']{4,}["']`),
			PatternType: events.InsecurePatternHardcodedCreds,
			Severity:    events.VulnerabilitySeverityHigh,
			CWE:         "CWE-798",
			OWASPID:     "A07:2021",
			Description: "Hardcoded password or secret in source code",
			Remediation: "Use environment variables or a secrets manager instead of hardcoding credentials",
		},
		{
			Name:        "Hardcoded API Key",
			Pattern:     regexp.MustCompile(`(?i)(api_key|apikey|api-key|access_token|auth_token)\s*[:=]\s*["'][a-zA-Z0-9_\-]{16,}["']`),
			PatternType: events.InsecurePatternHardcodedCreds,
			Severity:    events.VulnerabilitySeverityHigh,
			CWE:         "CWE-798",
			OWASPID:     "A07:2021",
			Description: "Hardcoded API key or token in source code",
			Remediation: "Use environment variables or a secrets manager instead of hardcoding keys",
		},
		// Weak Crypto patterns
		{
			Name:        "Weak Crypto - MD5",
			Pattern:     regexp.MustCompile(`(?i)(md5|MD5)\s*[.(]`),
			PatternType: events.InsecurePatternWeakCrypto,
			Severity:    events.VulnerabilitySeverityMedium,
			CWE:         "CWE-327",
			OWASPID:     "A02:2021",
			Description: "MD5 is a weak hash algorithm, vulnerable to collision attacks",
			Remediation: "Use SHA-256 or stronger hash algorithms for security-sensitive operations",
		},
		{
			Name:        "Weak Crypto - SHA1",
			Pattern:     regexp.MustCompile(`(?i)(sha1|SHA1)\s*[.(]`),
			PatternType: events.InsecurePatternWeakCrypto,
			Severity:    events.VulnerabilitySeverityMedium,
			CWE:         "CWE-327",
			OWASPID:     "A02:2021",
			Description: "SHA1 is a weak hash algorithm, vulnerable to collision attacks",
			Remediation: "Use SHA-256 or stronger hash algorithms for security-sensitive operations",
		},
		{
			Name:        "Weak Crypto - DES",
			Pattern:     regexp.MustCompile(`(?i)(DES|3DES|TripleDES)\.`),
			PatternType: events.InsecurePatternWeakCrypto,
			Severity:    events.VulnerabilitySeverityHigh,
			CWE:         "CWE-327",
			OWASPID:     "A02:2021",
			Description: "DES/3DES are weak encryption algorithms",
			Remediation: "Use AES-256 or ChaCha20 for encryption",
		},
		// Insecure Random patterns
		{
			Name:        "Insecure Random - math/rand",
			Pattern:     regexp.MustCompile(`(?i)math/rand`),
			PatternType: events.InsecurePatternInsecureRandom,
			Severity:    events.VulnerabilitySeverityMedium,
			CWE:         "CWE-330",
			OWASPID:     "A02:2021",
			Description: "math/rand is not cryptographically secure",
			Remediation: "Use crypto/rand for security-sensitive random number generation",
			Languages:   []string{"go"},
		},
		{
			Name:        "Insecure Random - Math.random",
			Pattern:     regexp.MustCompile(`Math\.random\s*\(`),
			PatternType: events.InsecurePatternInsecureRandom,
			Severity:    events.VulnerabilitySeverityMedium,
			CWE:         "CWE-330",
			OWASPID:     "A02:2021",
			Description: "Math.random() is not cryptographically secure",
			Remediation: "Use crypto.getRandomValues() for security-sensitive operations",
			Languages:   []string{"javascript", "typescript"},
		},
		// Insecure Deserialization patterns
		{
			Name:        "Insecure Deserialization - pickle",
			Pattern:     regexp.MustCompile(`pickle\.(load|loads)\s*\(`),
			PatternType: events.InsecurePatternInsecureDeserialize,
			Severity:    events.VulnerabilitySeverityCritical,
			CWE:         "CWE-502",
			OWASPID:     "A08:2021",
			Description: "pickle deserialization of untrusted data can lead to code execution",
			Remediation: "Use safe serialization formats like JSON for untrusted data",
			Languages:   []string{"python"},
		},
		{
			Name:        "Insecure Deserialization - eval",
			Pattern:     regexp.MustCompile(`(?i)\beval\s*\([^)]*(\+|req\.|request\.|input|user)`),
			PatternType: events.InsecurePatternInsecureDeserialize,
			Severity:    events.VulnerabilitySeverityCritical,
			CWE:         "CWE-94",
			OWASPID:     "A03:2021",
			Description: "eval() with user input can lead to code injection",
			Remediation: "Never use eval() with user-controlled input",
		},
		// Log Sensitive Data patterns
		{
			Name:        "Log Sensitive - Password",
			Pattern:     regexp.MustCompile(`(?i)(log\.|logger\.|console\.|print)\s*.*password`),
			PatternType: events.InsecurePatternLogSensitiveData,
			Severity:    events.VulnerabilitySeverityMedium,
			CWE:         "CWE-532",
			OWASPID:     "A09:2021",
			Description: "Logging sensitive data like passwords",
			Remediation: "Never log passwords or other sensitive credentials",
		},
		{
			Name:        "Log Sensitive - Token",
			Pattern:     regexp.MustCompile(`(?i)(log\.|logger\.|console\.|print)\s*.*(token|secret|key|credential)`),
			PatternType: events.InsecurePatternLogSensitiveData,
			Severity:    events.VulnerabilitySeverityMedium,
			CWE:         "CWE-532",
			OWASPID:     "A09:2021",
			Description: "Logging sensitive data like tokens or secrets",
			Remediation: "Never log tokens, secrets, or API keys",
		},
		// Insecure TLS patterns
		{
			Name:        "Insecure TLS - Skip Verify",
			Pattern:     regexp.MustCompile(`InsecureSkipVerify\s*:\s*true`),
			PatternType: events.InsecurePatternInsecureTLS,
			Severity:    events.VulnerabilitySeverityHigh,
			CWE:         "CWE-295",
			OWASPID:     "A07:2021",
			Description: "TLS certificate verification is disabled",
			Remediation: "Enable TLS certificate verification in production",
			Languages:   []string{"go"},
		},
		{
			Name:        "Insecure TLS - verify=False",
			Pattern:     regexp.MustCompile(`verify\s*=\s*False`),
			PatternType: events.InsecurePatternInsecureTLS,
			Severity:    events.VulnerabilitySeverityHigh,
			CWE:         "CWE-295",
			OWASPID:     "A07:2021",
			Description: "TLS certificate verification is disabled",
			Remediation: "Enable TLS certificate verification in production",
			Languages:   []string{"python"},
		},
		{
			Name:        "Insecure TLS - rejectUnauthorized",
			Pattern:     regexp.MustCompile(`rejectUnauthorized\s*:\s*false`),
			PatternType: events.InsecurePatternInsecureTLS,
			Severity:    events.VulnerabilitySeverityHigh,
			CWE:         "CWE-295",
			OWASPID:     "A07:2021",
			Description: "TLS certificate verification is disabled",
			Remediation: "Enable TLS certificate verification in production",
			Languages:   []string{"javascript", "typescript"},
		},
	}
}

// initSecretPatterns initializes secret detection patterns.
func initSecretPatterns() []secretPattern {
	return []secretPattern{
		{
			Name:        "AWS Access Key",
			Pattern:     regexp.MustCompile(`(?i)(AKIA|ABIA|ACCA|ASIA)[A-Z0-9]{16}`),
			SecretType:  "aws_access_key",
			Description: "AWS Access Key ID detected",
		},
		{
			Name:        "AWS Secret Key",
			Pattern:     regexp.MustCompile(`(?i)aws.{0,20}secret.{0,20}['\"][A-Za-z0-9/+=]{40}['\"]`),
			SecretType:  "aws_secret_key",
			Description: "AWS Secret Access Key detected",
		},
		{
			Name:        "GitHub Token",
			Pattern:     regexp.MustCompile(`(?i)(ghp_[A-Za-z0-9]{36}|gho_[A-Za-z0-9]{36}|ghu_[A-Za-z0-9]{36}|ghs_[A-Za-z0-9]{36}|ghr_[A-Za-z0-9]{36})`),
			SecretType:  "github_token",
			Description: "GitHub personal access token detected",
		},
		{
			Name:        "GitHub OAuth",
			Pattern:     regexp.MustCompile(`(?i)github.{0,20}['\"][A-Za-z0-9]{35,40}['\"]`),
			SecretType:  "github_oauth",
			Description: "GitHub OAuth token detected",
		},
		{
			Name:        "Google API Key",
			Pattern:     regexp.MustCompile(`AIza[A-Za-z0-9_-]{35}`),
			SecretType:  "google_api_key",
			Description: "Google API Key detected",
		},
		{
			Name:        "Slack Token",
			Pattern:     regexp.MustCompile(`xox[baprs]-[0-9]{10,13}-[0-9]{10,13}[a-zA-Z0-9-]*`),
			SecretType:  "slack_token",
			Description: "Slack token detected",
		},
		{
			Name:        "Stripe API Key",
			Pattern:     regexp.MustCompile(`(?i)sk_live_[A-Za-z0-9]{24}`),
			SecretType:  "stripe_key",
			Description: "Stripe live API key detected",
		},
		{
			Name:        "Stripe Test Key",
			Pattern:     regexp.MustCompile(`(?i)sk_test_[A-Za-z0-9]{24}`),
			SecretType:  "stripe_test_key",
			Description: "Stripe test API key detected",
		},
		{
			Name:        "Private Key",
			Pattern:     regexp.MustCompile(`-----BEGIN (RSA |EC |DSA |OPENSSH |PGP )?PRIVATE KEY( BLOCK)?-----`),
			SecretType:  "private_key",
			Description: "Private key detected",
		},
		{
			Name:        "Generic Password",
			Pattern:     regexp.MustCompile(`(?i)(password|passwd|pwd)\s*[:=]\s*['\"][^'\"]{8,}['\"]`),
			SecretType:  "password",
			Description: "Password in source code detected",
		},
		{
			Name:        "Generic API Key",
			Pattern:     regexp.MustCompile(`(?i)(api[_-]?key|apikey)\s*[:=]\s*['\"][A-Za-z0-9_\-]{20,}['\"]`),
			SecretType:  "api_key",
			Description: "Generic API key detected",
		},
		{
			Name:        "Generic Secret",
			Pattern:     regexp.MustCompile(`(?i)(secret|token)\s*[:=]\s*['\"][A-Za-z0-9_\-]{16,}['\"]`),
			SecretType:  "secret",
			Description: "Generic secret or token detected",
		},
		{
			Name:        "JWT Token",
			Pattern:     regexp.MustCompile(`eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*`),
			SecretType:  "jwt_token",
			Description: "JWT token detected in source code",
		},
		{
			Name:        "Database Connection String",
			Pattern:     regexp.MustCompile(`(?i)(postgres|mysql|mongodb|redis)://[^:]+:[^@]+@`),
			SecretType:  "database_url",
			Description: "Database connection string with credentials detected",
		},
		{
			Name:        "SendGrid API Key",
			Pattern:     regexp.MustCompile(`SG\.[A-Za-z0-9_-]{22}\.[A-Za-z0-9_-]{43}`),
			SecretType:  "sendgrid_key",
			Description: "SendGrid API key detected",
		},
		{
			Name:        "Twilio API Key",
			Pattern:     regexp.MustCompile(`(?i)twilio.{0,20}['\"][A-Za-z0-9]{32}['\"]`),
			SecretType:  "twilio_key",
			Description: "Twilio API key detected",
		},
		{
			Name:        "Mailgun API Key",
			Pattern:     regexp.MustCompile(`(?i)key-[A-Za-z0-9]{32}`),
			SecretType:  "mailgun_key",
			Description: "Mailgun API key detected",
		},
		{
			Name:        "Heroku API Key",
			Pattern:     regexp.MustCompile(`(?i)heroku.{0,20}['\"][A-Fa-f0-9-]{36}['\"]`),
			SecretType:  "heroku_key",
			Description: "Heroku API key detected",
		},
	}
}

// Analyze performs security analysis on the provided code.
func (a *PatternSecurityAnalyzer) Analyze(ctx context.Context, req *SecurityAnalysisRequest) (*events.SecurityAssessment, error) {
	log := util.Log(ctx)
	log.Info("starting security analysis", "file_count", len(req.FileContents), "patch_count", len(req.Patches))

	assessment := &events.SecurityAssessment{
		OverallSecurityScore:   100,
		SecurityStatus:         events.SecurityStatusSecure,
		VulnerabilitiesFound:   []events.Vulnerability{},
		SecretsDetected:        []events.SecretFinding{},
		InsecurePatterns:       []events.InsecurePattern{},
		SecurityRegressions:    []events.SecurityRegression{},
		RequiresSecurityReview: false,
	}

	// Analyze each file
	for filePath, content := range req.FileContents {
		language := detectLanguage(filePath)

		// Check for security patterns
		patterns := a.findSecurityPatterns(filePath, content, language)
		assessment.InsecurePatterns = append(assessment.InsecurePatterns, patterns...)

		// Check for vulnerabilities (convert patterns to vulnerabilities for critical issues)
		for _, pattern := range patterns {
			if pattern.PatternType == events.InsecurePatternSQLInjection ||
				pattern.PatternType == events.InsecurePatternCommandInjection ||
				pattern.PatternType == events.InsecurePatternInsecureDeserialize {
				vuln := events.Vulnerability{
					ID:          generateVulnID(filePath, pattern.LineStart),
					Type:        patternTypeToVulnType(pattern.PatternType),
					Severity:    events.VulnerabilitySeverity(pattern.PatternType),
					CWE:         pattern.CWE,
					FilePath:    pattern.FilePath,
					LineStart:   pattern.LineStart,
					LineEnd:     pattern.LineEnd,
					Title:       pattern.Description,
					Description: pattern.Description,
					Remediation: pattern.Remediation,
				}
				assessment.VulnerabilitiesFound = append(assessment.VulnerabilitiesFound, vuln)
			}
		}

		// Check for secrets
		secrets := a.findSecrets(filePath, content)
		assessment.SecretsDetected = append(assessment.SecretsDetected, secrets...)
	}

	// Calculate security score
	assessment.OverallSecurityScore = a.calculateSecurityScore(assessment)

	// Determine security status
	assessment.SecurityStatus = a.determineSecurityStatus(assessment)

	// Determine if security review is required
	assessment.RequiresSecurityReview, assessment.SecurityReviewReason = a.determineSecurityReviewRequired(assessment)

	log.Info("security analysis complete",
		"score", assessment.OverallSecurityScore,
		"vulnerabilities", len(assessment.VulnerabilitiesFound),
		"secrets", len(assessment.SecretsDetected),
		"patterns", len(assessment.InsecurePatterns),
		"status", assessment.SecurityStatus,
	)

	return assessment, nil
}

// findSecurityPatterns finds security patterns in the content.
func (a *PatternSecurityAnalyzer) findSecurityPatterns(filePath, content, language string) []events.InsecurePattern {
	var patterns []events.InsecurePattern
	lines := strings.Split(content, "\n")

	for _, sp := range a.securityPatterns {
		// Check if pattern applies to this language
		if len(sp.Languages) > 0 && !containsString(sp.Languages, language) {
			continue
		}

		// Find all matches in the content
		matches := sp.Pattern.FindAllStringIndex(content, -1)
		for _, match := range matches {
			lineStart, lineEnd := findLineNumbers(content, match[0], match[1])
			codeSnippet := extractCodeSnippet(lines, lineStart, lineEnd)

			// Skip if it looks like a comment or test file
			if isCommentOrTest(filePath, codeSnippet) {
				continue
			}

			patterns = append(patterns, events.InsecurePattern{
				PatternType: sp.PatternType,
				Description: sp.Description,
				FilePath:    filePath,
				LineStart:   lineStart,
				LineEnd:     lineEnd,
				CodeSnippet: codeSnippet,
				Remediation: sp.Remediation,
				OWASPID:     sp.OWASPID,
				CWE:         sp.CWE,
			})
		}
	}

	return patterns
}

// findSecrets finds potential secrets in the content.
func (a *PatternSecurityAnalyzer) findSecrets(filePath, content string) []events.SecretFinding {
	var secrets []events.SecretFinding

	// Skip certain file types
	if isNonCodeFile(filePath) {
		return secrets
	}

	for _, sp := range a.secretPatterns {
		matches := sp.Pattern.FindAllStringIndex(content, -1)
		for _, match := range matches {
			lineNumber, _ := findLineNumbers(content, match[0], match[1])
			matchedText := content[match[0]:match[1]]

			// Skip if it's a test or example
			if isTestOrExample(filePath, matchedText) {
				continue
			}

			secrets = append(secrets, events.SecretFinding{
				Type:        sp.SecretType,
				FilePath:    filePath,
				LineNumber:  lineNumber,
				Description: sp.Description,
				Redacted:    redactSecret(matchedText),
			})
		}
	}

	return secrets
}

// calculateSecurityScore calculates the overall security score.
func (a *PatternSecurityAnalyzer) calculateSecurityScore(assessment *events.SecurityAssessment) int {
	score := 100

	// Deduct for vulnerabilities
	for _, vuln := range assessment.VulnerabilitiesFound {
		switch vuln.Severity {
		case events.VulnerabilitySeverityCritical:
			score -= 30
		case events.VulnerabilitySeverityHigh:
			score -= 20
		case events.VulnerabilitySeverityMedium:
			score -= 10
		case events.VulnerabilitySeverityLow:
			score -= 5
		}
	}

	// Deduct for insecure patterns
	for _, pattern := range assessment.InsecurePatterns {
		switch {
		case pattern.PatternType == events.InsecurePatternSQLInjection ||
			pattern.PatternType == events.InsecurePatternCommandInjection ||
			pattern.PatternType == events.InsecurePatternInsecureDeserialize:
			score -= 25
		case pattern.PatternType == events.InsecurePatternXSS ||
			pattern.PatternType == events.InsecurePatternPathTraversal ||
			pattern.PatternType == events.InsecurePatternSSRF:
			score -= 15
		case pattern.PatternType == events.InsecurePatternHardcodedCreds ||
			pattern.PatternType == events.InsecurePatternInsecureTLS:
			score -= 12
		default:
			score -= 8
		}
	}

	// Deduct for secrets
	for range assessment.SecretsDetected {
		score -= 20
	}

	// Deduct for security regressions
	for _, reg := range assessment.SecurityRegressions {
		switch reg.Severity {
		case events.VulnerabilitySeverityCritical:
			score -= 35
		case events.VulnerabilitySeverityHigh:
			score -= 25
		default:
			score -= 15
		}
	}

	// Ensure score doesn't go below 0
	if score < 0 {
		score = 0
	}

	return score
}

// determineSecurityStatus determines the overall security status.
func (a *PatternSecurityAnalyzer) determineSecurityStatus(assessment *events.SecurityAssessment) events.SecurityStatus {
	// Check for critical issues
	for _, vuln := range assessment.VulnerabilitiesFound {
		if vuln.Severity == events.VulnerabilitySeverityCritical {
			return events.SecurityStatusCritical
		}
	}

	// Check for any secrets
	if len(assessment.SecretsDetected) > 0 {
		return events.SecurityStatusCritical
	}

	// Check for high severity patterns
	for _, pattern := range assessment.InsecurePatterns {
		if pattern.PatternType == events.InsecurePatternSQLInjection ||
			pattern.PatternType == events.InsecurePatternCommandInjection ||
			pattern.PatternType == events.InsecurePatternInsecureDeserialize {
			return events.SecurityStatusCritical
		}
	}

	// Check score thresholds
	if assessment.OverallSecurityScore < 50 {
		return events.SecurityStatusCritical
	} else if assessment.OverallSecurityScore < 80 {
		return events.SecurityStatusWarnings
	}

	return events.SecurityStatusSecure
}

// determineSecurityReviewRequired determines if manual security review is needed.
func (a *PatternSecurityAnalyzer) determineSecurityReviewRequired(assessment *events.SecurityAssessment) (bool, string) {
	if len(assessment.SecretsDetected) > 0 {
		return true, fmt.Sprintf("Found %d potential secrets in the code", len(assessment.SecretsDetected))
	}

	for _, vuln := range assessment.VulnerabilitiesFound {
		if vuln.Severity == events.VulnerabilitySeverityCritical {
			return true, "Critical vulnerability detected: " + vuln.Title
		}
	}

	criticalPatterns := 0
	for _, pattern := range assessment.InsecurePatterns {
		if pattern.PatternType == events.InsecurePatternSQLInjection ||
			pattern.PatternType == events.InsecurePatternCommandInjection {
			criticalPatterns++
		}
	}

	if criticalPatterns > 0 {
		return true, fmt.Sprintf("Found %d critical insecure patterns", criticalPatterns)
	}

	if assessment.OverallSecurityScore < 50 {
		return true, fmt.Sprintf("Low security score: %d/100", assessment.OverallSecurityScore)
	}

	return false, ""
}

// Helper functions

func detectLanguage(filePath string) string {
	ext := strings.ToLower(filePath)
	switch {
	case strings.HasSuffix(ext, ".go"):
		return "go"
	case strings.HasSuffix(ext, ".py"):
		return "python"
	case strings.HasSuffix(ext, ".js"):
		return "javascript"
	case strings.HasSuffix(ext, ".ts"), strings.HasSuffix(ext, ".tsx"):
		return "typescript"
	case strings.HasSuffix(ext, ".java"):
		return "java"
	case strings.HasSuffix(ext, ".rb"):
		return "ruby"
	case strings.HasSuffix(ext, ".php"):
		return "php"
	case strings.HasSuffix(ext, ".rs"):
		return "rust"
	case strings.HasSuffix(ext, ".cs"):
		return "csharp"
	default:
		return "unknown"
	}
}

func findLineNumbers(content string, start, end int) (int, int) {
	lineStart := 1
	lineEnd := 1

	for i := 0; i < len(content) && i < end; i++ {
		if content[i] == '\n' {
			if i < start {
				lineStart++
			}
			lineEnd++
		}
	}

	return lineStart, lineEnd
}

func extractCodeSnippet(lines []string, lineStart, lineEnd int) string {
	if lineStart < 1 {
		lineStart = 1
	}
	if lineEnd > len(lines) {
		lineEnd = len(lines)
	}

	// Get up to 3 lines of context
	start := lineStart - 1
	end := lineEnd
	if end-start > 5 {
		end = start + 5
	}

	return strings.Join(lines[start:end], "\n")
}

func isCommentOrTest(filePath, snippet string) bool {
	// Check if file is a test file
	if strings.Contains(filePath, "_test.") || strings.Contains(filePath, ".test.") ||
		strings.Contains(filePath, "test_") || strings.Contains(filePath, "/tests/") ||
		strings.Contains(filePath, "/test/") {
		return true
	}

	// Check if snippet is likely a comment
	trimmed := strings.TrimSpace(snippet)
	return strings.HasPrefix(trimmed, "//") || strings.HasPrefix(trimmed, "#") ||
		strings.HasPrefix(trimmed, "/*") || strings.HasPrefix(trimmed, "*")
}

func isNonCodeFile(filePath string) bool {
	nonCodeExtensions := []string{".md", ".txt", ".json", ".yaml", ".yml", ".xml", ".csv", ".lock"}
	for _, ext := range nonCodeExtensions {
		if strings.HasSuffix(filePath, ext) {
			return true
		}
	}
	return false
}

func isTestOrExample(filePath, matchedText string) bool {
	if strings.Contains(filePath, "test") || strings.Contains(filePath, "example") ||
		strings.Contains(filePath, "sample") || strings.Contains(filePath, "mock") ||
		strings.Contains(filePath, "fixture") {
		return true
	}

	// Check for common test/example patterns in the match
	lowerMatch := strings.ToLower(matchedText)
	testPatterns := []string{"test", "example", "sample", "dummy", "fake", "mock", "xxx", "placeholder"}
	for _, pattern := range testPatterns {
		if strings.Contains(lowerMatch, pattern) {
			return true
		}
	}

	return false
}

func redactSecret(secret string) string {
	if len(secret) <= 8 {
		return "***"
	}
	return secret[:4] + "..." + secret[len(secret)-4:]
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func generateVulnID(filePath string, line int) string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", filePath, line)))
	return "VULN-" + hex.EncodeToString(hash[:8])
}

func patternTypeToVulnType(pt events.InsecurePatternType) events.VulnerabilityType {
	switch pt {
	case events.InsecurePatternSQLInjection:
		return events.VulnerabilityTypeInjection
	case events.InsecurePatternXSS:
		return events.VulnerabilityTypeXSS
	case events.InsecurePatternPathTraversal:
		return events.VulnerabilityTypePathTraversal
	case events.InsecurePatternSSRF:
		return events.VulnerabilityTypeSSRF
	case events.InsecurePatternInsecureDeserialize:
		return events.VulnerabilityTypeDeserialization
	case events.InsecurePatternWeakCrypto:
		return events.VulnerabilityTypeCrypto
	case events.InsecurePatternHardcodedCreds:
		return events.VulnerabilityTypeDataExposure
	default:
		return events.VulnerabilityTypeDataExposure
	}
}
