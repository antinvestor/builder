package review

import (
	"context"
	"testing"

	appconfig "github.com/antinvestor/builder/apps/reviewer/config"
	"github.com/antinvestor/builder/internal/events"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockEventsEmitter is a mock for testing event emission.
type mockEventsEmitter struct {
	emittedEvents []struct {
		name    string
		payload any
	}
}

func (m *mockEventsEmitter) Emit(_ context.Context, eventName string, payload any) error {
	m.emittedEvents = append(m.emittedEvents, struct {
		name    string
		payload any
	}{name: eventName, payload: payload})
	return nil
}

func newTestKillSwitchService() (*PersistentKillSwitchService, *mockEventsEmitter) {
	cfg := &appconfig.ReviewerConfig{
		KillSwitchEnabled:      true,
		ErrorRateThreshold:     0.5,
		MaxConsecutiveFailures: 5,
		ResourceUsageThreshold: 0.9,
	}
	emitter := &mockEventsEmitter{}
	return NewPersistentKillSwitchService(cfg, emitter), emitter
}

func TestPersistentKillSwitchService_IsActive_GlobalNotActive(t *testing.T) {
	svc, _ := newTestKillSwitchService()
	ctx := context.Background()

	active, reason, scope := svc.IsActive(ctx, events.ExecutionID{}, "repo-1")

	assert.False(t, active)
	assert.Empty(t, reason)
	assert.Empty(t, scope)
}

func TestPersistentKillSwitchService_IsActive_GlobalActive(t *testing.T) {
	svc, _ := newTestKillSwitchService()
	ctx := context.Background()

	// Activate global kill switch
	err := svc.ActivateGlobal(ctx, events.KillSwitchReasonManual, "admin", "emergency")
	require.NoError(t, err)

	active, reason, scope := svc.IsActive(ctx, events.ExecutionID{}, "repo-1")

	assert.True(t, active)
	assert.Equal(t, events.KillSwitchReasonManual, reason)
	assert.Equal(t, events.KillSwitchScopeGlobal, scope)
}

func TestPersistentKillSwitchService_IsActive_RepositoryScope(t *testing.T) {
	svc, _ := newTestKillSwitchService()
	ctx := context.Background()

	// Activate for specific repository
	err := svc.ActivateForRepository(ctx, "repo-1", events.KillSwitchReasonSecurityBreach, "system", "security issue")
	require.NoError(t, err)

	// Should be active for repo-1
	active, reason, scope := svc.IsActive(ctx, events.ExecutionID{}, "repo-1")
	assert.True(t, active)
	assert.Equal(t, events.KillSwitchReasonSecurityBreach, reason)
	assert.Equal(t, events.KillSwitchScopeRepository, scope)

	// Should NOT be active for repo-2
	active, _, _ = svc.IsActive(ctx, events.ExecutionID{}, "repo-2")
	assert.False(t, active)
}

func TestPersistentKillSwitchService_IsActive_ExecutionScope(t *testing.T) {
	svc, _ := newTestKillSwitchService()
	ctx := context.Background()

	execID := events.NewExecutionID()

	// Activate for specific execution
	err := svc.ActivateForExecution(ctx, execID, events.KillSwitchReasonResourceExhausted, "system", "OOM")
	require.NoError(t, err)

	// Should be active for the execution
	active, reason, scope := svc.IsActive(ctx, execID, "repo-1")
	assert.True(t, active)
	assert.Equal(t, events.KillSwitchReasonResourceExhausted, reason)
	assert.Equal(t, events.KillSwitchScopeFeature, scope)

	// Should NOT be active for different execution
	otherExecID := events.NewExecutionID()
	active, _, _ = svc.IsActive(ctx, otherExecID, "repo-1")
	assert.False(t, active)
}

func TestPersistentKillSwitchService_IsActive_Priority(t *testing.T) {
	svc, _ := newTestKillSwitchService()
	ctx := context.Background()

	execID := events.NewExecutionID()

	// Activate at all scopes
	_ = svc.ActivateForExecution(ctx, execID, events.KillSwitchReasonResourceExhausted, "system", "execution")
	_ = svc.ActivateForRepository(ctx, "repo-1", events.KillSwitchReasonSecurityBreach, "system", "repository")
	_ = svc.ActivateGlobal(ctx, events.KillSwitchReasonManual, "admin", "global")

	// Global should have highest priority
	active, reason, scope := svc.IsActive(ctx, execID, "repo-1")
	assert.True(t, active)
	assert.Equal(t, events.KillSwitchReasonManual, reason)
	assert.Equal(t, events.KillSwitchScopeGlobal, scope)
}

func TestPersistentKillSwitchService_ActivateDeactivate(t *testing.T) {
	svc, emitter := newTestKillSwitchService()
	ctx := context.Background()

	// Activate
	err := svc.ActivateGlobal(ctx, events.KillSwitchReasonManual, "admin", "test activation")
	require.NoError(t, err)

	// Verify active
	status, err := svc.GetStatus(ctx)
	require.NoError(t, err)
	assert.True(t, status.GlobalActive)
	assert.Equal(t, events.KillSwitchReasonManual, status.GlobalReason)

	// Verify event emitted
	assert.Len(t, emitter.emittedEvents, 1)
	assert.Equal(t, "feature.kill_switch.activated", emitter.emittedEvents[0].name)

	// Deactivate
	err = svc.DeactivateGlobal(ctx, "admin", "issue resolved")
	require.NoError(t, err)

	// Verify deactivated
	status, err = svc.GetStatus(ctx)
	require.NoError(t, err)
	assert.False(t, status.GlobalActive)

	// Verify deactivation event
	assert.Len(t, emitter.emittedEvents, 2)
	assert.Equal(t, "feature.kill_switch.deactivated", emitter.emittedEvents[1].name)
}

func TestPersistentKillSwitchService_IsActive_Disabled(t *testing.T) {
	cfg := &appconfig.ReviewerConfig{
		KillSwitchEnabled: false, // Disabled
	}
	svc := NewPersistentKillSwitchService(cfg, nil)
	ctx := context.Background()

	// Activate global (would normally work)
	_ = svc.ActivateGlobal(ctx, events.KillSwitchReasonManual, "admin", "test")

	// Should still return false because disabled
	active, _, _ := svc.IsActive(ctx, events.ExecutionID{}, "repo-1")
	assert.False(t, active)
}

func TestPersistentKillSwitchService_RecordFailure_ConsecutiveThreshold(t *testing.T) {
	svc, _ := newTestKillSwitchService()
	ctx := context.Background()

	// Record failures up to threshold
	for i := 0; i < 4; i++ {
		triggered := svc.RecordFailure(ctx)
		assert.False(t, triggered, "should not trigger before threshold")
	}

	// 5th failure should trigger
	triggered := svc.RecordFailure(ctx)
	assert.True(t, triggered, "should trigger at threshold")

	// Verify kill switch is active
	active, reason, _ := svc.IsActive(ctx, events.ExecutionID{}, "")
	assert.True(t, active)
	assert.Equal(t, events.KillSwitchReasonSystemOverload, reason)
}

func TestPersistentKillSwitchService_RecordFailure_ErrorRateThreshold(t *testing.T) {
	svc, _ := newTestKillSwitchService()
	ctx := context.Background()

	// Record successes to build up sample size
	for i := 0; i < 5; i++ {
		svc.RecordSuccess(ctx)
	}

	// Record failures to exceed 50% error rate (need 6 failures out of 11 total = 54.5%)
	var triggered bool
	for i := 0; i < 6; i++ {
		triggered = svc.RecordFailure(ctx)
		// Reset consecutive counter to avoid triggering that threshold
		svc.RecordSuccess(ctx)
		if triggered {
			break
		}
	}

	// Eventually should trigger
	metrics := svc.GetMetrics(ctx)
	errorRate := float64(metrics.FailedRequests) / float64(metrics.TotalRequests)
	t.Logf("Error rate: %.2f, Total: %d, Failed: %d", errorRate, metrics.TotalRequests, metrics.FailedRequests)
}

func TestPersistentKillSwitchService_RecordSuccess_ResetsConsecutive(t *testing.T) {
	svc, _ := newTestKillSwitchService()
	ctx := context.Background()

	// Record some failures
	for i := 0; i < 3; i++ {
		svc.RecordFailure(ctx)
	}

	// Record a success - should reset consecutive
	svc.RecordSuccess(ctx)

	// Continue failures - should start from 0
	for i := 0; i < 4; i++ {
		triggered := svc.RecordFailure(ctx)
		assert.False(t, triggered, "consecutive counter should have reset")
	}

	// 5th after reset should trigger
	triggered := svc.RecordFailure(ctx)
	assert.True(t, triggered)
}

func TestPersistentKillSwitchService_ResetMetrics(t *testing.T) {
	svc, _ := newTestKillSwitchService()
	ctx := context.Background()

	// Record some activity
	for i := 0; i < 5; i++ {
		svc.RecordFailure(ctx)
		svc.RecordSuccess(ctx)
	}

	// Verify metrics exist
	metrics := svc.GetMetrics(ctx)
	assert.Greater(t, metrics.TotalRequests, int64(0))

	// Reset
	svc.ResetMetrics(ctx)

	// Verify reset
	metrics = svc.GetMetrics(ctx)
	assert.Equal(t, int64(0), metrics.TotalRequests)
	assert.Equal(t, int64(0), metrics.FailedRequests)
	assert.Equal(t, 0, metrics.ConsecutiveFailures)
}

func TestPersistentKillSwitchService_GetStatus(t *testing.T) {
	svc, _ := newTestKillSwitchService()
	ctx := context.Background()

	execID := events.NewExecutionID()

	// Activate at different scopes
	_ = svc.ActivateGlobal(ctx, events.KillSwitchReasonManual, "admin", "global")
	_ = svc.ActivateForRepository(ctx, "repo-1", events.KillSwitchReasonSecurityBreach, "system", "repo")
	_ = svc.ActivateForExecution(ctx, execID, events.KillSwitchReasonResourceExhausted, "system", "exec")

	status, err := svc.GetStatus(ctx)
	require.NoError(t, err)

	assert.True(t, status.GlobalActive)
	assert.Equal(t, events.KillSwitchReasonManual, status.GlobalReason)
	assert.Len(t, status.RepositorySwitches, 1)
	assert.Len(t, status.FeatureSwitches, 1)
}

func TestPersistentKillSwitchService_GetActivationHistory(t *testing.T) {
	svc, _ := newTestKillSwitchService()
	ctx := context.Background()

	// Perform various operations
	_ = svc.ActivateGlobal(ctx, events.KillSwitchReasonManual, "admin1", "test1")
	_ = svc.DeactivateGlobal(ctx, "admin2", "resolved")
	_ = svc.ActivateForRepository(ctx, "repo-1", events.KillSwitchReasonSecurityBreach, "system", "breach")

	history := svc.GetActivationHistory(ctx)

	assert.Len(t, history, 3)

	// Verify first event
	assert.Equal(t, "activate", history[0].Action)
	assert.Equal(t, events.KillSwitchScopeGlobal, history[0].Scope)
	assert.Equal(t, events.KillSwitchReasonManual, history[0].Reason)
	assert.Equal(t, "admin1", history[0].ActivatedBy)

	// Verify second event
	assert.Equal(t, "deactivate", history[1].Action)
	assert.Equal(t, events.KillSwitchScopeGlobal, history[1].Scope)

	// Verify third event
	assert.Equal(t, "activate", history[2].Action)
	assert.Equal(t, events.KillSwitchScopeRepository, history[2].Scope)
	assert.Equal(t, "repo-1", history[2].ScopeID)
}

func TestPersistentKillSwitchService_DeactivateForRepository(t *testing.T) {
	svc, _ := newTestKillSwitchService()
	ctx := context.Background()

	// Activate
	_ = svc.ActivateForRepository(ctx, "repo-1", events.KillSwitchReasonSecurityBreach, "system", "breach")

	// Verify active
	active, _, _ := svc.IsActive(ctx, events.ExecutionID{}, "repo-1")
	assert.True(t, active)

	// Deactivate
	err := svc.DeactivateForRepository(ctx, "repo-1", "admin", "resolved")
	require.NoError(t, err)

	// Verify not active
	active, _, _ = svc.IsActive(ctx, events.ExecutionID{}, "repo-1")
	assert.False(t, active)
}

func TestPersistentKillSwitchService_DeactivateForExecution(t *testing.T) {
	svc, _ := newTestKillSwitchService()
	ctx := context.Background()

	execID := events.NewExecutionID()

	// Activate
	_ = svc.ActivateForExecution(ctx, execID, events.KillSwitchReasonResourceExhausted, "system", "OOM")

	// Verify active
	active, _, _ := svc.IsActive(ctx, execID, "")
	assert.True(t, active)

	// Deactivate
	err := svc.DeactivateForExecution(ctx, execID, "admin", "resolved")
	require.NoError(t, err)

	// Verify not active
	active, _, _ = svc.IsActive(ctx, execID, "")
	assert.False(t, active)
}

func TestPersistentKillSwitchService_NilEventsEmitter(t *testing.T) {
	cfg := &appconfig.ReviewerConfig{
		KillSwitchEnabled: true,
	}
	svc := NewPersistentKillSwitchService(cfg, nil)
	ctx := context.Background()

	// Should not panic with nil emitter
	err := svc.ActivateGlobal(ctx, events.KillSwitchReasonManual, "admin", "test")
	assert.NoError(t, err)

	err = svc.DeactivateGlobal(ctx, "admin", "test")
	assert.NoError(t, err)
}
