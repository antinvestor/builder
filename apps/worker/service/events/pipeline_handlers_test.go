//nolint:testpackage // white-box testing requires internal package access
package events

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	appconfig "github.com/antinvestor/builder/apps/worker/config"
	"github.com/antinvestor/builder/internal/events"
)

// =============================================================================
// Mock Implementations
// =============================================================================

type mockQueueManager struct {
	publishedMessages []publishedMessage
	publishError      error
}

type publishedMessage struct {
	queueName string
	payload   any
}

func (m *mockQueueManager) Publish(_ context.Context, queueName string, payload any, _ ...map[string]string) error {
	if m.publishError != nil {
		return m.publishError
	}
	m.publishedMessages = append(m.publishedMessages, publishedMessage{
		queueName: queueName,
		payload:   payload,
	})
	return nil
}

type mockEmitter struct {
	emittedEvents []emittedEvent
	emitError     error
}

type emittedEvent struct {
	name    string
	payload any
}

func (m *mockEmitter) Emit(_ context.Context, name string, payload any) error {
	if m.emitError != nil {
		return m.emitError
	}
	m.emittedEvents = append(m.emittedEvents, emittedEvent{
		name:    name,
		payload: payload,
	})
	return nil
}

type mockBAMLClient struct {
	generatePatchResponse *GeneratePatchResponse
	generatePatchError    error
}

func (m *mockBAMLClient) GeneratePatch(_ context.Context, _ *GeneratePatchRequest) (*GeneratePatchResponse, error) {
	if m.generatePatchError != nil {
		return nil, m.generatePatchError
	}
	if m.generatePatchResponse != nil {
		return m.generatePatchResponse, nil
	}
	return &GeneratePatchResponse{
		TokensUsed: 100,
	}, nil
}

// =============================================================================
// TestExecutionRequestEvent Tests
// =============================================================================

func TestTestExecutionRequestEvent_Name(t *testing.T) {
	handler := NewTestExecutionRequestEvent(nil, nil, nil)
	assert.Equal(t, string(events.PatchGenerationCompleted), handler.Name())
}

func TestTestExecutionRequestEvent_PayloadType(t *testing.T) {
	handler := NewTestExecutionRequestEvent(nil, nil, nil)
	assert.IsType(t, &events.PatchGenerationCompletedPayload{}, handler.PayloadType())
}

func TestTestExecutionRequestEvent_Execute_Success(t *testing.T) {
	cfg := &appconfig.WorkerConfig{
		QueueExecutionRequestName: "test-execution-queue",
	}
	queueMan := &mockQueueManager{}
	eventsMan := &mockEmitter{}

	handler := NewTestExecutionRequestEvent(cfg, queueMan, eventsMan)

	executionID := events.NewExecutionID()
	payload := &events.PatchGenerationCompletedPayload{
		ExecutionID:    executionID,
		TotalSteps:     3,
		StepsCompleted: 3,
		FinalCommitSHA: "abc123",
		CompletedAt:    time.Now(),
	}

	err := handler.Execute(context.Background(), payload)

	require.NoError(t, err)
	require.Len(t, eventsMan.emittedEvents, 1)
	assert.Equal(t, string(events.TestExecutionStarted), eventsMan.emittedEvents[0].name)

	require.Len(t, queueMan.publishedMessages, 1)
	assert.Equal(t, "test-execution-queue", queueMan.publishedMessages[0].queueName)

	testReq, ok := queueMan.publishedMessages[0].payload.(*events.TestExecutionRequestedPayload)
	require.True(t, ok)
	assert.Equal(t, executionID, testReq.ExecutionID)
}

func TestTestExecutionRequestEvent_Execute_InvalidPayload(t *testing.T) {
	handler := NewTestExecutionRequestEvent(nil, nil, nil)
	err := handler.Execute(context.Background(), "invalid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid payload type")
}

func TestTestExecutionRequestEvent_Execute_EventEmitError(t *testing.T) {
	cfg := &appconfig.WorkerConfig{}
	eventsMan := &mockEmitter{emitError: errors.New("emit failed")}

	handler := NewTestExecutionRequestEvent(cfg, nil, eventsMan)

	payload := &events.PatchGenerationCompletedPayload{
		ExecutionID: events.NewExecutionID(),
	}

	err := handler.Execute(context.Background(), payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "emit failed")
}

// =============================================================================
// ReviewRequestEvent Tests
// =============================================================================

func TestReviewRequestEvent_Name(t *testing.T) {
	handler := NewReviewRequestEvent(nil, nil, nil)
	assert.Equal(t, string(events.TestExecutionCompleted), handler.Name())
}

func TestReviewRequestEvent_Execute_TestsPassed(t *testing.T) {
	cfg := &appconfig.WorkerConfig{
		QueueReviewRequestName: "review-request-queue",
	}
	queueMan := &mockQueueManager{}
	eventsMan := &mockEmitter{}

	handler := NewReviewRequestEvent(cfg, queueMan, eventsMan)

	executionID := events.NewExecutionID()
	payload := &events.TestExecutionCompletedPayload{
		ExecutionID: executionID,
		Success:     true,
		Result: &events.TestResult{
			TotalTests:  10,
			PassedTests: 10,
			Success:     true,
		},
	}

	err := handler.Execute(context.Background(), payload)

	require.NoError(t, err)
	require.Len(t, eventsMan.emittedEvents, 1)
	assert.Equal(t, string(events.ReviewStarted), eventsMan.emittedEvents[0].name)

	require.Len(t, queueMan.publishedMessages, 1)
	assert.Equal(t, "review-request-queue", queueMan.publishedMessages[0].queueName)
}

func TestReviewRequestEvent_Execute_TestsFailed(t *testing.T) {
	cfg := &appconfig.WorkerConfig{
		ReviewThresholds: events.ReviewThresholds{
			MaxIterations: 5,
		},
	}
	queueMan := &mockQueueManager{}
	eventsMan := &mockEmitter{}

	handler := NewReviewRequestEvent(cfg, queueMan, eventsMan)

	executionID := events.NewExecutionID()
	payload := &events.TestExecutionCompletedPayload{
		ExecutionID: executionID,
		Success:     false,
		Result: &events.TestResult{
			TotalTests:  10,
			PassedTests: 5,
			FailedTests: 5,
			Success:     false,
		},
	}

	err := handler.Execute(context.Background(), payload)

	require.NoError(t, err)
	require.Len(t, eventsMan.emittedEvents, 1)
	assert.Equal(t, string(events.IterationRequired), eventsMan.emittedEvents[0].name)

	// Should emit iteration required, not publish to review queue
	assert.Empty(t, queueMan.publishedMessages)

	iterPayload, ok := eventsMan.emittedEvents[0].payload.(*events.IterationRequiredPayload)
	require.True(t, ok)
	assert.Equal(t, events.IterationReasonTestsFailed, iterPayload.Reason)
	assert.Equal(t, 4, iterPayload.MaxIterationsRemaining) // 5 - 1
}

// =============================================================================
// ReviewResultEvent Tests
// =============================================================================

func TestReviewResultEvent_Name(t *testing.T) {
	handler := NewReviewResultEvent(nil, nil, nil, nil, nil)
	assert.Equal(t, string(events.ReviewCompleted), handler.Name())
}

func TestReviewResultEvent_Execute_ManualReview(t *testing.T) {
	cfg := &appconfig.WorkerConfig{}
	eventsMan := &mockEmitter{}

	// Create a minimal handler - we test ManualReview decision which doesn't require repoService
	handler := &ReviewResultEvent{
		cfg:       cfg,
		eventsMan: eventsMan,
	}

	executionID := events.NewExecutionID()
	payload := &events.ComprehensiveReviewCompletedPayload{
		ExecutionID:       executionID,
		Decision:          events.ControlDecisionManualReview,
		DecisionRationale: "Manual review required",
	}

	err := handler.Execute(context.Background(), payload)
	require.NoError(t, err)
	// ManualReview decision doesn't emit any events, it just pauses
	assert.Empty(t, eventsMan.emittedEvents)
}

func TestReviewResultEvent_Execute_Iterate(t *testing.T) {
	cfg := &appconfig.WorkerConfig{}
	eventsMan := &mockEmitter{}

	handler := &ReviewResultEvent{
		cfg:       cfg,
		eventsMan: eventsMan,
	}

	executionID := events.NewExecutionID()
	payload := &events.ComprehensiveReviewCompletedPayload{
		ExecutionID:       executionID,
		ReviewID:          "review-123",
		Decision:          events.ControlDecisionIterate,
		DecisionRationale: "Issues found",
		BlockingIssues: []events.ReviewIssue{
			{
				ID:          "issue-1",
				Type:        events.ReviewIssueTypeSecurity,
				Severity:    events.ReviewIssueSeverityHigh,
				Title:       "SQL Injection vulnerability",
				Description: "User input not sanitized",
			},
		},
	}

	err := handler.Execute(context.Background(), payload)

	require.NoError(t, err)
	require.Len(t, eventsMan.emittedEvents, 1)
	assert.Equal(t, string(events.IterationRequired), eventsMan.emittedEvents[0].name)

	iterPayload, ok := eventsMan.emittedEvents[0].payload.(*events.FeatureIterationRequestedPayload)
	require.True(t, ok)
	assert.Equal(t, executionID, iterPayload.ExecutionID)
	assert.Equal(t, "review-123", iterPayload.ReviewID)
	assert.Len(t, iterPayload.Issues, 1)
}

func TestReviewResultEvent_Execute_Abort(t *testing.T) {
	cfg := &appconfig.WorkerConfig{}
	eventsMan := &mockEmitter{}

	handler := &ReviewResultEvent{
		cfg:       cfg,
		eventsMan: eventsMan,
	}

	executionID := events.NewExecutionID()
	payload := &events.ComprehensiveReviewCompletedPayload{
		ExecutionID:       executionID,
		Decision:          events.ControlDecisionAbort,
		DecisionRationale: "Critical security issue",
	}

	err := handler.Execute(context.Background(), payload)

	require.NoError(t, err)
	require.Len(t, eventsMan.emittedEvents, 1)
	assert.Equal(t, string(events.FeatureExecutionFailed), eventsMan.emittedEvents[0].name)

	failPayload, ok := eventsMan.emittedEvents[0].payload.(*events.FeatureExecutionFailedPayload)
	require.True(t, ok)
	assert.Equal(t, "review_abort", failPayload.ErrorCode)
	assert.Equal(t, "Critical security issue", failPayload.ErrorMessage)
}

func TestReviewResultEvent_Execute_UnknownDecision(t *testing.T) {
	cfg := &appconfig.WorkerConfig{}
	eventsMan := &mockEmitter{}

	handler := &ReviewResultEvent{
		cfg:       cfg,
		eventsMan: eventsMan,
	}

	payload := &events.ComprehensiveReviewCompletedPayload{
		ExecutionID: events.NewExecutionID(),
		Decision:    events.ControlDecision("unknown"),
	}

	err := handler.Execute(context.Background(), payload)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown decision")
}

// =============================================================================
// IterationEvent Tests
// =============================================================================

func TestIterationEvent_Name(t *testing.T) {
	handler := NewIterationEvent(nil, nil, nil)
	assert.Equal(t, string(events.IterationRequired), handler.Name())
}

func TestIterationEvent_Execute_Success(t *testing.T) {
	cfg := &appconfig.WorkerConfig{
		ReviewThresholds: events.ReviewThresholds{
			MaxIterations: 5,
		},
	}
	bamlClient := &mockBAMLClient{
		generatePatchResponse: &GeneratePatchResponse{
			TokensUsed: 150,
		},
	}
	eventsMan := &mockEmitter{}

	handler := NewIterationEvent(cfg, bamlClient, eventsMan)

	executionID := events.NewExecutionID()
	payload := &events.FeatureIterationRequestedPayload{
		ExecutionID:     executionID,
		ReviewID:        "review-123",
		IterationNumber: 1,
		Issues: []events.ReviewIssue{
			{
				ID:          "issue-1",
				Type:        events.ReviewIssueTypeBug,
				Severity:    events.ReviewIssueSeverityMedium,
				Title:       "Bug fix needed",
				Description: "Fix the bug",
			},
		},
	}

	err := handler.Execute(context.Background(), payload)

	require.NoError(t, err)
	require.Len(t, eventsMan.emittedEvents, 2)
	assert.Equal(t, string(events.IterationStarted), eventsMan.emittedEvents[0].name)
	assert.Equal(t, string(events.PatchGenerationCompleted), eventsMan.emittedEvents[1].name)

	// Verify iteration started payload
	iterStarted, ok := eventsMan.emittedEvents[0].payload.(*events.IterationStartedPayload)
	require.True(t, ok)
	assert.Equal(t, 1, iterStarted.IterationNumber)
	assert.Equal(t, events.IterationReasonReviewRejected, iterStarted.Reason)

	// Verify patch generation completed payload
	patchCompleted, ok := eventsMan.emittedEvents[1].payload.(*events.PatchGenerationCompletedPayload)
	require.True(t, ok)
	assert.Equal(t, executionID, patchCompleted.ExecutionID)
	assert.Equal(t, 150, patchCompleted.TotalLLMTokens)
}

func TestIterationEvent_Execute_MaxIterationsReached(t *testing.T) {
	cfg := &appconfig.WorkerConfig{
		ReviewThresholds: events.ReviewThresholds{
			MaxIterations: 3,
		},
	}
	eventsMan := &mockEmitter{}

	handler := NewIterationEvent(cfg, nil, eventsMan)

	payload := &events.FeatureIterationRequestedPayload{
		ExecutionID:     events.NewExecutionID(),
		IterationNumber: 3, // Equal to max
	}

	err := handler.Execute(context.Background(), payload)

	require.NoError(t, err)
	require.Len(t, eventsMan.emittedEvents, 1)
	assert.Equal(t, string(events.FeatureExecutionFailed), eventsMan.emittedEvents[0].name)

	failPayload, ok := eventsMan.emittedEvents[0].payload.(*events.FeatureExecutionFailedPayload)
	require.True(t, ok)
	assert.Equal(t, "max_iterations_exceeded", failPayload.ErrorCode)
}

func TestIterationEvent_Execute_DefaultMaxIterations(t *testing.T) {
	cfg := &appconfig.WorkerConfig{
		ReviewThresholds: events.ReviewThresholds{
			MaxIterations: 0, // Not set, should default to 3
		},
	}
	eventsMan := &mockEmitter{}

	handler := NewIterationEvent(cfg, nil, eventsMan)

	payload := &events.FeatureIterationRequestedPayload{
		ExecutionID:     events.NewExecutionID(),
		IterationNumber: 3, // Equal to default max of 3
	}

	err := handler.Execute(context.Background(), payload)

	require.NoError(t, err)
	require.Len(t, eventsMan.emittedEvents, 1)
	assert.Equal(t, string(events.FeatureExecutionFailed), eventsMan.emittedEvents[0].name)
}

func TestIterationEvent_Execute_BAMLError(t *testing.T) {
	cfg := &appconfig.WorkerConfig{
		ReviewThresholds: events.ReviewThresholds{
			MaxIterations: 5,
		},
	}
	bamlClient := &mockBAMLClient{
		generatePatchError: errors.New("BAML generation failed"),
	}
	eventsMan := &mockEmitter{}

	handler := NewIterationEvent(cfg, bamlClient, eventsMan)

	payload := &events.FeatureIterationRequestedPayload{
		ExecutionID:     events.NewExecutionID(),
		IterationNumber: 1,
	}

	err := handler.Execute(context.Background(), payload)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "BAML generation failed")
}

// =============================================================================
// DeliveryEvent Tests
// =============================================================================

func TestDeliveryEvent_Name(t *testing.T) {
	handler := NewDeliveryEvent(nil, nil, nil, nil)
	assert.Equal(t, string(events.GitPushCompleted), handler.Name())
}

func TestDeliveryEvent_PayloadType(t *testing.T) {
	handler := NewDeliveryEvent(nil, nil, nil, nil)
	assert.IsType(t, &events.GitPushCompletedPayload{}, handler.PayloadType())
}

func TestDeliveryEvent_Execute_Success(t *testing.T) {
	cfg := &appconfig.WorkerConfig{}
	eventsMan := &mockEmitter{}

	handler := NewDeliveryEvent(cfg, nil, nil, eventsMan)

	payload := &events.GitPushCompletedPayload{
		BranchName:      "feature/test-123",
		RemoteRef:       "refs/heads/feature/test-123",
		RemoteCommitSHA: "abc123def456",
		CommitsPushed:   3,
		DurationMS:      1500,
		CompletedAt:     time.Now(),
	}

	err := handler.Execute(context.Background(), payload)

	require.NoError(t, err)
	require.Len(t, eventsMan.emittedEvents, 1)
	assert.Equal(t, string(events.FeatureExecutionCompleted), eventsMan.emittedEvents[0].name)

	completePayload, ok := eventsMan.emittedEvents[0].payload.(*events.FeatureExecutionCompletedPayload)
	require.True(t, ok)
	assert.Equal(t, "feature/test-123", completePayload.BranchName)
	assert.Equal(t, "abc123def456", completePayload.FinalCommit.SHA)
}

func TestDeliveryEvent_Execute_InvalidPayload(t *testing.T) {
	handler := NewDeliveryEvent(nil, nil, nil, nil)
	err := handler.Execute(context.Background(), "invalid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid payload type")
}

// =============================================================================
// Helper Function Tests
// =============================================================================

func TestExtractIssueTitles(t *testing.T) {
	issues := []events.ReviewIssue{
		{Severity: events.ReviewIssueSeverityHigh, Title: "Security issue"},
		{Severity: events.ReviewIssueSeverityMedium, Title: "Code smell"},
	}

	titles := extractIssueTitles(issues)

	require.Len(t, titles, 2)
	assert.Equal(t, "[high] Security issue", titles[0])
	assert.Equal(t, "[medium] Code smell", titles[1])
}

func TestExtractIssueTitles_Empty(t *testing.T) {
	titles := extractIssueTitles(nil)
	assert.Empty(t, titles)
}

func TestBuildFeedbackFromReviewIssues(t *testing.T) {
	issues := []events.ReviewIssue{
		{
			Severity:    events.ReviewIssueSeverityHigh,
			Title:       "SQL Injection",
			Description: "User input not sanitized",
			Suggestion:  "Use parameterized queries",
		},
		{
			Severity: events.ReviewIssueSeverityLow,
			Title:    "Missing comment",
		},
	}

	feedback := buildFeedbackFromReviewIssues(issues)

	assert.Contains(t, feedback, "Please fix the following issues")
	assert.Contains(t, feedback, "[high] SQL Injection")
	assert.Contains(t, feedback, "User input not sanitized")
	assert.Contains(t, feedback, "Suggestion: Use parameterized queries")
	assert.Contains(t, feedback, "[low] Missing comment")
}

func TestBuildFeedbackFromReviewIssues_Empty(t *testing.T) {
	feedback := buildFeedbackFromReviewIssues(nil)
	assert.Empty(t, feedback)
}

func TestConvertToIterationIssues(t *testing.T) {
	issues := []events.ReviewIssue{
		{
			Type:        events.ReviewIssueTypeSecurity,
			FilePath:    "main.go",
			LineStart:   42,
			Description: "SQL injection vulnerability",
			Severity:    events.ReviewIssueSeverityHigh,
		},
		{
			Type:        events.ReviewIssueTypeBug,
			FilePath:    "utils.go",
			LineStart:   100,
			Description: "Null pointer dereference",
			Severity:    events.ReviewIssueSeverityCritical,
		},
	}

	result := convertToIterationIssues(issues)

	require.Len(t, result, 2)
	assert.Equal(t, "security", result[0].Type)
	assert.Equal(t, "main.go", result[0].FilePath)
	assert.Equal(t, 42, result[0].LineNumber)
	assert.Equal(t, "SQL injection vulnerability", result[0].Description)
	assert.Equal(t, "high", result[0].Severity)

	assert.Equal(t, "bug", result[1].Type)
	assert.Equal(t, "utils.go", result[1].FilePath)
	assert.Equal(t, 100, result[1].LineNumber)
	assert.Equal(t, "critical", result[1].Severity)
}

func TestConvertToIterationIssues_Empty(t *testing.T) {
	result := convertToIterationIssues(nil)
	assert.Empty(t, result)
}
