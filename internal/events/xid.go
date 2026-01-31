// Package events provides event-driven workflow primitives for feature execution.
package events

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/rs/xid"
)

// XID provides globally unique, sortable identifiers.
// Format: 20 characters, base32-hex encoded, 12 bytes
// Structure:
//   - 4 bytes: timestamp (seconds since Unix epoch)
//   - 3 bytes: machine identifier
//   - 2 bytes: process identifier
//   - 3 bytes: random counter
//
// Properties:
//   - Sortable by creation time
//   - No coordination required
//   - URL-safe (base32-hex)
//   - Smaller than UUID (20 chars vs 36)

// ExecutionID represents a feature execution identifier.
type ExecutionID struct {
	id xid.ID
}

// NewExecutionID generates a new execution ID.
func NewExecutionID() ExecutionID {
	return ExecutionID{id: xid.New()}
}

// ParseExecutionID parses an execution ID from string.
func ParseExecutionID(s string) (ExecutionID, error) {
	id, err := xid.FromString(s)
	if err != nil {
		return ExecutionID{}, fmt.Errorf("invalid execution ID %q: %w", s, err)
	}
	return ExecutionID{id: id}, nil
}

// MustParseExecutionID parses an execution ID, panicking on error.
func MustParseExecutionID(s string) ExecutionID {
	id, err := ParseExecutionID(s)
	if err != nil {
		panic(err)
	}
	return id
}

// String returns the string representation.
func (e ExecutionID) String() string {
	return e.id.String()
}

// Short returns the first 8 characters for human-readable contexts.
func (e ExecutionID) Short() string {
	s := e.id.String()
	if len(s) >= 8 {
		return s[:8]
	}
	return s
}

// Time returns the timestamp embedded in the ID.
func (e ExecutionID) Time() time.Time {
	return e.id.Time()
}

// IsZero returns true if this is the zero value.
func (e ExecutionID) IsZero() bool {
	return e.id.IsNil()
}

// MarshalJSON implements json.Marshaler.
func (e ExecutionID) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.id.String())
}

// UnmarshalJSON implements json.Unmarshaler.
func (e *ExecutionID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	id, err := xid.FromString(s)
	if err != nil {
		return err
	}
	e.id = id
	return nil
}

// Bytes returns the raw bytes of the ID.
func (e ExecutionID) Bytes() []byte {
	return e.id.Bytes()
}

// Compare returns -1, 0, or 1 comparing two IDs.
func (e ExecutionID) Compare(other ExecutionID) int {
	return e.id.Compare(other.id)
}

// EventID represents an event identifier.
type EventID struct {
	id xid.ID
}

// NewEventID generates a new event ID.
func NewEventID() EventID {
	return EventID{id: xid.New()}
}

// ParseEventID parses an event ID from string.
func ParseEventID(s string) (EventID, error) {
	id, err := xid.FromString(s)
	if err != nil {
		return EventID{}, fmt.Errorf("invalid event ID %q: %w", s, err)
	}
	return EventID{id: id}, nil
}

// String returns the string representation.
func (e EventID) String() string {
	return e.id.String()
}

// Time returns the timestamp embedded in the ID.
func (e EventID) Time() time.Time {
	return e.id.Time()
}

// IsZero returns true if this is the zero value.
func (e EventID) IsZero() bool {
	return e.id.IsNil()
}

// MarshalJSON implements json.Marshaler.
func (e EventID) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.id.String())
}

// UnmarshalJSON implements json.Unmarshaler.
func (e *EventID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	id, err := xid.FromString(s)
	if err != nil {
		return err
	}
	e.id = id
	return nil
}

// StepID represents a step identifier within an execution.
type StepID struct {
	ExecutionID   ExecutionID `json:"execution_id"`
	StepIndex     int         `json:"step_index"`
	AttemptNumber int         `json:"attempt_number"`
}

// NewStepID creates a new step identifier.
func NewStepID(execID ExecutionID, stepIndex, attempt int) StepID {
	return StepID{
		ExecutionID:   execID,
		StepIndex:     stepIndex,
		AttemptNumber: attempt,
	}
}

// String returns a string representation.
func (s StepID) String() string {
	return fmt.Sprintf("%s/step/%d/attempt/%d", s.ExecutionID.String(), s.StepIndex, s.AttemptNumber)
}

// IDGenerator provides thread-safe ID generation with machine/process binding.
type IDGenerator struct {
	mu      sync.Mutex
	counter uint32
}

// NewIDGenerator creates a new ID generator.
func NewIDGenerator() *IDGenerator {
	return &IDGenerator{}
}

// NewExecutionID generates a new execution ID.
func (g *IDGenerator) NewExecutionID() ExecutionID {
	return NewExecutionID()
}

// NewEventID generates a new event ID.
func (g *IDGenerator) NewEventID() EventID {
	return NewEventID()
}

// DerivedIdentifiers provides derived identifiers from an execution ID.
type DerivedIdentifiers struct {
	ExecutionID ExecutionID
	TenantID    string
	RepoID      string
}

// BranchName returns the feature branch name.
func (d DerivedIdentifiers) BranchName() string {
	return fmt.Sprintf("feature/%s", d.ExecutionID.Short())
}

// WorkspacePath returns the workspace directory path.
func (d DerivedIdentifiers) WorkspacePath(basePath string) string {
	return fmt.Sprintf("%s/%s", basePath, d.ExecutionID.String())
}

// LockKey returns the repository-branch lock key.
func (d DerivedIdentifiers) LockKey(targetBranch string) string {
	return fmt.Sprintf("repo:%s:branch:%s", d.RepoID, targetBranch)
}

// ArtifactPrefix returns the S3 artifact prefix.
func (d DerivedIdentifiers) ArtifactPrefix() string {
	return fmt.Sprintf("%s/%s/", d.TenantID, d.ExecutionID.String())
}

// CredentialLeaseKey returns the credential lease lock key.
func (d DerivedIdentifiers) CredentialLeaseKey() string {
	return fmt.Sprintf("credential:%s:lease:%s", d.RepoID, d.ExecutionID.String())
}

// NewDerivedIdentifiers creates derived identifiers for an execution.
func NewDerivedIdentifiers(execID ExecutionID, tenantID, repoID string) DerivedIdentifiers {
	return DerivedIdentifiers{
		ExecutionID: execID,
		TenantID:    tenantID,
		RepoID:      repoID,
	}
}
