package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"time"

	appconfig "github.com/antinvestor/builder/apps/executor/config"
	"github.com/antinvestor/builder/internal/events"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/pitabwire/util"
)

// languageConfig contains configuration for a specific language.
type languageConfig struct {
	Image        string
	TestCommand  []string
	WorkDir      string
	Env          []string
	BuildCommand []string
}

// nodeJSConfig is the shared configuration for Node.js based languages.
//
//nolint:gochecknoglobals // package-level config needed for language configuration lookup
var nodeJSConfig = languageConfig{
	Image:       "node:20-slim",
	TestCommand: []string{"npm", "test"},
	WorkDir:     "/app",
	Env:         []string{"CI=true"},
}

// defaultLanguageConfigs provides default configurations for supported languages.
// This is a package-level variable to enable test access and configuration lookup.
//
//nolint:gochecknoglobals // package-level config map needed for language configuration lookup
var defaultLanguageConfigs = map[string]languageConfig{
	"go": {
		Image:       "golang:1.22-alpine",
		TestCommand: []string{"go", "test", "-v", "-cover", "./..."},
		WorkDir:     "/app",
		Env:         []string{"CGO_ENABLED=0"},
	},
	"python": {
		Image:       "python:3.12-slim",
		TestCommand: []string{"python", "-m", "pytest", "-v", "--tb=short"},
		WorkDir:     "/app",
		Env:         []string{"PYTHONDONTWRITEBYTECODE=1"},
	},
	"node":       nodeJSConfig,
	"javascript": nodeJSConfig,
	"typescript": nodeJSConfig,
	"java": {
		Image:       "maven:3.9-eclipse-temurin-21-alpine",
		TestCommand: []string{"mvn", "test", "-B"},
		WorkDir:     "/app",
		Env:         []string{"MAVEN_OPTS=-Xmx512m"},
	},
	"rust": {
		Image:       "rust:1.76-slim",
		TestCommand: []string{"cargo", "test"},
		WorkDir:     "/app",
		Env:         []string{},
	},
	"ruby": {
		Image:       "ruby:3.3-slim",
		TestCommand: []string{"bundle", "exec", "rspec"},
		WorkDir:     "/app",
		Env:         []string{"RAILS_ENV=test"},
	},
}

// DockerExecutor executes tests in Docker containers.
type DockerExecutor struct {
	cfg    *appconfig.ExecutorConfig
	client *client.Client
}

// NewDockerExecutor creates a new Docker-based executor.
func NewDockerExecutor(cfg *appconfig.ExecutorConfig) (*DockerExecutor, error) {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}

	return &DockerExecutor{
		cfg:    cfg,
		client: cli,
	}, nil
}

// Close closes the Docker client.
func (e *DockerExecutor) Close() error {
	if e.client != nil {
		return e.client.Close()
	}
	return nil
}

// Execute runs tests in a Docker container.
func (e *DockerExecutor) Execute(ctx context.Context, req *SandboxExecutionRequest) (*SandboxExecutionResult, error) {
	log := util.Log(ctx)
	startTime := time.Now()

	// Get language configuration
	langConfig := e.getLanguageConfig(req.Language)

	// Build workspace path
	workspacePath := filepath.Join(e.cfg.WorkspaceBasePath, req.ExecutionID.String())

	log.Info("starting docker sandbox execution",
		"execution_id", req.ExecutionID,
		"language", req.Language,
		"image", langConfig.Image,
		"workspace", workspacePath,
	)

	// Create container
	containerID, err := e.createContainer(ctx, req, langConfig, workspacePath)
	if err != nil {
		return nil, fmt.Errorf("create container: %w", err)
	}

	// Ensure cleanup
	defer e.cleanupContainer(ctx, containerID)

	// Start container
	if startErr := e.client.ContainerStart(ctx, containerID, container.StartOptions{}); startErr != nil {
		return nil, fmt.Errorf("start container: %w", startErr)
	}

	// Wait for container to finish with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(e.cfg.SandboxTimeoutSeconds)*time.Second)
	defer cancel()

	statusCh, errCh := e.client.ContainerWait(timeoutCtx, containerID, container.WaitConditionNotRunning)

	var exitCode int64
	select {
	case waitErr := <-errCh:
		if waitErr != nil {
			// Timeout or other error - kill the container
			log.Warn("container wait error, killing container", "error", waitErr)
			_ = e.client.ContainerKill(ctx, containerID, "KILL")
			return &SandboxExecutionResult{
				Output:   fmt.Sprintf("Execution error: %v", waitErr),
				ExitCode: -1,
				Duration: time.Since(startTime).Milliseconds(),
			}, nil
		}
	case status := <-statusCh:
		exitCode = status.StatusCode
	case <-timeoutCtx.Done():
		// Timeout reached - kill the container
		log.Warn("container execution timeout, killing container")
		_ = e.client.ContainerKill(ctx, containerID, "KILL")
		return &SandboxExecutionResult{
			Output:   "Execution timed out",
			ExitCode: -1,
			Duration: time.Since(startTime).Milliseconds(),
		}, nil
	}

	// Get container logs
	output, err := e.getContainerLogs(ctx, containerID)
	if err != nil {
		log.WithError(err).Warn("failed to get container logs")
		output = "Failed to retrieve test output"
	}

	duration := time.Since(startTime).Milliseconds()

	log.Info("docker sandbox execution completed",
		"execution_id", req.ExecutionID,
		"exit_code", exitCode,
		"duration_ms", duration,
	)

	return &SandboxExecutionResult{
		Output:   output,
		ExitCode: int(exitCode),
		Duration: duration,
	}, nil
}

// createContainer creates a Docker container for test execution.
func (e *DockerExecutor) createContainer(
	ctx context.Context,
	req *SandboxExecutionRequest,
	langConfig *languageConfig,
	workspacePath string,
) (string, error) {
	// Build container configuration
	config := &container.Config{
		Image:      langConfig.Image,
		Cmd:        langConfig.TestCommand,
		WorkingDir: langConfig.WorkDir,
		Env:        langConfig.Env,
		Tty:        false,
		Labels: map[string]string{
			"builder.execution.id": req.ExecutionID.String(),
			"builder.managed":      "true",
		},
	}

	// Calculate resource limits
	memoryLimit := int64(e.cfg.SandboxMemoryLimitMB) * 1024 * 1024 // Convert MB to bytes
	cpuQuota := int64(e.cfg.SandboxCPULimit * 100000)              // CPU quota in microseconds (100000 = 1 CPU)

	hostConfig := &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:     mount.TypeBind,
				Source:   workspacePath,
				Target:   langConfig.WorkDir,
				ReadOnly: false,
			},
		},
		Resources: container.Resources{
			Memory:   memoryLimit,
			CPUQuota: cpuQuota,
		},
		AutoRemove: false, // We'll remove manually after getting logs
	}

	// Disable network if configured
	var networkConfig *network.NetworkingConfig
	if !e.cfg.SandboxNetworkEnabled {
		hostConfig.NetworkMode = "none"
	}

	// Create container
	containerName := fmt.Sprintf("builder-test-%s", req.ExecutionID.String()[:8])
	resp, err := e.client.ContainerCreate(ctx, config, hostConfig, networkConfig, nil, containerName)
	if err != nil {
		return "", fmt.Errorf("container create: %w", err)
	}

	return resp.ID, nil
}

// getContainerLogs retrieves the logs from a container.
func (e *DockerExecutor) getContainerLogs(ctx context.Context, containerID string) (string, error) {
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     false,
		Tail:       "all",
	}

	reader, err := e.client.ContainerLogs(ctx, containerID, options)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	var buf bytes.Buffer
	_, err = io.Copy(&buf, reader)
	if err != nil {
		return "", err
	}

	// Docker multiplexed stream has 8-byte header per frame
	// We need to strip these headers for clean output
	return stripDockerLogHeaders(buf.Bytes()), nil
}

// stripDockerLogHeaders removes the 8-byte header from each log frame.
func stripDockerLogHeaders(data []byte) string {
	var result bytes.Buffer
	for len(data) >= 8 {
		// First byte is stream type (1=stdout, 2=stderr)
		// Bytes 4-7 are the frame size (big-endian)
		frameSize := int(data[4])<<24 | int(data[5])<<16 | int(data[6])<<8 | int(data[7])

		// Skip header and extract frame content
		data = data[8:]
		if frameSize > len(data) {
			frameSize = len(data)
		}

		result.Write(data[:frameSize])
		data = data[frameSize:]
	}

	// If there's remaining data without proper headers, append it
	if len(data) > 0 {
		result.Write(data)
	}

	return result.String()
}

// cleanupContainer removes a container.
func (e *DockerExecutor) cleanupContainer(ctx context.Context, containerID string) {
	log := util.Log(ctx)

	// Stop container if still running
	stopTimeout := 5
	_ = e.client.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &stopTimeout})

	// Remove container
	err := e.client.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force: true,
	})
	if err != nil {
		log.WithError(err).Warn("failed to remove container", "container_id", containerID)
	} else {
		log.Debug("container cleaned up", "container_id", containerID)
	}
}

// getLanguageConfig returns the configuration for a language.
func (e *DockerExecutor) getLanguageConfig(language string) *languageConfig {
	lang := strings.ToLower(language)

	// Check for configured language
	if config, ok := defaultLanguageConfigs[lang]; ok {
		// Override image if configured
		if e.cfg.SandboxImage != "" && e.cfg.SandboxImage != "feature-sandbox:latest" {
			config.Image = e.cfg.SandboxImage
		}
		return &config
	}

	// Default to Go if language not recognized
	defaultConfig := defaultLanguageConfigs["go"]
	return &defaultConfig
}

// ExecuteWithWorkspace is a convenience method that accepts a workspace path directly.
func (e *DockerExecutor) ExecuteWithWorkspace(
	ctx context.Context,
	executionID events.ExecutionID,
	language string,
	workspacePath string,
	testCommand []string,
) (*SandboxExecutionResult, error) {
	req := &SandboxExecutionRequest{
		ExecutionID: executionID,
		Language:    language,
		Config:      e.cfg,
	}

	// Get language config and override test command if provided
	langConfig := e.getLanguageConfig(language)
	if len(testCommand) > 0 {
		langConfig.TestCommand = testCommand
	}

	return e.executeWithConfig(ctx, req, langConfig, workspacePath)
}

// executeWithConfig is the internal execution method that accepts a custom config.
func (e *DockerExecutor) executeWithConfig(
	ctx context.Context,
	req *SandboxExecutionRequest,
	langConfig *languageConfig,
	workspacePath string,
) (*SandboxExecutionResult, error) {
	log := util.Log(ctx)
	startTime := time.Now()

	log.Info("starting docker sandbox execution with custom config",
		"execution_id", req.ExecutionID,
		"language", req.Language,
		"image", langConfig.Image,
		"workspace", workspacePath,
	)

	// Create container
	containerID, err := e.createContainer(ctx, req, langConfig, workspacePath)
	if err != nil {
		return nil, fmt.Errorf("create container: %w", err)
	}

	// Ensure cleanup
	defer e.cleanupContainer(ctx, containerID)

	// Start container
	if startErr := e.client.ContainerStart(ctx, containerID, container.StartOptions{}); startErr != nil {
		return nil, fmt.Errorf("start container: %w", startErr)
	}

	// Wait for container to finish with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(e.cfg.SandboxTimeoutSeconds)*time.Second)
	defer cancel()

	statusCh, errCh := e.client.ContainerWait(timeoutCtx, containerID, container.WaitConditionNotRunning)

	var exitCode int64
	select {
	case waitErr := <-errCh:
		if waitErr != nil {
			log.Warn("container wait error, killing container", "error", waitErr)
			_ = e.client.ContainerKill(ctx, containerID, "KILL")
			return &SandboxExecutionResult{
				Output:   fmt.Sprintf("Execution error: %v", waitErr),
				ExitCode: -1,
				Duration: time.Since(startTime).Milliseconds(),
			}, nil
		}
	case status := <-statusCh:
		exitCode = status.StatusCode
	case <-timeoutCtx.Done():
		log.Warn("container execution timeout, killing container")
		_ = e.client.ContainerKill(ctx, containerID, "KILL")
		return &SandboxExecutionResult{
			Output:   "Execution timed out",
			ExitCode: -1,
			Duration: time.Since(startTime).Milliseconds(),
		}, nil
	}

	// Get container logs
	output, logsErr := e.getContainerLogs(ctx, containerID)
	if logsErr != nil {
		log.WithError(logsErr).Warn("failed to get container logs")
		output = "Failed to retrieve test output"
	}

	duration := time.Since(startTime).Milliseconds()

	log.Info("docker sandbox execution completed",
		"execution_id", req.ExecutionID,
		"exit_code", exitCode,
		"duration_ms", duration,
	)

	return &SandboxExecutionResult{
		Output:   output,
		ExitCode: int(exitCode),
		Duration: duration,
	}, nil
}
