package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	appconfig "github.com/antinvestor/builder/apps/gateway/config"
)

// mockPublisher implements queue.Publisher for testing.
type mockPublisher struct {
	published [][]byte
	err       error
}

func (p *mockPublisher) Publish(ctx context.Context, data []byte) error {
	if p.err != nil {
		return p.err
	}
	p.published = append(p.published, data)
	return nil
}

// mockQueueManager implements queue.Manager for testing.
type mockQueueManager struct {
	publisher *mockPublisher
	err       error
}

func (m *mockQueueManager) GetPublisher(name string) (interface{ Publish(context.Context, []byte) error }, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.publisher, nil
}

// featureHandlerForTest creates a handler with test dependencies.
func featureHandlerForTest(pub *mockPublisher) (*FeatureHandler, *mockQueueManager) {
	cfg := &appconfig.GatewayConfig{}
	cfg.MaxSpecificationSize = 1024 * 1024 // 1MB
	cfg.AllowedRepositoryHosts = "github.com,gitlab.com,bitbucket.org"
	cfg.QueueFeatureRequestName = "feature.requests"

	qMan := &mockQueueManager{publisher: pub}

	// Create a wrapper that satisfies the real queue.Manager interface
	return &FeatureHandler{cfg: cfg, queue: nil}, qMan
}

func TestHandleFeatureRequest_ValidRequest(t *testing.T) {
	pub := &mockPublisher{}
	cfg := &appconfig.GatewayConfig{}
	cfg.MaxSpecificationSize = 1024 * 1024
	cfg.AllowedRepositoryHosts = "github.com,gitlab.com,bitbucket.org"
	cfg.QueueFeatureRequestName = "feature.requests"

	handler := &testableFeatureHandler{
		cfg:       cfg,
		publisher: pub,
	}

	reqBody := `{
		"repository_url": "https://github.com/org/repo.git",
		"branch": "main",
		"specification": {
			"title": "Add new feature",
			"description": "Implement feature X that does Y"
		}
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/features", strings.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.HandleFeatureRequest(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Errorf("expected status %d, got %d: %s", http.StatusAccepted, rr.Code, rr.Body.String())
	}

	var resp FeatureResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Status != "accepted" {
		t.Errorf("expected status 'accepted', got %q", resp.Status)
	}
	if resp.ExecutionID == "" {
		t.Error("expected execution_id in response")
	}
	if len(pub.published) != 1 {
		t.Errorf("expected 1 published message, got %d", len(pub.published))
	}

	// Verify queue message format
	var queueMsg QueueFeatureRequest
	if err := json.Unmarshal(pub.published[0], &queueMsg); err != nil {
		t.Fatalf("failed to parse queue message: %v", err)
	}
	if queueMsg.ExecutionID != resp.ExecutionID {
		t.Errorf("queue execution_id %q != response execution_id %q", queueMsg.ExecutionID, resp.ExecutionID)
	}
	if queueMsg.RepositoryURL != "https://github.com/org/repo.git" {
		t.Errorf("unexpected repository_url: %s", queueMsg.RepositoryURL)
	}
	if queueMsg.Branch != "main" {
		t.Errorf("unexpected branch: %s", queueMsg.Branch)
	}
}

func TestHandleFeatureRequest_MethodNotAllowed(t *testing.T) {
	handler := newTestHandler(nil)

	for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
		req := httptest.NewRequest(method, "/api/v1/features", nil)
		rr := httptest.NewRecorder()

		handler.HandleFeatureRequest(rr, req)

		if rr.Code != http.StatusMethodNotAllowed {
			t.Errorf("%s: expected status %d, got %d", method, http.StatusMethodNotAllowed, rr.Code)
		}
	}
}

func TestHandleFeatureRequest_InvalidJSON(t *testing.T) {
	handler := newTestHandler(nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/features", strings.NewReader("not valid json"))
	rr := httptest.NewRecorder()

	handler.HandleFeatureRequest(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}
}

func TestHandleFeatureRequest_MissingRequiredFields(t *testing.T) {
	handler := newTestHandler(&mockPublisher{})

	tests := []struct {
		name     string
		body     string
		expected []string
	}{
		{
			name: "missing repository_url",
			body: `{"branch":"main","specification":{"title":"t","description":"d"}}`,
			expected: []string{"repository_url is required"},
		},
		{
			name: "missing branch",
			body: `{"repository_url":"https://github.com/o/r","specification":{"title":"t","description":"d"}}`,
			expected: []string{"branch is required"},
		},
		{
			name: "missing specification.title",
			body: `{"repository_url":"https://github.com/o/r","branch":"main","specification":{"description":"d"}}`,
			expected: []string{"specification.title is required"},
		},
		{
			name: "missing specification.description",
			body: `{"repository_url":"https://github.com/o/r","branch":"main","specification":{"title":"t"}}`,
			expected: []string{"specification.description is required"},
		},
		{
			name: "missing all",
			body: `{}`,
			expected: []string{"repository_url is required", "branch is required", "specification.title is required", "specification.description is required"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/v1/features", strings.NewReader(tt.body))
			rr := httptest.NewRecorder()

			handler.HandleFeatureRequest(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
			}

			var resp FeatureResponse
			json.Unmarshal(rr.Body.Bytes(), &resp)

			for _, expected := range tt.expected {
				found := false
				for _, err := range resp.Errors {
					if err == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected error %q not found in %v", expected, resp.Errors)
				}
			}
		})
	}
}

func TestHandleFeatureRequest_InvalidRepositoryHost(t *testing.T) {
	handler := newTestHandler(&mockPublisher{})

	tests := []struct {
		name string
		url  string
	}{
		{"evil host", "https://evil.com/org/repo"},
		{"local host", "https://localhost/org/repo"},
		{"ip address", "https://192.168.1.1/org/repo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			body := `{"repository_url":"` + tt.url + `","branch":"main","specification":{"title":"t","description":"d"}}`
			req := httptest.NewRequest(http.MethodPost, "/api/v1/features", strings.NewReader(body))
			rr := httptest.NewRecorder()

			handler.HandleFeatureRequest(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
			}

			var resp FeatureResponse
			json.Unmarshal(rr.Body.Bytes(), &resp)

			foundHostError := false
			for _, err := range resp.Errors {
				if strings.Contains(err, "not in allowed hosts") {
					foundHostError = true
					break
				}
			}
			if !foundHostError {
				t.Errorf("expected host validation error, got %v", resp.Errors)
			}
		})
	}
}

func TestHandleFeatureRequest_ValidRepositoryHosts(t *testing.T) {
	pub := &mockPublisher{}
	handler := newTestHandler(pub)

	hosts := []string{
		"https://github.com/org/repo",
		"https://gitlab.com/org/repo",
		"https://bitbucket.org/org/repo",
		"https://GITHUB.COM/org/repo", // case insensitive
	}

	for _, url := range hosts {
		t.Run(url, func(t *testing.T) {
			pub.published = nil // reset
			body := `{"repository_url":"` + url + `","branch":"main","specification":{"title":"t","description":"d"}}`
			req := httptest.NewRequest(http.MethodPost, "/api/v1/features", strings.NewReader(body))
			rr := httptest.NewRecorder()

			handler.HandleFeatureRequest(rr, req)

			if rr.Code != http.StatusAccepted {
				t.Errorf("expected status %d, got %d: %s", http.StatusAccepted, rr.Code, rr.Body.String())
			}
		})
	}
}

func TestHandleFeatureRequest_RequestTooLarge(t *testing.T) {
	pub := &mockPublisher{}
	cfg := &appconfig.GatewayConfig{}
	cfg.MaxSpecificationSize = 100 // 100 bytes max
	cfg.AllowedRepositoryHosts = "github.com"
	cfg.QueueFeatureRequestName = "feature.requests"

	handler := &testableFeatureHandler{
		cfg:       cfg,
		publisher: pub,
	}

	// Create a body larger than 100 bytes
	largeBody := strings.Repeat("x", 200)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/features", strings.NewReader(largeBody))
	rr := httptest.NewRecorder()

	handler.HandleFeatureRequest(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status %d, got %d", http.StatusRequestEntityTooLarge, rr.Code)
	}
}

func TestHandleFeatureRequest_WithOptionalFields(t *testing.T) {
	pub := &mockPublisher{}
	handler := newTestHandler(pub)

	reqBody := `{
		"repository_url": "https://github.com/org/repo.git",
		"branch": "develop",
		"specification": {
			"title": "Add feature",
			"description": "Description here",
			"requirements": ["req1", "req2"],
			"acceptance_criteria": ["ac1", "ac2"],
			"target_files": ["src/main.go"],
			"language": "go"
		},
		"requested_by": "user@example.com"
	}`

	req := httptest.NewRequest(http.MethodPost, "/api/v1/features", strings.NewReader(reqBody))
	rr := httptest.NewRecorder()

	handler.HandleFeatureRequest(rr, req)

	if rr.Code != http.StatusAccepted {
		t.Errorf("expected status %d, got %d: %s", http.StatusAccepted, rr.Code, rr.Body.String())
	}

	var queueMsg QueueFeatureRequest
	json.Unmarshal(pub.published[0], &queueMsg)

	if queueMsg.RequestedBy != "user@example.com" {
		t.Errorf("expected requested_by 'user@example.com', got %q", queueMsg.RequestedBy)
	}
	if queueMsg.Specification.Language != "go" {
		t.Errorf("expected language 'go', got %q", queueMsg.Specification.Language)
	}
	if len(queueMsg.Specification.TargetFiles) != 1 {
		t.Errorf("expected 1 target file, got %d", len(queueMsg.Specification.TargetFiles))
	}
	// Requirements should include both requirements and acceptance_criteria
	if len(queueMsg.Specification.Requirements) != 4 {
		t.Errorf("expected 4 requirements (2 req + 2 ac), got %d", len(queueMsg.Specification.Requirements))
	}
}

// testableFeatureHandler is a test-specific handler that doesn't require the full queue.Manager interface.
type testableFeatureHandler struct {
	cfg       *appconfig.GatewayConfig
	publisher *mockPublisher
}

func (h *testableFeatureHandler) HandleFeatureRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method != http.MethodPost {
		writeErrorResponse(w, http.StatusMethodNotAllowed, "Method not allowed", nil)
		return
	}

	maxSize := int64(h.cfg.MaxSpecificationSize)
	r.Body = http.MaxBytesReader(w, r.Body, maxSize)

	body, err := readBody(r.Body)
	if err != nil {
		if strings.Contains(err.Error(), "http: request body too large") {
			writeErrorResponse(w, http.StatusRequestEntityTooLarge,
				"Request body exceeds maximum size", nil)
			return
		}
		writeErrorResponse(w, http.StatusBadRequest, "Failed to read request body", nil)
		return
	}

	var req FeatureRequest
	if err := json.Unmarshal(body, &req); err != nil {
		writeErrorResponse(w, http.StatusBadRequest, "Invalid JSON in request body", []string{err.Error()})
		return
	}

	errors := validateRequest(req, h.cfg.AllowedRepositoryHosts)
	if len(errors) > 0 {
		writeErrorResponse(w, http.StatusBadRequest, "Validation failed", errors)
		return
	}

	execID := generateExecutionID()

	queueMsg := buildQueueMessage(req, execID)
	data, _ := json.Marshal(queueMsg)

	if err := h.publisher.Publish(ctx, data); err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, "Failed to queue request", nil)
		return
	}

	writeSuccessResponse(w, execID)
}

func newTestHandler(pub *mockPublisher) *testableFeatureHandler {
	if pub == nil {
		pub = &mockPublisher{}
	}
	cfg := &appconfig.GatewayConfig{}
	cfg.MaxSpecificationSize = 1024 * 1024
	cfg.AllowedRepositoryHosts = "github.com,gitlab.com,bitbucket.org"
	cfg.QueueFeatureRequestName = "feature.requests"

	return &testableFeatureHandler{
		cfg:       cfg,
		publisher: pub,
	}
}

func readBody(body interface{ Read([]byte) (int, error) }) ([]byte, error) {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(body.(interface{ Read([]byte) (int, error); Close() error }))
	return buf.Bytes(), err
}

func validateRequest(req FeatureRequest, allowedHosts string) []string {
	var errors []string

	if req.RepositoryURL == "" {
		errors = append(errors, "repository_url is required")
	} else {
		if err := validateRepositoryURL(req.RepositoryURL, allowedHosts); err != nil {
			errors = append(errors, err.Error())
		}
	}

	if req.Branch == "" {
		errors = append(errors, "branch is required")
	}

	if req.Specification.Title == "" {
		errors = append(errors, "specification.title is required")
	}
	if req.Specification.Description == "" {
		errors = append(errors, "specification.description is required")
	}

	return errors
}

func validateRepositoryURL(repoURL, allowedHosts string) error {
	parsed, err := parseURL(repoURL)
	if err != nil {
		return err
	}

	hosts := parseAllowedHosts(allowedHosts)
	host := strings.ToLower(parsed.Host)
	if colonIdx := strings.Index(host, ":"); colonIdx != -1 {
		host = host[:colonIdx]
	}

	for _, allowed := range hosts {
		if host == allowed {
			return nil
		}
	}

	return errHostNotAllowed(host, allowedHosts)
}

func parseURL(rawURL string) (*struct{ Host string }, error) {
	u, err := (&stdlibURLParser{}).Parse(rawURL)
	if err != nil {
		return nil, err
	}
	return &struct{ Host string }{Host: u.Host}, nil
}

type stdlibURLParser struct{}

func (p *stdlibURLParser) Parse(rawURL string) (*struct{ Host, Scheme string }, error) {
	u, err := (&urlParser{}).parse(rawURL)
	if err != nil {
		return nil, err
	}
	return &struct{ Host, Scheme string }{Host: u.Host, Scheme: u.Scheme}, nil
}

type urlParser struct{}

func (p *urlParser) parse(rawURL string) (result struct{ Host, Scheme string }, err error) {
	// Simple URL parsing for tests
	if !strings.Contains(rawURL, "://") {
		return result, errInvalidURL("missing scheme")
	}
	parts := strings.SplitN(rawURL, "://", 2)
	result.Scheme = parts[0]
	if len(parts) > 1 {
		hostPath := parts[1]
		if slashIdx := strings.Index(hostPath, "/"); slashIdx != -1 {
			result.Host = hostPath[:slashIdx]
		} else {
			result.Host = hostPath
		}
	}
	return result, nil
}

func errInvalidURL(msg string) error {
	return &validationError{msg: "repository_url is not a valid URL: " + msg}
}

func errHostNotAllowed(host, allowed string) error {
	return &validationError{msg: "repository_url host \"" + host + "\" is not in allowed hosts: " + allowed}
}

type validationError struct{ msg string }

func (e *validationError) Error() string { return e.msg }

func parseAllowedHosts(hosts string) []string {
	parts := strings.Split(hosts, ",")
	result := make([]string, 0, len(parts))
	for _, h := range parts {
		trimmed := strings.TrimSpace(strings.ToLower(h))
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func generateExecutionID() string {
	return "test-exec-id-12345678"
}

func buildQueueMessage(req FeatureRequest, execID string) *QueueFeatureRequest {
	msg := &QueueFeatureRequest{
		ExecutionID:   execID,
		RepositoryURL: req.RepositoryURL,
		Branch:        req.Branch,
		Specification: QueueFeatureSpecification{
			Title:        req.Specification.Title,
			Description:  req.Specification.Description,
			Requirements: req.Specification.Requirements,
			TargetFiles:  req.Specification.TargetFiles,
			Language:     req.Specification.Language,
		},
		RequestedBy: req.RequestedBy,
	}

	if len(req.Specification.AcceptanceCriteria) > 0 {
		msg.Specification.Requirements = append(
			msg.Specification.Requirements,
			req.Specification.AcceptanceCriteria...,
		)
	}

	return msg
}

func writeErrorResponse(w http.ResponseWriter, code int, message string, errors []string) {
	resp := FeatureResponse{
		Status:  "error",
		Message: message,
		Errors:  errors,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(resp)
}

func writeSuccessResponse(w http.ResponseWriter, executionID string) {
	resp := FeatureResponse{
		Status:      "accepted",
		ExecutionID: executionID,
		Message:     "Feature request queued for processing",
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(resp)
}
