package audit

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// Schema is idempotent: safe to run on every boot.
const Schema = `
CREATE TABLE IF NOT EXISTS audit_events (
	id BIGSERIAL PRIMARY KEY,
	ts TIMESTAMPTZ NOT NULL DEFAULT now(),
	tenant_id TEXT NOT NULL DEFAULT '',
	actor TEXT NOT NULL DEFAULT '',
	channel TEXT NOT NULL DEFAULT '',
	prompt TEXT NOT NULL DEFAULT '',
	intent TEXT NOT NULL DEFAULT '',
	target TEXT NOT NULL DEFAULT '',
	approval TEXT NOT NULL DEFAULT 'none',
	prev_hash TEXT NOT NULL DEFAULT '',
	hash TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_audit_tenant_ts ON audit_events (tenant_id, id DESC);
`

// PostgresStore is the production audit Store. Append serializes writers per
// tenant via a transaction holding the latest hash.
type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) (*PostgresStore, error) {
	if _, err := db.Exec(Schema); err != nil {
		return nil, fmt.Errorf("audit migrate failed: %w", err)
	}
	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) Append(ctx context.Context, e Event) (Event, error) {
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Event{}, err
	}
	defer tx.Rollback()

	var prev sql.NullString
	err = tx.QueryRowContext(ctx,
		`SELECT hash FROM audit_events WHERE tenant_id = $1 ORDER BY id DESC LIMIT 1 FOR UPDATE`,
		e.TenantID).Scan(&prev)
	if err != nil && err != sql.ErrNoRows {
		return Event{}, err
	}
	if prev.Valid {
		e.PrevHash = prev.String
	}
	e.Hash = Chain(e.PrevHash, e)

	err = tx.QueryRowContext(ctx,
		`INSERT INTO audit_events (ts, tenant_id, actor, channel, prompt, intent, target, approval, prev_hash, hash)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING id`,
		e.Timestamp, e.TenantID, e.Actor, e.Channel, e.Prompt,
		e.Intent, e.Target, e.Approval, e.PrevHash, e.Hash).Scan(&e.ID)
	if err != nil {
		return Event{}, err
	}
	return e, tx.Commit()
}

func scanEvent(scan func(...any) error) (Event, error) {
	var e Event
	err := scan(&e.ID, &e.Timestamp, &e.TenantID, &e.Actor, &e.Channel,
		&e.Prompt, &e.Intent, &e.Target, &e.Approval, &e.PrevHash, &e.Hash)
	return e, err
}

func (s *PostgresStore) List(ctx context.Context, tenantID string, limit int) ([]Event, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, ts, tenant_id, actor, channel, prompt, intent, target, approval, prev_hash, hash
		 FROM audit_events WHERE ($1 = '' OR tenant_id = $1) ORDER BY id DESC LIMIT $2`,
		tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Event
	for rows.Next() {
		e, err := scanEvent(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (s *PostgresStore) Verify(ctx context.Context, tenantID string) (int64, bool, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, ts, tenant_id, actor, channel, prompt, intent, target, approval, prev_hash, hash
		 FROM audit_events WHERE ($1 = '' OR tenant_id = $1) ORDER BY id ASC`,
		tenantID)
	if err != nil {
		return 0, false, err
	}
	defer rows.Close()

	var prev string
	var checked int64
	for rows.Next() {
		e, err := scanEvent(rows.Scan)
		if err != nil {
			return checked, false, err
		}
		if e.PrevHash != prev {
			return checked, false, fmt.Errorf("chain break at event %d", e.ID)
		}
		if Chain(prev, e) != e.Hash {
			return checked, false, fmt.Errorf("hash mismatch at event %d", e.ID)
		}
		prev = e.Hash
		checked++
	}
	return checked, true, rows.Err()
}
