package sandbox

import (
	"testing"

	appconfig "github.com/antinvestor/builder/apps/executor/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDockerExecutor_GetLanguageConfig(t *testing.T) {
	cfg := &appconfig.ExecutorConfig{
		SandboxImage: "",
	}

	tests := []struct {
		name        string
		language    string
		wantImage   string
		wantWorkDir string
		wantCommand []string
	}{
		{
			name:        "go",
			language:    "go",
			wantImage:   "golang:1.22-alpine",
			wantWorkDir: "/app",
			wantCommand: []string{"go", "test", "-v", "-cover", "./..."},
		},
		{
			name:        "Go uppercase",
			language:    "Go",
			wantImage:   "golang:1.22-alpine",
			wantWorkDir: "/app",
			wantCommand: []string{"go", "test", "-v", "-cover", "./..."},
		},
		{
			name:        "python",
			language:    "python",
			wantImage:   "python:3.12-slim",
			wantWorkDir: "/app",
			wantCommand: []string{"python", "-m", "pytest", "-v", "--tb=short"},
		},
		{
			name:        "node",
			language:    "node",
			wantImage:   "node:20-slim",
			wantWorkDir: "/app",
			wantCommand: []string{"npm", "test"},
		},
		{
			name:        "javascript",
			language:    "javascript",
			wantImage:   "node:20-slim",
			wantWorkDir: "/app",
			wantCommand: []string{"npm", "test"},
		},
		{
			name:        "typescript",
			language:    "typescript",
			wantImage:   "node:20-slim",
			wantWorkDir: "/app",
			wantCommand: []string{"npm", "test"},
		},
		{
			name:        "java",
			language:    "java",
			wantImage:   "maven:3.9-eclipse-temurin-21-alpine",
			wantWorkDir: "/app",
			wantCommand: []string{"mvn", "test", "-B"},
		},
		{
			name:        "rust",
			language:    "rust",
			wantImage:   "rust:1.76-slim",
			wantWorkDir: "/app",
			wantCommand: []string{"cargo", "test"},
		},
		{
			name:        "ruby",
			language:    "ruby",
			wantImage:   "ruby:3.3-slim",
			wantWorkDir: "/app",
			wantCommand: []string{"bundle", "exec", "rspec"},
		},
		{
			name:        "unknown language defaults to go",
			language:    "unknown",
			wantImage:   "golang:1.22-alpine",
			wantWorkDir: "/app",
			wantCommand: []string{"go", "test", "-v", "-cover", "./..."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exec := &DockerExecutor{cfg: cfg}
			langConfig := exec.getLanguageConfig(tt.language)

			require.NotNil(t, langConfig)
			assert.Equal(t, tt.wantImage, langConfig.Image, "image")
			assert.Equal(t, tt.wantWorkDir, langConfig.WorkDir, "workdir")
			assert.Equal(t, tt.wantCommand, langConfig.TestCommand, "command")
		})
	}
}

func TestDockerExecutor_GetLanguageConfig_CustomImage(t *testing.T) {
	customImage := "custom-test-runner:latest"
	cfg := &appconfig.ExecutorConfig{
		SandboxImage: customImage,
	}

	exec := &DockerExecutor{cfg: cfg}

	languages := []string{"go", "python", "node", "java", "rust", "ruby"}

	for _, lang := range languages {
		t.Run(lang, func(t *testing.T) {
			langConfig := exec.getLanguageConfig(lang)

			require.NotNil(t, langConfig)
			assert.Equal(t, customImage, langConfig.Image, "should use custom image")
		})
	}
}

func TestDockerExecutor_GetLanguageConfig_DefaultImage(t *testing.T) {
	// When SandboxImage is the default "feature-sandbox:latest", use language-specific image
	cfg := &appconfig.ExecutorConfig{
		SandboxImage: "feature-sandbox:latest",
	}

	exec := &DockerExecutor{cfg: cfg}
	langConfig := exec.getLanguageConfig("go")

	require.NotNil(t, langConfig)
	assert.Equal(t, "golang:1.22-alpine", langConfig.Image, "should use default go image")
}

func TestDefaultLanguageConfigs(t *testing.T) {
	// Verify all expected languages are configured
	expectedLanguages := []string{"go", "python", "node", "javascript", "typescript", "java", "rust", "ruby"}

	for _, lang := range expectedLanguages {
		t.Run(lang+" has config", func(t *testing.T) {
			config, ok := defaultLanguageConfigs[lang]
			require.True(t, ok, "language %s should have a config", lang)
			assert.NotEmpty(t, config.Image, "image should be set")
			assert.NotEmpty(t, config.TestCommand, "test command should be set")
			assert.NotEmpty(t, config.WorkDir, "workdir should be set")
		})
	}
}

func TestLanguageEnvironmentVariables(t *testing.T) {
	tests := []struct {
		language string
		wantEnv  []string
	}{
		{"go", []string{"CGO_ENABLED=0"}},
		{"python", []string{"PYTHONDONTWRITEBYTECODE=1"}},
		{"node", []string{"CI=true"}},
		{"javascript", []string{"CI=true"}},
		{"typescript", []string{"CI=true"}},
		{"java", []string{"MAVEN_OPTS=-Xmx512m"}},
		{"ruby", []string{"RAILS_ENV=test"}},
	}

	for _, tt := range tests {
		t.Run(tt.language, func(t *testing.T) {
			config := defaultLanguageConfigs[tt.language]
			assert.Equal(t, tt.wantEnv, config.Env, "environment variables")
		})
	}
}

func TestStripDockerLogHeaders(t *testing.T) {
	tests := []struct {
		name   string
		input  []byte
		output string
	}{
		{
			name:   "empty input",
			input:  []byte{},
			output: "",
		},
		{
			name: "single stdout frame",
			input: []byte{
				0x01, 0x00, 0x00, 0x00, // stream type (stdout)
				0x00, 0x00, 0x00, 0x05, // frame size (5 bytes)
				'h', 'e', 'l', 'l', 'o', // content
			},
			output: "hello",
		},
		{
			name: "multiple frames",
			input: []byte{
				0x01, 0x00, 0x00, 0x00, // stdout
				0x00, 0x00, 0x00, 0x05, // 5 bytes
				'h', 'e', 'l', 'l', 'o',
				0x02, 0x00, 0x00, 0x00, // stderr
				0x00, 0x00, 0x00, 0x05, // 5 bytes
				'w', 'o', 'r', 'l', 'd',
			},
			output: "helloworld",
		},
		{
			name: "frame with newline",
			input: []byte{
				0x01, 0x00, 0x00, 0x00, // stdout
				0x00, 0x00, 0x00, 0x0A, // 10 bytes
				'l', 'i', 'n', 'e', ' ', 'o', 'n', 'e', '\n', ' ',
			},
			output: "line one\n ",
		},
		{
			name: "trailing data without headers",
			input: []byte{
				0x01, 0x00, 0x00, 0x00, // stdout
				0x00, 0x00, 0x00, 0x05, // 5 bytes
				'h', 'e', 'l', 'l', 'o',
				'X', 'Y', 'Z', // trailing bytes (< 8 bytes, no header)
			},
			output: "helloXYZ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripDockerLogHeaders(tt.input)
			assert.Equal(t, tt.output, result)
		})
	}
}
