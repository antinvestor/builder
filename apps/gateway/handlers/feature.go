package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/pitabwire/frame/queue"
	"github.com/pitabwire/util"

	appconfig "github.com/antinvestor/builder/apps/gateway/config"
	"github.com/antinvestor/builder/internal/events"
)

// FeatureHandler handles feature request submissions.
type FeatureHandler struct {
	cfg   *appconfig.GatewayConfig
	queue queue.Manager
}

// NewFeatureHandler creates a new feature handler.
func NewFeatureHandler(cfg *appconfig.GatewayConfig, qMan queue.Manager) *FeatureHandler {
	return &FeatureHandler{
		cfg:   cfg,
		queue: qMan,
	}
}

// FeatureRequest represents an incoming feature request from clients.
type FeatureRequest struct {
	// RepositoryURL is the git repository URL (required).
	RepositoryURL string `json:"repository_url"`

	// Branch is the target branch (required).
	Branch string `json:"branch"`

	// Specification is the feature specification (required).
	Specification FeatureSpecification `json:"specification"`

	// RequestedBy identifies who requested the feature (optional).
	RequestedBy string `json:"requested_by,omitempty"`
}

// FeatureSpecification describes the feature to build.
type FeatureSpecification struct {
	// Title is the feature title (required).
	Title string `json:"title"`

	// Description is the detailed description (required).
	Description string `json:"description"`

	// Requirements are the feature requirements (optional).
	Requirements []string `json:"requirements,omitempty"`

	// AcceptanceCriteria are the acceptance criteria (optional).
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`

	// TargetFiles are hints for files to modify (optional).
	TargetFiles []string `json:"target_files,omitempty"`

	// Language is the primary language (optional).
	Language string `json:"language,omitempty"`
}

// FeatureResponse is the response for a feature request submission.
type FeatureResponse struct {
	// Status is the response status.
	Status string `json:"status"`

	// ExecutionID is the unique identifier for tracking this request.
	ExecutionID string `json:"execution_id,omitempty"`

	// Message provides additional information.
	Message string `json:"message"`

	// Errors contains validation errors if any.
	Errors []string `json:"errors,omitempty"`
}

// QueueFeatureRequest is the message format for the feature request queue.
// This matches what the worker service expects.
type QueueFeatureRequest struct {
	ExecutionID   string                    `json:"execution_id"`
	RepositoryURL string                    `json:"repository_url"`
	Branch        string                    `json:"branch"`
	Specification QueueFeatureSpecification `json:"specification"`
	RequestedBy   string                    `json:"requested_by,omitempty"`
	RequestedAt   time.Time                 `json:"requested_at"`
}

// QueueFeatureSpecification is the specification format for the queue.
type QueueFeatureSpecification struct {
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Requirements []string `json:"requirements,omitempty"`
	TargetFiles  []string `json:"target_files,omitempty"`
	Language     string   `json:"language,omitempty"`
}

// HandleFeatureRequest handles POST /api/v1/features requests.
func (h *FeatureHandler) HandleFeatureRequest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := util.Log(ctx)

	// Only allow POST
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed", nil)
		return
	}

	// Read request body with size limit
	maxSize := int64(h.cfg.MaxSpecificationSize)
	r.Body = http.MaxBytesReader(w, r.Body, maxSize)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		if strings.Contains(err.Error(), "http: request body too large") {
			h.writeError(w, http.StatusRequestEntityTooLarge,
				fmt.Sprintf("Request body exceeds maximum size of %d bytes", maxSize), nil)
			return
		}
		log.WithError(err).Error("failed to read request body")
		h.writeError(w, http.StatusBadRequest, "Failed to read request body", nil)
		return
	}
	defer util.CloseAndLogOnError(ctx, r.Body, "failed to close request body")

	// Parse JSON request
	var req FeatureRequest
	if err := json.Unmarshal(body, &req); err != nil {
		log.WithError(err).Debug("invalid JSON in request body")
		h.writeError(w, http.StatusBadRequest, "Invalid JSON in request body", []string{err.Error()})
		return
	}

	// Validate request
	validationErrors := h.validateRequest(&req)
	if len(validationErrors) > 0 {
		h.writeError(w, http.StatusBadRequest, "Validation failed", validationErrors)
		return
	}

	// Generate execution ID
	execID := events.NewExecutionID()

	// Build queue message
	queueMsg := &QueueFeatureRequest{
		ExecutionID:   execID.String(),
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
		RequestedAt: time.Now().UTC(),
	}

	// Merge acceptance criteria into requirements if present
	if len(req.Specification.AcceptanceCriteria) > 0 {
		queueMsg.Specification.Requirements = append(
			queueMsg.Specification.Requirements,
			req.Specification.AcceptanceCriteria...,
		)
	}

	// Marshal queue message
	data, err := json.Marshal(queueMsg)
	if err != nil {
		log.WithError(err).Error("failed to marshal queue message")
		h.writeError(w, http.StatusInternalServerError, "Internal server error", nil)
		return
	}

	// Get publisher and publish to queue
	publisher, err := h.queue.GetPublisher(h.cfg.QueueFeatureRequestName)
	if err != nil {
		log.WithError(err).Error("failed to get queue publisher")
		h.writeError(w, http.StatusInternalServerError, "Internal server error", nil)
		return
	}

	if err := publisher.Publish(ctx, data); err != nil {
		log.WithError(err).Error("failed to publish to queue")
		h.writeError(w, http.StatusInternalServerError, "Failed to queue request", nil)
		return
	}

	log.Info("feature request queued",
		"execution_id", execID.String(),
		"repository", req.RepositoryURL,
		"branch", req.Branch,
		"title", req.Specification.Title,
	)

	// Return success response
	h.writeSuccess(w, execID.String())
}

// validateRequest validates the feature request.
func (h *FeatureHandler) validateRequest(req *FeatureRequest) []string {
	var errors []string

	// Validate repository URL
	if req.RepositoryURL == "" {
		errors = append(errors, "repository_url is required")
	} else {
		if err := h.validateRepositoryURL(req.RepositoryURL); err != nil {
			errors = append(errors, err.Error())
		}
	}

	// Validate branch
	if req.Branch == "" {
		errors = append(errors, "branch is required")
	}

	// Validate specification
	if req.Specification.Title == "" {
		errors = append(errors, "specification.title is required")
	}
	if req.Specification.Description == "" {
		errors = append(errors, "specification.description is required")
	}

	return errors
}

// validateRepositoryURL validates the repository URL against allowed hosts.
func (h *FeatureHandler) validateRepositoryURL(repoURL string) error {
	parsedURL, err := url.Parse(repoURL)
	if err != nil {
		return fmt.Errorf("repository_url is not a valid URL: %s", err.Error())
	}

	// Must have a scheme
	if parsedURL.Scheme == "" {
		return fmt.Errorf("repository_url must include a scheme (https://)")
	}

	// Must be https (or allow http for development)
	if parsedURL.Scheme != "https" && parsedURL.Scheme != "http" {
		return fmt.Errorf("repository_url must use https or http scheme")
	}

	// Validate against allowed hosts
	allowedHosts := h.getAllowedHosts()
	host := strings.ToLower(parsedURL.Host)

	// Remove port if present
	if colonIdx := strings.Index(host, ":"); colonIdx != -1 {
		host = host[:colonIdx]
	}

	if slices.Contains(allowedHosts, host) {
		return nil
	}

	return fmt.Errorf("repository_url host %q is not in allowed hosts: %s",
		host, h.cfg.AllowedRepositoryHosts)
}

// getAllowedHosts parses the allowed repository hosts from config.
func (h *FeatureHandler) getAllowedHosts() []string {
	hosts := strings.Split(h.cfg.AllowedRepositoryHosts, ",")
	result := make([]string, 0, len(hosts))
	for _, host := range hosts {
		trimmed := strings.TrimSpace(strings.ToLower(host))
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// writeError writes an error response.
func (h *FeatureHandler) writeError(w http.ResponseWriter, statusCode int, message string, errors []string) {
	resp := FeatureResponse{
		Status:  "error",
		Message: message,
		Errors:  errors,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(resp)
}

// writeSuccess writes a success response.
func (h *FeatureHandler) writeSuccess(w http.ResponseWriter, executionID string) {
	resp := FeatureResponse{
		Status:      "accepted",
		ExecutionID: executionID,
		Message:     "Feature request queued for processing",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(resp)
}
