package domain

import (
	"time"

	"github.com/google/uuid"
)

type AuditAction string

const (
	AuditActionCreate AuditAction = "create"
	AuditActionUpdate AuditAction = "update"
	AuditActionDelete AuditAction = "delete"
	AuditActionLogin  AuditAction = "login"
)

// DiffEntry captures the before/after value of a single field.
type DiffEntry struct {
	Old any `json:"old"`
	New any `json:"new"`
}

// AuditLog is an immutable record of every state-changing action.
// OldState/NewState/Diff are stored as JSONB in Postgres for flexible querying.
type AuditLog struct {
	ID           uuid.UUID            `json:"id"`
	ActorID      *uuid.UUID           `json:"actor_id"`
	ResourceType string               `json:"resource_type"`
	ResourceID   uuid.UUID            `json:"resource_id"`
	Action       AuditAction          `json:"action"`
	OldState     map[string]any       `json:"old_state,omitempty"`
	NewState     map[string]any       `json:"new_state,omitempty"`
	Diff         map[string]DiffEntry `json:"diff,omitempty"`
	IPAddress    string               `json:"ip_address,omitempty"`
	UserAgent    string               `json:"user_agent,omitempty"`
	CreatedAt    time.Time            `json:"created_at"`
}
