package events

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/pitabwire/util"
)

// DLQAPIHandler provides REST API endpoints for DLQ management.
type DLQAPIHandler struct {
	recoveryService DLQRecoveryService
}

// NewDLQAPIHandler creates a new DLQ API handler.
func NewDLQAPIHandler(recoveryService DLQRecoveryService) *DLQAPIHandler {
	return &DLQAPIHandler{
		recoveryService: recoveryService,
	}
}

// RegisterRoutes registers the DLQ API routes on the given mux.
func (h *DLQAPIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/dlq", h.handleDLQ)
	mux.HandleFunc("/api/v1/dlq/stats", h.handleStats)
	mux.HandleFunc("/api/v1/dlq/cleanup", h.handleCleanup)
	// Pattern matching for entry-specific routes
	mux.HandleFunc("/api/v1/dlq/", h.handleDLQEntry)
}

// handleDLQ handles GET /api/v1/dlq - list DLQ entries.
func (h *DLQAPIHandler) handleDLQ(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := util.Log(ctx)

	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Parse query parameters
	filter, filterErr := h.parseFilter(r)
	if filterErr != nil {
		h.writeError(w, http.StatusBadRequest, filterErr.Error())
		return
	}

	result, err := h.recoveryService.ListDLQEntries(ctx, filter)
	if err != nil {
		log.WithError(err).Error("failed to list DLQ entries")
		h.writeError(w, http.StatusInternalServerError, "failed to list entries")
		return
	}

	h.writeJSON(w, http.StatusOK, result)
}

// handleStats handles GET /api/v1/dlq/stats - get DLQ statistics.
func (h *DLQAPIHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := util.Log(ctx)

	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	stats, err := h.recoveryService.GetDLQStats(ctx)
	if err != nil {
		log.WithError(err).Error("failed to get DLQ stats")
		h.writeError(w, http.StatusInternalServerError, "failed to get stats")
		return
	}

	h.writeJSON(w, http.StatusOK, stats)
}

// handleCleanup handles POST /api/v1/dlq/cleanup - cleanup expired entries.
func (h *DLQAPIHandler) handleCleanup(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := util.Log(ctx)

	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	count, err := h.recoveryService.CleanupExpired(ctx)
	if err != nil {
		log.WithError(err).Error("failed to cleanup DLQ")
		h.writeError(w, http.StatusInternalServerError, "failed to cleanup")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]any{
		"removed": count,
	})
}

// handleDLQEntry handles operations on specific DLQ entries.
func (h *DLQAPIHandler) handleDLQEntry(w http.ResponseWriter, r *http.Request) {
	// Parse entry ID and action from path
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/dlq/")
	parts := strings.Split(path, "/")

	if len(parts) == 0 || parts[0] == "" {
		h.writeError(w, http.StatusBadRequest, "entry ID required")
		return
	}

	entryID := parts[0]
	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch {
	case r.Method == http.MethodGet && action == "":
		h.handleGetEntry(w, r, entryID)
	case r.Method == http.MethodPost && action == "requeue":
		h.handleRequeueEntry(w, r, entryID)
	case r.Method == http.MethodDelete && action == "":
		h.handleDiscardEntry(w, r, entryID)
	default:
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleGetEntry handles GET /api/v1/dlq/{id}.
func (h *DLQAPIHandler) handleGetEntry(w http.ResponseWriter, r *http.Request, entryID string) {
	ctx := r.Context()
	log := util.Log(ctx)

	entry, err := h.recoveryService.GetDLQEntry(ctx, entryID)
	if err != nil {
		if errors.Is(err, ErrDLQEntryNotFound) {
			h.writeError(w, http.StatusNotFound, "entry not found")
			return
		}
		log.WithError(err).Error("failed to get DLQ entry")
		h.writeError(w, http.StatusInternalServerError, "failed to get entry")
		return
	}
	h.writeJSON(w, http.StatusOK, entry)
}

// handleRequeueEntry handles POST /api/v1/dlq/{id}/requeue.
func (h *DLQAPIHandler) handleRequeueEntry(w http.ResponseWriter, r *http.Request, entryID string) {
	ctx := r.Context()
	log := util.Log(ctx)

	var req RequeueRequest
	if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil {
		h.writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.ResolvedBy == "" {
		h.writeError(w, http.StatusBadRequest, "resolved_by is required")
		return
	}

	if requeueErr := h.recoveryService.RequeueEntry(ctx, entryID, req); requeueErr != nil {
		if errors.Is(requeueErr, ErrDLQEntryNotFound) {
			h.writeError(w, http.StatusNotFound, "entry not found")
			return
		}
		if errors.Is(requeueErr, ErrDLQEntryResolved) {
			h.writeError(w, http.StatusConflict, "entry already resolved")
			return
		}
		log.WithError(requeueErr).Error("failed to requeue entry")
		h.writeError(w, http.StatusInternalServerError, "failed to requeue")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{
		"status": "requeued",
	})
}

// handleDiscardEntry handles DELETE /api/v1/dlq/{id}.
func (h *DLQAPIHandler) handleDiscardEntry(w http.ResponseWriter, r *http.Request, entryID string) {
	ctx := r.Context()
	log := util.Log(ctx)

	var req DiscardRequest
	if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil {
		// Allow empty body, but require query params
		req.ResolvedBy = r.URL.Query().Get("resolved_by")
		req.Notes = r.URL.Query().Get("notes")
	}

	if req.ResolvedBy == "" {
		h.writeError(w, http.StatusBadRequest, "resolved_by is required")
		return
	}

	if req.Notes == "" {
		h.writeError(w, http.StatusBadRequest, "notes are required for discard")
		return
	}

	if discardErr := h.recoveryService.DiscardEntry(ctx, entryID, req); discardErr != nil {
		if errors.Is(discardErr, ErrDLQEntryNotFound) {
			h.writeError(w, http.StatusNotFound, "entry not found")
			return
		}
		if errors.Is(discardErr, ErrDLQEntryResolved) {
			h.writeError(w, http.StatusConflict, "entry already resolved")
			return
		}
		log.WithError(discardErr).Error("failed to discard entry")
		h.writeError(w, http.StatusInternalServerError, "failed to discard")
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]string{
		"status": "discarded",
	})
}

// parseFilter parses DLQ filter from query parameters.
// Returns an error if numeric parameters (limit, offset) are malformed.
func (h *DLQAPIHandler) parseFilter(r *http.Request) (DLQFilter, error) {
	q := r.URL.Query()
	filter := DLQFilter{}

	if execID := q.Get("execution_id"); execID != "" {
		if id, err := ParseExecutionID(execID); err == nil {
			filter.ExecutionID = id
		}
	}

	if eventType := q.Get("event_type"); eventType != "" {
		filter.EventType = EventType(eventType)
	}

	if failureClass := q.Get("failure_class"); failureClass != "" {
		filter.FailureClass = DLQFailureClass(failureClass)
	}

	if q.Get("manual_review_only") == "true" {
		filter.ManualReviewOnly = true
	}

	if q.Get("include_resolved") == "true" {
		filter.IncludeResolved = true
	}

	if after := q.Get("entered_after"); after != "" {
		if t, err := time.Parse(time.RFC3339, after); err == nil {
			filter.EnteredAfter = t
		}
	}

	if before := q.Get("entered_before"); before != "" {
		if t, err := time.Parse(time.RFC3339, before); err == nil {
			filter.EnteredBefore = t
		}
	}

	if limit := q.Get("limit"); limit != "" {
		n, err := strconv.Atoi(limit)
		if err != nil {
			return DLQFilter{}, errors.New("invalid limit parameter: must be a number")
		}
		filter.Limit = n
	}

	if offset := q.Get("offset"); offset != "" {
		n, err := strconv.Atoi(offset)
		if err != nil {
			return DLQFilter{}, errors.New("invalid offset parameter: must be a number")
		}
		filter.Offset = n
	}

	return filter, nil
}

// writeJSON writes a JSON response.
func (h *DLQAPIHandler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// writeError writes a JSON error response.
func (h *DLQAPIHandler) writeError(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]string{
		"error": message,
	})
}
