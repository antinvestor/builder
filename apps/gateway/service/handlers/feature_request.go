package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/pitabwire/util"
	"github.com/rs/xid"

	appconfig "github.com/antinvestor/builder/apps/gateway/config"
	"github.com/antinvestor/builder/apps/gateway/middleware"
)

// Validation constants.
const (
	maxTitleLength       = 200
	maxDescriptionLength = 10240 // 10KB
)

// QueuePublisher defines the interface for publishing messages to a queue.
type QueuePublisher interface {
	Publish(ctx context.Context, queueName string, payload any, headers ...map[string]string) error
}

// FeatureRequestHandler handles incoming feature requests.
type FeatureRequestHandler struct {
	cfg       *appconfig.GatewayConfig
	publisher QueuePublisher
}

// NewFeatureRequestHandler creates a new feature request handler.
func NewFeatureRequestHandler(
	cfg *appconfig.GatewayConfig,
	publisher QueuePublisher,
) *FeatureRequestHandler {
	return &FeatureRequestHandler{
		cfg:       cfg,
		publisher: publisher,
	}
}

// FeatureRequest represents an incoming feature request from API clients.
type FeatureRequest struct {
	// RepositoryURL is the git repository URL (required).
	RepositoryURL string `json:"repository_url"`

	// Branch is the target branch to build upon (required).
	Branch string `json:"branch"`

	// Specification contains the feature details (required).
	Specification FeatureSpecification `json:"specification"`

	// RequestedBy identifies who made the request (optional, defaults to authenticated user).
	RequestedBy string `json:"requested_by,omitempty"`

	// Priority is the request priority (optional).
	Priority string `json:"priority,omitempty"`

	// Constraints are optional execution constraints.
	Constraints *ExecutionConstraints `json:"constraints,omitempty"`
}

// FeatureSpecification describes the feature to build.
type FeatureSpecification struct {
	// Title is a human-readable title for the feature (required).
	Title string `json:"title"`

	// Description is the detailed feature description (required).
	Description string `json:"description"`

	// AcceptanceCriteria are specific criteria to satisfy (optional).
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`

	// TargetFiles are file path hints (optional).
	TargetFiles []string `json:"target_files,omitempty"`

	// Language is the primary programming language (optional).
	Language string `json:"language,omitempty"`

	// Category is the feature category (optional).
	Category string `json:"category,omitempty"`

	// AdditionalContext is extra context for the LLM (optional).
	AdditionalContext string `json:"additional_context,omitempty"`
}

// ExecutionConstraints define optional execution limits.
type ExecutionConstraints struct {
	// MaxSteps limits the number of implementation steps.
	MaxSteps int `json:"max_steps,omitempty"`

	// TimeoutMinutes is the execution timeout in minutes.
	TimeoutMinutes int `json:"timeout_minutes,omitempty"`

	// MaxLLMTokens limits the LLM tokens to consume.
	MaxLLMTokens int `json:"max_llm_tokens,omitempty"`

	// AllowedPaths are glob patterns for allowed modifications.
	AllowedPaths []string `json:"allowed_paths,omitempty"`

	// ForbiddenPaths are glob patterns for forbidden modifications.
	ForbiddenPaths []string `json:"forbidden_paths,omitempty"`
}

// FeatureResponse is the response returned to API clients.
type FeatureResponse struct {
	Status      string `json:"status"`
	ExecutionID string `json:"execution_id,omitempty"`
	Message     string `json:"message"`
}

// ErrorResponse is the error response returned to API clients.
type ErrorResponse struct {
	Error   string            `json:"error"`
	Message string            `json:"message"`
	Details map[string]string `json:"details,omitempty"`
}

// QueueFeatureRequest is the message format sent to the worker queue.
type QueueFeatureRequest struct {
	ExecutionID   string                `json:"execution_id"`
	RepositoryURL string                `json:"repository_url"`
	Branch        string                `json:"branch"`
	Specification FeatureSpecification  `json:"specification"`
	RequestedBy   string                `json:"requested_by"`
	RequestedAt   time.Time             `json:"requested_at"`
	Priority      string                `json:"priority,omitempty"`
	Constraints   *ExecutionConstraints `json:"constraints,omitempty"`
}

// ValidationError represents a validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ServeHTTP handles the HTTP request for feature creation.
func (h *FeatureRequestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := util.Log(ctx)

	// Only allow POST method
	if r.Method != http.MethodPost {
		h.writeErrorResponse(w, http.StatusMethodNotAllowed, "method_not_allowed",
			"Only POST method is allowed", nil)
		return
	}

	// Get authenticated user
	claims := middleware.GetUserFromContext(ctx)
	userID := ""
	if claims != nil {
		userID, _ = claims.GetSubject()
	}

	// Read and validate request body size
	bodyReader := http.MaxBytesReader(w, r.Body, int64(h.cfg.MaxSpecificationSize))
	body, err := io.ReadAll(bodyReader)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			h.writeErrorResponse(
				w,
				http.StatusRequestEntityTooLarge,
				"request_too_large",
				fmt.Sprintf(
					"Request body exceeds maximum size of %d bytes",
					h.cfg.MaxSpecificationSize,
				),
				nil,
			)
			return
		}
		h.writeErrorResponse(w, http.StatusBadRequest, "invalid_request",
			"Failed to read request body", nil)
		return
	}

	// Parse request
	var request FeatureRequest
	if unmarshalErr := json.Unmarshal(body, &request); unmarshalErr != nil {
		h.writeErrorResponse(
			w,
			http.StatusBadRequest,
			"invalid_json",
			"Failed to parse JSON request body",
			map[string]string{"parse_error": unmarshalErr.Error()},
		)
		return
	}

	// Validate request
	if validationErr := h.validateRequest(&request); validationErr != nil {
		h.writeErrorResponse(w, http.StatusBadRequest, "validation_error",
			validationErr.Error(), map[string]string{"field": validationErr.Field})
		return
	}

	// Set requester from auth if not provided
	if request.RequestedBy == "" {
		request.RequestedBy = userID
	}

	// Generate execution ID
	executionID := xid.New().String()

	// Create queue message
	queueRequest := QueueFeatureRequest{
		ExecutionID:   executionID,
		RepositoryURL: request.RepositoryURL,
		Branch:        request.Branch,
		Specification: request.Specification,
		RequestedBy:   request.RequestedBy,
		RequestedAt:   time.Now(),
		Priority:      request.Priority,
		Constraints:   request.Constraints,
	}

	// Publish to queue
	if publishErr := h.publisher.Publish(ctx, h.cfg.QueueFeatureRequestName, queueRequest); publishErr != nil {
		log.WithError(publishErr).Error("failed to publish feature request to queue",
			"execution_id", executionID,
			"repository_url", request.RepositoryURL,
		)
		h.writeErrorResponse(w, http.StatusInternalServerError, "queue_error",
			"Failed to queue feature request for processing", nil)
		return
	}

	log.Info("feature request queued",
		"execution_id", executionID,
		"repository_url", request.RepositoryURL,
		"branch", request.Branch,
		"title", request.Specification.Title,
		"requested_by", request.RequestedBy,
	)

	// Return success response
	h.writeSuccessResponse(w, http.StatusAccepted, FeatureResponse{
		Status:      "accepted",
		ExecutionID: executionID,
		Message:     "Feature request queued for processing",
	})
}

// validateRequest validates the feature request.
func (h *FeatureRequestHandler) validateRequest(req *FeatureRequest) *ValidationError {
	if err := h.validateRepositoryURL(req.RepositoryURL); err != nil {
		return err
	}

	if req.Branch == "" {
		return &ValidationError{Field: "branch", Message: "branch is required"}
	}

	if err := h.validateSpecification(&req.Specification); err != nil {
		return err
	}

	if err := validatePriority(req.Priority); err != nil {
		return err
	}

	if err := validateConstraints(req.Constraints); err != nil {
		return err
	}

	return nil
}

// validateRepositoryURL validates the repository URL.
func (h *FeatureRequestHandler) validateRepositoryURL(repoURL string) *ValidationError {
	if repoURL == "" {
		return &ValidationError{Field: "repository_url", Message: "repository URL is required"}
	}

	parsedURL, err := url.Parse(repoURL)
	if err != nil {
		return &ValidationError{Field: "repository_url", Message: "invalid repository URL format"}
	}

	if !h.isAllowedHost(parsedURL.Host) {
		return &ValidationError{
			Field: "repository_url",
			Message: fmt.Sprintf(
				"repository host '%s' is not allowed. Allowed hosts: %s",
				parsedURL.Host,
				h.cfg.AllowedRepositoryHosts,
			),
		}
	}

	return nil
}

// validateSpecification validates the feature specification.
func (h *FeatureRequestHandler) validateSpecification(spec *FeatureSpecification) *ValidationError {
	if spec.Title == "" {
		return &ValidationError{
			Field:   "specification.title",
			Message: "specification title is required",
		}
	}

	if spec.Description == "" {
		return &ValidationError{
			Field:   "specification.description",
			Message: "specification description is required",
		}
	}

	if len(spec.Title) > maxTitleLength {
		return &ValidationError{
			Field:   "specification.title",
			Message: fmt.Sprintf("title must be %d characters or less", maxTitleLength),
		}
	}

	if len(spec.Description) > maxDescriptionLength {
		return &ValidationError{
			Field:   "specification.description",
			Message: fmt.Sprintf("description must be %d bytes or less", maxDescriptionLength),
		}
	}

	if err := validateCategory(spec.Category); err != nil {
		return err
	}

	return nil
}

// validateCategory validates the feature category.
func validateCategory(category string) *ValidationError {
	if category == "" {
		return nil
	}

	validCategories := map[string]bool{
		"new_feature": true, "bug_fix": true, "refactor": true,
		"documentation": true, "test": true, "dependency": true,
	}
	if !validCategories[category] {
		return &ValidationError{
			Field:   "specification.category",
			Message: "invalid category. Valid values: new_feature, bug_fix, refactor, documentation, test, dependency",
		}
	}

	return nil
}

// validatePriority validates the request priority.
func validatePriority(priority string) *ValidationError {
	if priority == "" {
		return nil
	}

	validPriorities := map[string]bool{
		"low": true, "normal": true, "high": true, "critical": true,
	}
	if !validPriorities[priority] {
		return &ValidationError{
			Field:   "priority",
			Message: "invalid priority. Valid values: low, normal, high, critical",
		}
	}

	return nil
}

// validateConstraints validates the execution constraints.
func validateConstraints(constraints *ExecutionConstraints) *ValidationError {
	if constraints == nil {
		return nil
	}

	if constraints.MaxSteps < 0 {
		return &ValidationError{
			Field:   "constraints.max_steps",
			Message: "max_steps must be non-negative",
		}
	}
	if constraints.TimeoutMinutes < 0 {
		return &ValidationError{
			Field:   "constraints.timeout_minutes",
			Message: "timeout_minutes must be non-negative",
		}
	}
	if constraints.MaxLLMTokens < 0 {
		return &ValidationError{
			Field:   "constraints.max_llm_tokens",
			Message: "max_llm_tokens must be non-negative",
		}
	}

	return nil
}

// isAllowedHost checks if the host is in the allowed hosts list.
func (h *FeatureRequestHandler) isAllowedHost(host string) bool {
	allowedHosts := strings.Split(h.cfg.AllowedRepositoryHosts, ",")
	for _, allowed := range allowedHosts {
		allowed = strings.TrimSpace(allowed)
		if strings.EqualFold(host, allowed) {
			return true
		}
	}
	return false
}

// writeSuccessResponse writes a success JSON response.
func (h *FeatureRequestHandler) writeSuccessResponse(
	w http.ResponseWriter,
	statusCode int,
	response FeatureResponse,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(response)
}

// writeErrorResponse writes an error JSON response.
func (h *FeatureRequestHandler) writeErrorResponse(
	w http.ResponseWriter,
	statusCode int,
	errorCode, message string,
	details map[string]string,
) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(ErrorResponse{
		Error:   errorCode,
		Message: message,
		Details: details,
	})
}
