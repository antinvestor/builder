package events_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/antinvestor/builder/internal/events"
)

func TestDefaultRetryQueueConfigs(t *testing.T) {
	configs := events.DefaultRetryQueueConfigs()

	assert.Len(t, configs, 3)

	// Verify level 1 config
	assert.Equal(t, events.RetryLevel1, configs[0].Level)
	assert.Equal(t, "feature.events.retry.1", configs[0].QueueName)
	assert.Equal(t, "feature.events.retry.2", configs[0].NextLevelQueueName)
	assert.Equal(t, "feature.events.dlq", configs[0].DLQQueueName)

	// Verify level 2 config
	assert.Equal(t, events.RetryLevel2, configs[1].Level)
	assert.Equal(t, "feature.events.retry.2", configs[1].QueueName)
	assert.Equal(t, "feature.events.retry.3", configs[1].NextLevelQueueName)

	// Verify level 3 config (escalates to DLQ)
	assert.Equal(t, events.RetryLevel3, configs[2].Level)
	assert.Equal(t, "feature.events.retry.3", configs[2].QueueName)
	assert.Empty(t, configs[2].NextLevelQueueName, "level 3 should escalate to DLQ")
}

type mockQueuePublisher struct {
	published []publishedMessage
}

type publishedMessage struct {
	queueName string
	event     *events.Event
}

func (m *mockQueuePublisher) Publish(
	_ context.Context,
	queueName string,
	payload any,
	_ map[string]string,
) error {
	// For retry handler, payload should be JSONMap
	if jsonMap, ok := payload.(events.JSONMap); ok {
		eventJSON, _ := json.Marshal(jsonMap)
		var event events.Event
		_ = json.Unmarshal(eventJSON, &event)
		m.published = append(m.published, publishedMessage{
			queueName: queueName,
			event:     &event,
		})
	}
	return nil
}

type mockEventHandler struct {
	handleFunc func(ctx context.Context, event *events.Event) error
	calls      int
}

func (m *mockEventHandler) Handle(ctx context.Context, event *events.Event) error {
	m.calls++
	if m.handleFunc != nil {
		return m.handleFunc(ctx, event)
	}
	return nil
}

func TestRetryQueueHandler_SuccessfulProcessing(t *testing.T) {
	ctx := context.Background()

	publisher := &mockQueuePublisher{}
	queuePublisher := events.NewQueuePublisher(publisher.Publish)
	dedup := events.NewInMemoryDeduplicationStore()

	handler := &mockEventHandler{}
	config := events.RetryQueueConfig{
		Level:               events.RetryLevel1,
		QueueName:           "feature.events.retry.1",
		MaxAttemptsAtLevel:  2,
		NextLevelQueueName:  "feature.events.retry.2",
		DLQQueueName:        "feature.events.dlq",
		DelayBetweenRetries: time.Minute,
	}

	retryHandler := events.NewRetryQueueHandler(config, handler, queuePublisher, dedup)

	// Create a test event
	event := createTestEvent(t)
	eventPayload, err := json.Marshal(event)
	require.NoError(t, err)

	// Handle the event
	err = retryHandler.Handle(ctx, nil, eventPayload)
	require.NoError(t, err)

	// Verify handler was called
	assert.Equal(t, 1, handler.calls)

	// Verify event was marked as processed
	processed, err := dedup.IsProcessed(ctx, event.EventID)
	require.NoError(t, err)
	assert.True(t, processed)
}

func TestRetryQueueHandler_Deduplication(t *testing.T) {
	ctx := context.Background()

	publisher := &mockQueuePublisher{}
	queuePublisher := events.NewQueuePublisher(publisher.Publish)
	dedup := events.NewInMemoryDeduplicationStore()

	handler := &mockEventHandler{}
	config := events.RetryQueueConfig{
		Level:               events.RetryLevel1,
		QueueName:           "feature.events.retry.1",
		MaxAttemptsAtLevel:  2,
		NextLevelQueueName:  "feature.events.retry.2",
		DLQQueueName:        "feature.events.dlq",
		DelayBetweenRetries: time.Minute,
	}

	retryHandler := events.NewRetryQueueHandler(config, handler, queuePublisher, dedup)

	// Create a test event
	event := createTestEvent(t)
	eventPayload, err := json.Marshal(event)
	require.NoError(t, err)

	// Mark event as already processed
	err = dedup.MarkProcessed(ctx, event.EventID, event.FeatureExecutionID)
	require.NoError(t, err)

	// Handle the event
	err = retryHandler.Handle(ctx, nil, eventPayload)
	require.NoError(t, err)

	// Verify handler was NOT called (due to deduplication)
	assert.Equal(t, 0, handler.calls)
}

func TestRetryQueueManager(t *testing.T) {
	publisher := &mockQueuePublisher{}
	queuePublisher := events.NewQueuePublisher(publisher.Publish)
	dedup := events.NewInMemoryDeduplicationStore()

	handler := &mockEventHandler{}

	manager := events.NewRetryQueueManager(handler, queuePublisher, dedup)

	// Verify all handlers are created
	handlers := manager.GetHandlers()
	assert.Len(t, handlers, 3)
	assert.NotNil(t, handlers[events.RetryLevel1])
	assert.NotNil(t, handlers[events.RetryLevel2])
	assert.NotNil(t, handlers[events.RetryLevel3])

	// Verify queue subscriptions
	subs := manager.QueueSubscriptions()
	assert.Len(t, subs, 3)
	assert.NotNil(t, subs["feature.events.retry.1"])
	assert.NotNil(t, subs["feature.events.retry.2"])
	assert.NotNil(t, subs["feature.events.retry.3"])
}

func createTestEvent(t *testing.T) *events.Event {
	t.Helper()

	execID := events.NewExecutionID()
	eventID := events.NewEventID()

	return &events.Event{
		EventID:            eventID,
		FeatureExecutionID: execID,
		EventType:          events.FeatureExecutionInitialized,
		SchemaVersion:      "1.0.0",
		SequenceNumber:     1,
		CorrelationID:      eventID,
		CreatedAt:          time.Now().UTC(),
		HLCTimestamp:       events.NewHybridTimestamp(),
		Payload:            []byte(`{"test": "data"}`),
		PayloadChecksum:    "abc123",
		Metadata:           events.EventMetadata{},
		Hints:              events.DefaultProcessingHints(),
	}
}
