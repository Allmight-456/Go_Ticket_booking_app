package service

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Allmight-456/ticketflow/internal/domain"
	"github.com/Allmight-456/ticketflow/internal/repository/postgres"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type AuditService struct {
	repo *postgres.AuditRepo
}

func NewAuditService(repo *postgres.AuditRepo) *AuditService {
	return &AuditService{repo: repo}
}

// Log records an auditable action.
func (s *AuditService) Log(ctx context.Context, a *domain.AuditLog) error {
	return s.repo.Log(ctx, a)
}

// LogInTx records an auditable action within the provided transaction.
func (s *AuditService) LogInTx(ctx context.Context, tx pgx.Tx, a *domain.AuditLog) error {
	return s.repo.LogInTx(ctx, tx, a)
}

// GetHistory returns the full audit trail for a given resource, newest first.
func (s *AuditService) GetHistory(ctx context.Context, resourceType string, resourceID uuid.UUID) ([]domain.AuditLog, error) {
	logs, err := s.repo.GetHistory(ctx, resourceType, resourceID)
	if err != nil {
		return nil, fmt.Errorf("get history: %w", err)
	}
	return logs, nil
}

// ComputeDiff produces a field-level diff between two states.
// Both states are map[string]any, typically derived from JSON marshalling a struct.
// Only changed fields appear in the result, keeping audit storage minimal.
func ComputeDiff(oldState, newState map[string]any) map[string]domain.DiffEntry {
	diff := make(map[string]domain.DiffEntry)

	for key, newVal := range newState {
		oldVal, exists := oldState[key]
		if !exists || fmt.Sprintf("%v", oldVal) != fmt.Sprintf("%v", newVal) {
			diff[key] = domain.DiffEntry{Old: oldVal, New: newVal}
		}
	}

	// Also capture fields removed in new state
	for key, oldVal := range oldState {
		if _, exists := newState[key]; !exists {
			diff[key] = domain.DiffEntry{Old: oldVal, New: nil}
		}
	}

	return diff
}

// ToMap converts any struct to a map[string]any via JSON round-trip.
// Used to produce old_state / new_state snapshots for audit logs.
func ToMap(v any) map[string]any {
	b, _ := json.Marshal(v)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}
