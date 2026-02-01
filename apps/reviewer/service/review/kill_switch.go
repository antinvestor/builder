package review

import (
	"context"
	"sync"
	"time"

	appconfig "github.com/antinvestor/builder/apps/reviewer/config"
	"github.com/antinvestor/builder/internal/events"
	"github.com/pitabwire/util"
)

// =============================================================================
// Kill Switch State
// =============================================================================

// KillSwitchState represents the state of a kill switch.
type KillSwitchState struct {
	Active        bool                    `json:"active"`
	Reason        events.KillSwitchReason `json:"reason,omitempty"`
	ActivatedBy   string                  `json:"activated_by,omitempty"`
	Details       string                  `json:"details,omitempty"`
	ActivatedAt   time.Time               `json:"activated_at,omitempty"`
	DeactivatedBy string                  `json:"deactivated_by,omitempty"`
	DeactivatedAt time.Time               `json:"deactivated_at,omitempty"`
}

// FailureMetrics tracks failure metrics for automatic kill switch triggering.
type FailureMetrics struct {
	TotalRequests       int64
	FailedRequests      int64
	ConsecutiveFailures int
	LastFailureTime     time.Time
}

// =============================================================================
// Persistent Kill Switch Service
// =============================================================================

// PersistentKillSwitchService implements KillSwitchService with in-memory state.
// For production, this should be backed by a database or Redis.
type PersistentKillSwitchService struct {
	cfg       *appconfig.ReviewerConfig
	eventsMan EventsEmitter

	mu sync.RWMutex

	// Global kill switch state
	globalState KillSwitchState

	// Repository-scoped kill switches
	repositoryStates map[string]KillSwitchState

	// Execution-scoped kill switches
	executionStates map[string]KillSwitchState

	// Failure metrics for automatic triggering
	failureMetrics FailureMetrics

	// History for audit trail
	activationHistory []KillSwitchActivation
}

// KillSwitchActivation records an activation event.
type KillSwitchActivation struct {
	Timestamp   time.Time               `json:"timestamp"`
	Action      string                  `json:"action"` // "activate" or "deactivate"
	Scope       events.KillSwitchScope  `json:"scope"`
	ScopeID     string                  `json:"scope_id,omitempty"`
	Reason      events.KillSwitchReason `json:"reason,omitempty"`
	ActivatedBy string                  `json:"activated_by"`
	Details     string                  `json:"details,omitempty"`
}

// NewPersistentKillSwitchService creates a new persistent kill switch service.
func NewPersistentKillSwitchService(cfg *appconfig.ReviewerConfig, eventsMan EventsEmitter) *PersistentKillSwitchService {
	return &PersistentKillSwitchService{
		cfg:               cfg,
		eventsMan:         eventsMan,
		repositoryStates:  make(map[string]KillSwitchState),
		executionStates:   make(map[string]KillSwitchState),
		activationHistory: make([]KillSwitchActivation, 0),
	}
}

// =============================================================================
// KillSwitchService Interface Implementation
// =============================================================================

// IsActive checks if kill switch is active for the given execution and repository.
// Returns the active status, reason, and scope.
func (s *PersistentKillSwitchService) IsActive(
	ctx context.Context,
	executionID events.ExecutionID,
	repositoryID string,
) (bool, events.KillSwitchReason, events.KillSwitchScope) {
	log := util.Log(ctx)

	// Check if kill switch is disabled
	if !s.cfg.KillSwitchEnabled {
		return false, "", ""
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Check global kill switch first (highest priority)
	if s.globalState.Active {
		log.Debug("global kill switch is active",
			"reason", s.globalState.Reason,
			"execution_id", executionID,
		)
		return true, s.globalState.Reason, events.KillSwitchScopeGlobal
	}

	// Check repository-scoped kill switch
	if repositoryID != "" {
		if state, exists := s.repositoryStates[repositoryID]; exists && state.Active {
			log.Debug("repository kill switch is active",
				"reason", state.Reason,
				"repository_id", repositoryID,
				"execution_id", executionID,
			)
			return true, state.Reason, events.KillSwitchScopeRepository
		}
	}

	// Check execution-scoped kill switch
	execIDStr := executionID.String()
	if state, exists := s.executionStates[execIDStr]; exists && state.Active {
		log.Debug("execution kill switch is active",
			"reason", state.Reason,
			"execution_id", executionID,
		)
		return true, state.Reason, events.KillSwitchScopeFeature
	}

	return false, "", ""
}

// GetStatus returns the current kill switch status.
func (s *PersistentKillSwitchService) GetStatus(ctx context.Context) (*events.KillSwitchStatusPayload, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := &events.KillSwitchStatusPayload{
		GlobalActive: s.globalState.Active,
		GlobalReason: s.globalState.Reason,
	}

	// Add feature/execution switches
	if len(s.executionStates) > 0 {
		status.FeatureSwitches = make(map[string]events.FeatureKillSwitch)
		for execID, state := range s.executionStates {
			if state.Active {
				status.FeatureSwitches[execID] = events.FeatureKillSwitch{
					Active:      state.Active,
					Reason:      state.Reason,
					ActivatedAt: state.ActivatedAt,
					ActivatedBy: state.ActivatedBy,
				}
			}
		}
	}

	// Add repository switches (map[string]bool in the API)
	if len(s.repositoryStates) > 0 {
		status.RepositorySwitches = make(map[string]bool)
		for repoID, state := range s.repositoryStates {
			status.RepositorySwitches[repoID] = state.Active
		}
	}

	return status, nil
}

// ActivateGlobal activates the global kill switch.
func (s *PersistentKillSwitchService) ActivateGlobal(
	ctx context.Context,
	reason events.KillSwitchReason,
	activatedBy, details string,
) error {
	log := util.Log(ctx)

	s.mu.Lock()
	s.globalState = KillSwitchState{
		Active:      true,
		Reason:      reason,
		ActivatedBy: activatedBy,
		Details:     details,
		ActivatedAt: time.Now(),
	}

	// Record in history
	s.activationHistory = append(s.activationHistory, KillSwitchActivation{
		Timestamp:   time.Now(),
		Action:      "activate",
		Scope:       events.KillSwitchScopeGlobal,
		Reason:      reason,
		ActivatedBy: activatedBy,
		Details:     details,
	})
	s.mu.Unlock()

	log.Info("global kill switch activated",
		"reason", reason,
		"activated_by", activatedBy,
		"details", details,
	)

	// Emit event
	if s.eventsMan != nil {
		_ = s.eventsMan.Emit(ctx, "feature.kill_switch.activated", &events.KillSwitchActivatedPayload{
			Scope:       events.KillSwitchScopeGlobal,
			Reason:      reason,
			ActivatedBy: activatedBy,
			Details:     details,
			ActivatedAt: time.Now(),
		})
	}

	return nil
}

// DeactivateGlobal deactivates the global kill switch.
func (s *PersistentKillSwitchService) DeactivateGlobal(
	ctx context.Context,
	deactivatedBy, reason string,
) error {
	log := util.Log(ctx)

	s.mu.Lock()
	s.globalState = KillSwitchState{
		Active:        false,
		DeactivatedBy: deactivatedBy,
		DeactivatedAt: time.Now(),
	}

	// Record in history
	s.activationHistory = append(s.activationHistory, KillSwitchActivation{
		Timestamp:   time.Now(),
		Action:      "deactivate",
		Scope:       events.KillSwitchScopeGlobal,
		ActivatedBy: deactivatedBy,
		Details:     reason,
	})
	s.mu.Unlock()

	log.Info("global kill switch deactivated",
		"deactivated_by", deactivatedBy,
		"reason", reason,
	)

	// Emit event
	if s.eventsMan != nil {
		_ = s.eventsMan.Emit(ctx, "feature.kill_switch.deactivated", &events.KillSwitchDeactivatedPayload{
			Scope:         events.KillSwitchScopeGlobal,
			DeactivatedBy: deactivatedBy,
			Reason:        reason,
			DeactivatedAt: time.Now(),
		})
	}

	return nil
}

// =============================================================================
// Scope-Specific Activation/Deactivation
// =============================================================================

// ActivateForRepository activates the kill switch for a specific repository.
func (s *PersistentKillSwitchService) ActivateForRepository(
	ctx context.Context,
	repositoryID string,
	reason events.KillSwitchReason,
	activatedBy, details string,
) error {
	log := util.Log(ctx)

	s.mu.Lock()
	s.repositoryStates[repositoryID] = KillSwitchState{
		Active:      true,
		Reason:      reason,
		ActivatedBy: activatedBy,
		Details:     details,
		ActivatedAt: time.Now(),
	}

	// Record in history
	s.activationHistory = append(s.activationHistory, KillSwitchActivation{
		Timestamp:   time.Now(),
		Action:      "activate",
		Scope:       events.KillSwitchScopeRepository,
		ScopeID:     repositoryID,
		Reason:      reason,
		ActivatedBy: activatedBy,
		Details:     details,
	})
	s.mu.Unlock()

	log.Info("repository kill switch activated",
		"repository_id", repositoryID,
		"reason", reason,
		"activated_by", activatedBy,
	)

	// Emit event (note: API doesn't have RepositoryID field, so we put it in Details)
	if s.eventsMan != nil {
		_ = s.eventsMan.Emit(ctx, "feature.kill_switch.activated", &events.KillSwitchActivatedPayload{
			Scope:       events.KillSwitchScopeRepository,
			Reason:      reason,
			ActivatedBy: activatedBy,
			Details:     "repository:" + repositoryID + "; " + details,
			ActivatedAt: time.Now(),
		})
	}

	return nil
}

// DeactivateForRepository deactivates the kill switch for a specific repository.
func (s *PersistentKillSwitchService) DeactivateForRepository(
	ctx context.Context,
	repositoryID string,
	deactivatedBy, reasonStr string,
) error {
	log := util.Log(ctx)

	s.mu.Lock()
	delete(s.repositoryStates, repositoryID)

	// Record in history
	s.activationHistory = append(s.activationHistory, KillSwitchActivation{
		Timestamp:   time.Now(),
		Action:      "deactivate",
		Scope:       events.KillSwitchScopeRepository,
		ScopeID:     repositoryID,
		ActivatedBy: deactivatedBy,
		Details:     reasonStr,
	})
	s.mu.Unlock()

	log.Info("repository kill switch deactivated",
		"repository_id", repositoryID,
		"deactivated_by", deactivatedBy,
	)

	// Emit event (note: API doesn't have RepositoryID field, so we put it in Reason)
	if s.eventsMan != nil {
		_ = s.eventsMan.Emit(ctx, "feature.kill_switch.deactivated", &events.KillSwitchDeactivatedPayload{
			Scope:         events.KillSwitchScopeRepository,
			DeactivatedBy: deactivatedBy,
			Reason:        "repository:" + repositoryID + "; " + reasonStr,
			DeactivatedAt: time.Now(),
		})
	}

	return nil
}

// ActivateForExecution activates the kill switch for a specific execution.
func (s *PersistentKillSwitchService) ActivateForExecution(
	ctx context.Context,
	executionID events.ExecutionID,
	reason events.KillSwitchReason,
	activatedBy, details string,
) error {
	log := util.Log(ctx)
	execIDStr := executionID.String()

	s.mu.Lock()
	s.executionStates[execIDStr] = KillSwitchState{
		Active:      true,
		Reason:      reason,
		ActivatedBy: activatedBy,
		Details:     details,
		ActivatedAt: time.Now(),
	}

	// Record in history
	s.activationHistory = append(s.activationHistory, KillSwitchActivation{
		Timestamp:   time.Now(),
		Action:      "activate",
		Scope:       events.KillSwitchScopeFeature,
		ScopeID:     execIDStr,
		Reason:      reason,
		ActivatedBy: activatedBy,
		Details:     details,
	})
	s.mu.Unlock()

	log.Info("execution kill switch activated",
		"execution_id", executionID,
		"reason", reason,
		"activated_by", activatedBy,
	)

	// Emit event
	if s.eventsMan != nil {
		_ = s.eventsMan.Emit(ctx, "feature.kill_switch.activated", &events.KillSwitchActivatedPayload{
			Scope:       events.KillSwitchScopeFeature,
			ExecutionID: executionID,
			Reason:      reason,
			ActivatedBy: activatedBy,
			Details:     details,
			ActivatedAt: time.Now(),
		})
	}

	return nil
}

// DeactivateForExecution deactivates the kill switch for a specific execution.
func (s *PersistentKillSwitchService) DeactivateForExecution(
	ctx context.Context,
	executionID events.ExecutionID,
	deactivatedBy, reason string,
) error {
	log := util.Log(ctx)
	execIDStr := executionID.String()

	s.mu.Lock()
	delete(s.executionStates, execIDStr)

	// Record in history
	s.activationHistory = append(s.activationHistory, KillSwitchActivation{
		Timestamp:   time.Now(),
		Action:      "deactivate",
		Scope:       events.KillSwitchScopeFeature,
		ScopeID:     execIDStr,
		ActivatedBy: deactivatedBy,
		Details:     reason,
	})
	s.mu.Unlock()

	log.Info("execution kill switch deactivated",
		"execution_id", executionID,
		"deactivated_by", deactivatedBy,
	)

	// Emit event
	if s.eventsMan != nil {
		_ = s.eventsMan.Emit(ctx, "feature.kill_switch.deactivated", &events.KillSwitchDeactivatedPayload{
			Scope:         events.KillSwitchScopeFeature,
			ExecutionID:   executionID,
			DeactivatedBy: deactivatedBy,
			Reason:        reason,
			DeactivatedAt: time.Now(),
		})
	}

	return nil
}

// =============================================================================
// Automatic Triggering
// =============================================================================

// RecordSuccess records a successful operation for failure rate tracking.
func (s *PersistentKillSwitchService) RecordSuccess(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.failureMetrics.TotalRequests++
	s.failureMetrics.ConsecutiveFailures = 0
}

// RecordFailure records a failed operation and checks if kill switch should activate.
// Returns true if kill switch was auto-activated.
func (s *PersistentKillSwitchService) RecordFailure(ctx context.Context) bool {
	log := util.Log(ctx)

	if !s.cfg.KillSwitchEnabled {
		return false
	}

	s.mu.Lock()
	s.failureMetrics.TotalRequests++
	s.failureMetrics.FailedRequests++
	s.failureMetrics.ConsecutiveFailures++
	s.failureMetrics.LastFailureTime = time.Now()

	totalRequests := s.failureMetrics.TotalRequests
	failedRequests := s.failureMetrics.FailedRequests
	consecutiveFailures := s.failureMetrics.ConsecutiveFailures
	s.mu.Unlock()

	// Check consecutive failures threshold
	if consecutiveFailures >= s.cfg.MaxConsecutiveFailures {
		log.Warn("consecutive failures threshold reached, activating kill switch",
			"consecutive_failures", consecutiveFailures,
			"threshold", s.cfg.MaxConsecutiveFailures,
		)
		_ = s.ActivateGlobal(ctx,
			events.KillSwitchReasonSystemOverload,
			"automatic",
			"consecutive failures threshold exceeded",
		)
		return true
	}

	// Check error rate threshold (only if we have enough data)
	if totalRequests >= 10 { // Minimum sample size
		errorRate := float64(failedRequests) / float64(totalRequests)
		if errorRate >= s.cfg.ErrorRateThreshold {
			log.Warn("error rate threshold reached, activating kill switch",
				"error_rate", errorRate,
				"threshold", s.cfg.ErrorRateThreshold,
				"total_requests", totalRequests,
				"failed_requests", failedRequests,
			)
			_ = s.ActivateGlobal(ctx,
				events.KillSwitchReasonSystemOverload,
				"automatic",
				"error rate threshold exceeded",
			)
			return true
		}
	}

	return false
}

// ResetMetrics resets the failure metrics (useful after resolving issues).
func (s *PersistentKillSwitchService) ResetMetrics(ctx context.Context) {
	log := util.Log(ctx)

	s.mu.Lock()
	s.failureMetrics = FailureMetrics{}
	s.mu.Unlock()

	log.Info("kill switch metrics reset")
}

// =============================================================================
// Audit and Monitoring
// =============================================================================

// GetActivationHistory returns the activation history.
func (s *PersistentKillSwitchService) GetActivationHistory(ctx context.Context) []KillSwitchActivation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy
	history := make([]KillSwitchActivation, len(s.activationHistory))
	copy(history, s.activationHistory)
	return history
}

// GetMetrics returns current failure metrics.
func (s *PersistentKillSwitchService) GetMetrics(ctx context.Context) FailureMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.failureMetrics
}
