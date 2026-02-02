package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appconfig "github.com/antinvestor/builder/apps/gateway/config"
	"github.com/antinvestor/builder/apps/gateway/service/handlers"
)

// mockPublisher is a mock queue publisher for testing.
type mockPublisher struct {
	publishedMessages []any
	shouldFail        bool
}

func (m *mockPublisher) Publish(_ context.Context, _ string, payload any, _ ...map[string]string) error {
	if m.shouldFail {
		return assert.AnError
	}
	m.publishedMessages = append(m.publishedMessages, payload)
	return nil
}

func newTestConfig() *appconfig.GatewayConfig {
	return &appconfig.GatewayConfig{
		QueueFeatureRequestName: "feature.requests",
		MaxSpecificationSize:    1048576, // 1MB
		AllowedRepositoryHosts:  "github.com,gitlab.com,bitbucket.org",
	}
}

func TestFeatureRequestHandler_ValidRequest(t *testing.T) {
	cfg := newTestConfig()
	publisher := &mockPublisher{}
	handler := handlers.NewFeatureRequestHandler(cfg, publisher)

	request := handlers.FeatureRequest{
		RepositoryURL: "https://github.com/example/repo.git",
		Branch:        "main",
		Specification: handlers.FeatureSpecification{
			Title:       "Add health endpoint",
			Description: "Add a /health endpoint that returns service status",
		},
	}
	body, _ := json.Marshal(request)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/features", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	var response handlers.FeatureResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "accepted", response.Status)
	assert.NotEmpty(t, response.ExecutionID)
	assert.Contains(t, response.Message, "queued")

	// Verify message was published
	require.Len(t, publisher.publishedMessages, 1)
	queueReq, ok := publisher.publishedMessages[0].(handlers.QueueFeatureRequest)
	require.True(t, ok)
	assert.Equal(t, response.ExecutionID, queueReq.ExecutionID)
	assert.Equal(t, "https://github.com/example/repo.git", queueReq.RepositoryURL)
	assert.Equal(t, "main", queueReq.Branch)
	assert.Equal(t, "Add health endpoint", queueReq.Specification.Title)
}

func TestFeatureRequestHandler_MethodNotAllowed(t *testing.T) {
	cfg := newTestConfig()
	publisher := &mockPublisher{}
	handler := handlers.NewFeatureRequestHandler(cfg, publisher)

	methods := []string{http.MethodGet, http.MethodPut, http.MethodDelete, http.MethodPatch}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			req := httptest.NewRequest(method, "/api/v1/features", nil)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusMethodNotAllowed, w.Code)
		})
	}
}

func TestFeatureRequestHandler_MissingRepositoryURL(t *testing.T) {
	cfg := newTestConfig()
	publisher := &mockPublisher{}
	handler := handlers.NewFeatureRequestHandler(cfg, publisher)

	request := handlers.FeatureRequest{
		Branch: "main",
		Specification: handlers.FeatureSpecification{
			Title:       "Add feature",
			Description: "Description",
		},
	}
	body, _ := json.Marshal(request)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/features", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response handlers.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "validation_error", response.Error)
	assert.Contains(t, response.Message, "repository")
}

func TestFeatureRequestHandler_InvalidRepositoryHost(t *testing.T) {
	cfg := newTestConfig()
	publisher := &mockPublisher{}
	handler := handlers.NewFeatureRequestHandler(cfg, publisher)

	request := handlers.FeatureRequest{
		RepositoryURL: "https://evil.com/malicious/repo.git",
		Branch:        "main",
		Specification: handlers.FeatureSpecification{
			Title:       "Add feature",
			Description: "Description",
		},
	}
	body, _ := json.Marshal(request)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/features", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response handlers.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "validation_error", response.Error)
	assert.Contains(t, response.Message, "not allowed")
}

func TestFeatureRequestHandler_MissingBranch(t *testing.T) {
	cfg := newTestConfig()
	publisher := &mockPublisher{}
	handler := handlers.NewFeatureRequestHandler(cfg, publisher)

	request := handlers.FeatureRequest{
		RepositoryURL: "https://github.com/example/repo.git",
		Specification: handlers.FeatureSpecification{
			Title:       "Add feature",
			Description: "Description",
		},
	}
	body, _ := json.Marshal(request)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/features", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response handlers.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "validation_error", response.Error)
	assert.Contains(t, response.Message, "branch")
}

func TestFeatureRequestHandler_MissingSpecificationTitle(t *testing.T) {
	cfg := newTestConfig()
	publisher := &mockPublisher{}
	handler := handlers.NewFeatureRequestHandler(cfg, publisher)

	request := handlers.FeatureRequest{
		RepositoryURL: "https://github.com/example/repo.git",
		Branch:        "main",
		Specification: handlers.FeatureSpecification{
			Description: "Description",
		},
	}
	body, _ := json.Marshal(request)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/features", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response handlers.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "validation_error", response.Error)
	assert.Contains(t, response.Message, "title")
}

func TestFeatureRequestHandler_MissingSpecificationDescription(t *testing.T) {
	cfg := newTestConfig()
	publisher := &mockPublisher{}
	handler := handlers.NewFeatureRequestHandler(cfg, publisher)

	request := handlers.FeatureRequest{
		RepositoryURL: "https://github.com/example/repo.git",
		Branch:        "main",
		Specification: handlers.FeatureSpecification{
			Title: "Add feature",
		},
	}
	body, _ := json.Marshal(request)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/features", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response handlers.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "validation_error", response.Error)
	assert.Contains(t, response.Message, "description")
}

func TestFeatureRequestHandler_InvalidJSON(t *testing.T) {
	cfg := newTestConfig()
	publisher := &mockPublisher{}
	handler := handlers.NewFeatureRequestHandler(cfg, publisher)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/features", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response handlers.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "invalid_json", response.Error)
}

func TestFeatureRequestHandler_QueuePublishFailure(t *testing.T) {
	cfg := newTestConfig()
	publisher := &mockPublisher{shouldFail: true}
	handler := handlers.NewFeatureRequestHandler(cfg, publisher)

	request := handlers.FeatureRequest{
		RepositoryURL: "https://github.com/example/repo.git",
		Branch:        "main",
		Specification: handlers.FeatureSpecification{
			Title:       "Add feature",
			Description: "Description",
		},
	}
	body, _ := json.Marshal(request)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/features", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response handlers.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "queue_error", response.Error)
}

func TestFeatureRequestHandler_ValidCategory(t *testing.T) {
	cfg := newTestConfig()
	publisher := &mockPublisher{}
	handler := handlers.NewFeatureRequestHandler(cfg, publisher)

	validCategories := []string{"new_feature", "bug_fix", "refactor", "documentation", "test", "dependency"}
	for _, category := range validCategories {
		t.Run(category, func(t *testing.T) {
			publisher.publishedMessages = nil
			request := handlers.FeatureRequest{
				RepositoryURL: "https://github.com/example/repo.git",
				Branch:        "main",
				Specification: handlers.FeatureSpecification{
					Title:       "Add feature",
					Description: "Description",
					Category:    category,
				},
			}
			body, _ := json.Marshal(request)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/features", bytes.NewReader(body))
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusAccepted, w.Code)
		})
	}
}

func TestFeatureRequestHandler_InvalidCategory(t *testing.T) {
	cfg := newTestConfig()
	publisher := &mockPublisher{}
	handler := handlers.NewFeatureRequestHandler(cfg, publisher)

	request := handlers.FeatureRequest{
		RepositoryURL: "https://github.com/example/repo.git",
		Branch:        "main",
		Specification: handlers.FeatureSpecification{
			Title:       "Add feature",
			Description: "Description",
			Category:    "invalid_category",
		},
	}
	body, _ := json.Marshal(request)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/features", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response handlers.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "validation_error", response.Error)
	assert.Contains(t, response.Message, "category")
}

func TestFeatureRequestHandler_ValidPriority(t *testing.T) {
	cfg := newTestConfig()
	publisher := &mockPublisher{}
	handler := handlers.NewFeatureRequestHandler(cfg, publisher)

	validPriorities := []string{"low", "normal", "high", "critical"}
	for _, priority := range validPriorities {
		t.Run(priority, func(t *testing.T) {
			publisher.publishedMessages = nil
			request := handlers.FeatureRequest{
				RepositoryURL: "https://github.com/example/repo.git",
				Branch:        "main",
				Priority:      priority,
				Specification: handlers.FeatureSpecification{
					Title:       "Add feature",
					Description: "Description",
				},
			}
			body, _ := json.Marshal(request)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/features", bytes.NewReader(body))
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusAccepted, w.Code)
		})
	}
}

func TestFeatureRequestHandler_InvalidPriority(t *testing.T) {
	cfg := newTestConfig()
	publisher := &mockPublisher{}
	handler := handlers.NewFeatureRequestHandler(cfg, publisher)

	request := handlers.FeatureRequest{
		RepositoryURL: "https://github.com/example/repo.git",
		Branch:        "main",
		Priority:      "urgent", // invalid
		Specification: handlers.FeatureSpecification{
			Title:       "Add feature",
			Description: "Description",
		},
	}
	body, _ := json.Marshal(request)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/features", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response handlers.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "validation_error", response.Error)
	assert.Contains(t, response.Message, "priority")
}

func TestFeatureRequestHandler_WithConstraints(t *testing.T) {
	cfg := newTestConfig()
	publisher := &mockPublisher{}
	handler := handlers.NewFeatureRequestHandler(cfg, publisher)

	request := handlers.FeatureRequest{
		RepositoryURL: "https://github.com/example/repo.git",
		Branch:        "main",
		Specification: handlers.FeatureSpecification{
			Title:       "Add feature",
			Description: "Description",
		},
		Constraints: &handlers.ExecutionConstraints{
			MaxSteps:       10,
			TimeoutMinutes: 30,
			MaxLLMTokens:   50000,
			AllowedPaths:   []string{"src/**/*.go"},
			ForbiddenPaths: []string{"vendor/**"},
		},
	}
	body, _ := json.Marshal(request)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/features", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusAccepted, w.Code)

	// Verify constraints were passed through
	require.Len(t, publisher.publishedMessages, 1)
	queueReq, ok := publisher.publishedMessages[0].(handlers.QueueFeatureRequest)
	require.True(t, ok)
	require.NotNil(t, queueReq.Constraints)
	assert.Equal(t, 10, queueReq.Constraints.MaxSteps)
	assert.Equal(t, 30, queueReq.Constraints.TimeoutMinutes)
}

func TestFeatureRequestHandler_AllowedHosts(t *testing.T) {
	cfg := newTestConfig()
	publisher := &mockPublisher{}
	handler := handlers.NewFeatureRequestHandler(cfg, publisher)

	allowedHosts := []string{
		"https://github.com/org/repo.git",
		"https://gitlab.com/org/repo.git",
		"https://bitbucket.org/org/repo.git",
	}

	for _, repoURL := range allowedHosts {
		t.Run(repoURL, func(t *testing.T) {
			publisher.publishedMessages = nil
			request := handlers.FeatureRequest{
				RepositoryURL: repoURL,
				Branch:        "main",
				Specification: handlers.FeatureSpecification{
					Title:       "Add feature",
					Description: "Description",
				},
			}
			body, _ := json.Marshal(request)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/features", bytes.NewReader(body))
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusAccepted, w.Code)
		})
	}
}

func TestFeatureRequestHandler_TitleTooLong(t *testing.T) {
	cfg := newTestConfig()
	publisher := &mockPublisher{}
	handler := handlers.NewFeatureRequestHandler(cfg, publisher)

	// Create a title that's 201 characters long
	longTitle := make([]byte, 201)
	for i := range longTitle {
		longTitle[i] = 'a'
	}

	request := handlers.FeatureRequest{
		RepositoryURL: "https://github.com/example/repo.git",
		Branch:        "main",
		Specification: handlers.FeatureSpecification{
			Title:       string(longTitle),
			Description: "Description",
		},
	}
	body, _ := json.Marshal(request)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/features", bytes.NewReader(body))
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response handlers.ErrorResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, "validation_error", response.Error)
	assert.Contains(t, response.Message, "200 characters")
}
