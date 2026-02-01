package events_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/antinvestor/builder/internal/events"
)

func createTestDLQEntry(t *testing.T) *events.DLQEntry {
	t.Helper()

	event := createTestEvent(t)

	return &events.DLQEntry{
		Event: event,
		RetryMetadata: events.RetryMetadata{
			OriginalEventID:  event.EventID,
			CurrentAttempt:   3,
			MaxAttempts:      6,
			LastAttemptAt:    time.Now(),
			LastErrorCode:    "timeout",
			LastErrorMessage: "connection timed out",
		},
		FailureReason:         "connection timed out",
		FailureClassification: events.DLQFailureTransient,
		EnteredDLQAt:          time.Now(),
		ExpiresAt:             time.Now().AddDate(0, 0, 28),
		ManualReviewRequired:  false,
	}
}

func TestInMemoryDLQStore_AddAndGet(t *testing.T) {
	ctx := context.Background()
	publisher := &mockQueuePublisher{}
	queuePublisher := events.NewQueuePublisher(publisher.Publish)

	store := events.NewInMemoryDLQStore(queuePublisher, "feature.events")

	entry := createTestDLQEntry(t)

	// Add entry
	id, err := store.AddEntry(ctx, entry)
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	// Get entry
	retrieved, err := store.GetDLQEntry(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, retrieved)
	assert.Equal(t, entry.FailureReason, retrieved.FailureReason)
	assert.Equal(t, entry.FailureClassification, retrieved.FailureClassification)
}

func TestInMemoryDLQStore_GetNonExistent(t *testing.T) {
	ctx := context.Background()
	publisher := &mockQueuePublisher{}
	queuePublisher := events.NewQueuePublisher(publisher.Publish)

	store := events.NewInMemoryDLQStore(queuePublisher, "feature.events")

	// Try to get non-existent entry
	_, err := store.GetDLQEntry(ctx, "non-existent")
	assert.ErrorIs(t, err, events.ErrDLQEntryNotFound)
}

func TestInMemoryDLQStore_ListEntries(t *testing.T) {
	ctx := context.Background()
	publisher := &mockQueuePublisher{}
	queuePublisher := events.NewQueuePublisher(publisher.Publish)

	store := events.NewInMemoryDLQStore(queuePublisher, "feature.events")

	// Add multiple entries
	for range 5 {
		entry := createTestDLQEntry(t)
		_, err := store.AddEntry(ctx, entry)
		require.NoError(t, err)
	}

	// List all entries
	result, err := store.ListDLQEntries(ctx, events.DLQFilter{})
	require.NoError(t, err)
	assert.Equal(t, 5, result.Total)
	assert.Len(t, result.Entries, 5)
	assert.False(t, result.HasMore)
}

func TestInMemoryDLQStore_ListWithPagination(t *testing.T) {
	ctx := context.Background()
	publisher := &mockQueuePublisher{}
	queuePublisher := events.NewQueuePublisher(publisher.Publish)

	store := events.NewInMemoryDLQStore(queuePublisher, "feature.events")

	// Add 10 entries
	for range 10 {
		entry := createTestDLQEntry(t)
		_, err := store.AddEntry(ctx, entry)
		require.NoError(t, err)
	}

	// List with limit
	result, err := store.ListDLQEntries(ctx, events.DLQFilter{
		Limit: 3,
	})
	require.NoError(t, err)
	assert.Equal(t, 10, result.Total)
	assert.Len(t, result.Entries, 3)
	assert.True(t, result.HasMore)

	// List with offset
	result, err = store.ListDLQEntries(ctx, events.DLQFilter{
		Limit:  3,
		Offset: 3,
	})
	require.NoError(t, err)
	assert.Equal(t, 10, result.Total)
	assert.Len(t, result.Entries, 3)
	assert.True(t, result.HasMore)
}

func TestInMemoryDLQStore_ListByFailureClass(t *testing.T) {
	ctx := context.Background()
	publisher := &mockQueuePublisher{}
	queuePublisher := events.NewQueuePublisher(publisher.Publish)

	store := events.NewInMemoryDLQStore(queuePublisher, "feature.events")

	// Add entries with different failure classes
	transientEntry := createTestDLQEntry(t)
	transientEntry.FailureClassification = events.DLQFailureTransient
	_, err := store.AddEntry(ctx, transientEntry)
	require.NoError(t, err)

	permanentEntry := createTestDLQEntry(t)
	permanentEntry.FailureClassification = events.DLQFailurePermanent
	_, err = store.AddEntry(ctx, permanentEntry)
	require.NoError(t, err)

	// Filter by transient
	result, err := store.ListDLQEntries(ctx, events.DLQFilter{
		FailureClass: events.DLQFailureTransient,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total)
}

func TestInMemoryDLQStore_RequeueEntry(t *testing.T) {
	ctx := context.Background()
	publisher := &mockQueuePublisher{}
	queuePublisher := events.NewQueuePublisher(publisher.Publish)

	store := events.NewInMemoryDLQStore(queuePublisher, "feature.events")

	entry := createTestDLQEntry(t)
	id, err := store.AddEntry(ctx, entry)
	require.NoError(t, err)

	// Requeue the entry
	err = store.RequeueEntry(ctx, id, events.RequeueRequest{
		ResolvedBy: "test-user",
		Notes:      "manual retry",
	})
	require.NoError(t, err)

	// Verify entry was resolved
	retrieved, err := store.GetDLQEntry(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, retrieved.Resolution)
	assert.Equal(t, events.DLQResolutionRequeued, retrieved.Resolution.Status)
	assert.Equal(t, "test-user", retrieved.Resolution.ResolvedBy)

	// Verify event was published
	assert.Len(t, publisher.published, 1)
	assert.Equal(t, "feature.events", publisher.published[0].queueName)
}

func TestInMemoryDLQStore_RequeueAlreadyResolved(t *testing.T) {
	ctx := context.Background()
	publisher := &mockQueuePublisher{}
	queuePublisher := events.NewQueuePublisher(publisher.Publish)

	store := events.NewInMemoryDLQStore(queuePublisher, "feature.events")

	entry := createTestDLQEntry(t)
	id, err := store.AddEntry(ctx, entry)
	require.NoError(t, err)

	// Requeue first time
	err = store.RequeueEntry(ctx, id, events.RequeueRequest{
		ResolvedBy: "test-user",
	})
	require.NoError(t, err)

	// Try to requeue again
	err = store.RequeueEntry(ctx, id, events.RequeueRequest{
		ResolvedBy: "test-user",
	})
	assert.ErrorIs(t, err, events.ErrDLQEntryResolved)
}

func TestInMemoryDLQStore_DiscardEntry(t *testing.T) {
	ctx := context.Background()
	publisher := &mockQueuePublisher{}
	queuePublisher := events.NewQueuePublisher(publisher.Publish)

	store := events.NewInMemoryDLQStore(queuePublisher, "feature.events")

	entry := createTestDLQEntry(t)
	id, err := store.AddEntry(ctx, entry)
	require.NoError(t, err)

	// Discard the entry
	err = store.DiscardEntry(ctx, id, events.DiscardRequest{
		ResolvedBy: "test-user",
		Notes:      "duplicate event",
	})
	require.NoError(t, err)

	// Verify entry was discarded
	retrieved, err := store.GetDLQEntry(ctx, id)
	require.NoError(t, err)
	require.NotNil(t, retrieved.Resolution)
	assert.Equal(t, events.DLQResolutionDiscarded, retrieved.Resolution.Status)
	assert.Equal(t, "test-user", retrieved.Resolution.ResolvedBy)
	assert.Equal(t, "duplicate event", retrieved.Resolution.Notes)
}

func TestInMemoryDLQStore_DiscardRequiresNotes(t *testing.T) {
	ctx := context.Background()
	publisher := &mockQueuePublisher{}
	queuePublisher := events.NewQueuePublisher(publisher.Publish)

	store := events.NewInMemoryDLQStore(queuePublisher, "feature.events")

	entry := createTestDLQEntry(t)
	id, err := store.AddEntry(ctx, entry)
	require.NoError(t, err)

	// Try to discard without notes
	err = store.DiscardEntry(ctx, id, events.DiscardRequest{
		ResolvedBy: "test-user",
		Notes:      "", // Empty notes
	})
	assert.Error(t, err)
}

func TestInMemoryDLQStore_GetStats(t *testing.T) {
	ctx := context.Background()
	publisher := &mockQueuePublisher{}
	queuePublisher := events.NewQueuePublisher(publisher.Publish)

	store := events.NewInMemoryDLQStore(queuePublisher, "feature.events")

	// Add entries with different classifications
	transientEntry := createTestDLQEntry(t)
	transientEntry.FailureClassification = events.DLQFailureTransient
	transientEntry.ManualReviewRequired = false
	_, err := store.AddEntry(ctx, transientEntry)
	require.NoError(t, err)

	permanentEntry := createTestDLQEntry(t)
	permanentEntry.FailureClassification = events.DLQFailurePermanent
	permanentEntry.ManualReviewRequired = true
	id, err := store.AddEntry(ctx, permanentEntry)
	require.NoError(t, err)

	// Resolve one entry
	err = store.DiscardEntry(ctx, id, events.DiscardRequest{
		ResolvedBy: "test-user",
		Notes:      "test",
	})
	require.NoError(t, err)

	// Get stats
	stats, err := store.GetDLQStats(ctx)
	require.NoError(t, err)

	assert.Equal(t, 2, stats.TotalEntries)
	assert.Equal(t, 1, stats.PendingEntries)
	assert.Equal(t, 1, stats.ResolvedEntries)
	assert.Equal(t, 1, stats.ByFailureClass[events.DLQFailureTransient])
	assert.Equal(t, 1, stats.ByFailureClass[events.DLQFailurePermanent])
}

func TestInMemoryDLQStore_CleanupExpired(t *testing.T) {
	ctx := context.Background()
	publisher := &mockQueuePublisher{}
	queuePublisher := events.NewQueuePublisher(publisher.Publish)

	store := events.NewInMemoryDLQStore(queuePublisher, "feature.events")

	// Add an expired entry
	expiredEntry := createTestDLQEntry(t)
	expiredEntry.ExpiresAt = time.Now().Add(-time.Hour) // Already expired
	_, err := store.AddEntry(ctx, expiredEntry)
	require.NoError(t, err)

	// Add a valid entry
	validEntry := createTestDLQEntry(t)
	validEntry.ExpiresAt = time.Now().Add(time.Hour) // Not expired
	_, err = store.AddEntry(ctx, validEntry)
	require.NoError(t, err)

	// Cleanup
	removed, err := store.CleanupExpired(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, removed)

	// Verify only one entry remains
	result, err := store.ListDLQEntries(ctx, events.DLQFilter{})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total)
}

func TestInMemoryDLQStore_ListIncludeResolved(t *testing.T) {
	ctx := context.Background()
	publisher := &mockQueuePublisher{}
	queuePublisher := events.NewQueuePublisher(publisher.Publish)

	store := events.NewInMemoryDLQStore(queuePublisher, "feature.events")

	// Add entry and resolve it
	entry := createTestDLQEntry(t)
	id, err := store.AddEntry(ctx, entry)
	require.NoError(t, err)

	err = store.DiscardEntry(ctx, id, events.DiscardRequest{
		ResolvedBy: "test-user",
		Notes:      "test",
	})
	require.NoError(t, err)

	// List without include resolved
	result, err := store.ListDLQEntries(ctx, events.DLQFilter{
		IncludeResolved: false,
	})
	require.NoError(t, err)
	assert.Equal(t, 0, result.Total)

	// List with include resolved
	result, err = store.ListDLQEntries(ctx, events.DLQFilter{
		IncludeResolved: true,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total)
}
