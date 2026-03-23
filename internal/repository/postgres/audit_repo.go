package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Allmight-456/ticketflow/internal/domain"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AuditRepo struct {
	db *pgxpool.Pool
}

func NewAuditRepo(db *pgxpool.Pool) *AuditRepo {
	return &AuditRepo{db: db}
}

// Log inserts an audit record. Pass a pgx.Tx to write within an existing transaction.
func (r *AuditRepo) Log(ctx context.Context, a *domain.AuditLog) error {
	return r.logWithQuerier(ctx, r.db, a)
}

// LogInTx writes the audit record inside the provided transaction so the log
// and the business mutation commit or rollback together.
func (r *AuditRepo) LogInTx(ctx context.Context, tx pgx.Tx, a *domain.AuditLog) error {
	return r.logWithQuerier(ctx, tx, a)
}

type querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func (r *AuditRepo) logWithQuerier(ctx context.Context, q querier, a *domain.AuditLog) error {
	oldJSON, _ := json.Marshal(a.OldState)
	newJSON, _ := json.Marshal(a.NewState)
	diffJSON, _ := json.Marshal(a.Diff)

	const query = `
		INSERT INTO audit_logs
			(actor_id, resource_type, resource_id, action, old_state, new_state, diff, ip_address, user_agent)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		RETURNING id, created_at`

	// ip_address is INET in postgres; pass as string and let the driver cast it.
	var ipStr *string
	if a.IPAddress != "" {
		ipStr = &a.IPAddress
	}

	return q.QueryRow(ctx, query,
		a.ActorID, a.ResourceType, a.ResourceID, a.Action,
		nullableJSON(oldJSON), nullableJSON(newJSON), nullableJSON(diffJSON),
		ipStr, nullableStr(a.UserAgent),
	).Scan(&a.ID, &a.CreatedAt)
}

func (r *AuditRepo) GetHistory(ctx context.Context, resourceType string, resourceID uuid.UUID) ([]domain.AuditLog, error) {
	const q = `
		SELECT id, actor_id, resource_type, resource_id, action,
		       old_state, new_state, diff, ip_address::text, user_agent, created_at
		FROM audit_logs
		WHERE resource_type=$1 AND resource_id=$2
		ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, q, resourceType, resourceID)
	if err != nil {
		return nil, fmt.Errorf("get audit history: %w", err)
	}
	defer rows.Close()

	var logs []domain.AuditLog
	for rows.Next() {
		var a domain.AuditLog
		var oldRaw, newRaw, diffRaw []byte
		// ip_address and user_agent are nullable — scan into pointers so NULL → nil
		var ipStr, uaStr *string

		if err := rows.Scan(
			&a.ID, &a.ActorID, &a.ResourceType, &a.ResourceID, &a.Action,
			&oldRaw, &newRaw, &diffRaw, &ipStr, &uaStr, &a.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan audit row: %w", err)
		}

		if ipStr != nil {
			a.IPAddress = *ipStr
		}
		if uaStr != nil {
			a.UserAgent = *uaStr
		}
		unmarshalJSON(oldRaw, &a.OldState)
		unmarshalJSON(newRaw, &a.NewState)

		var rawDiff map[string]json.RawMessage
		if err := unmarshalJSON(diffRaw, &rawDiff); err == nil {
			a.Diff = make(map[string]domain.DiffEntry, len(rawDiff))
			for k, v := range rawDiff {
				var entry domain.DiffEntry
				_ = json.Unmarshal(v, &entry)
				a.Diff[k] = entry
			}
		}

		logs = append(logs, a)
	}
	return logs, rows.Err()
}

// nullableJSON returns nil for an empty/null JSON value so the DB stores NULL.
func nullableJSON(b []byte) any {
	if string(b) == "null" || len(b) == 0 {
		return nil
	}
	return b
}

func nullableStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func unmarshalJSON(data []byte, v any) error {
	if len(data) == 0 {
		return nil
	}
	return json.Unmarshal(data, v)
}
