package connectors

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/lib/pq"
)

// Schema is idempotent: safe to run on every boot.
const Schema = `
CREATE TABLE IF NOT EXISTS tenant_connectors (
	tenant_id TEXT NOT NULL,
	name TEXT NOT NULL DEFAULT 'default',
	erp_type TEXT NOT NULL DEFAULT 'MOCK',
	base_url TEXT NOT NULL DEFAULT '',
	db_name TEXT NOT NULL DEFAULT '',
	username TEXT NOT NULL DEFAULT '',
	password TEXT NOT NULL DEFAULT '',
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY (tenant_id, name)
);
`

// PostgresStore is the production connector Store.
type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) (*PostgresStore, error) {
	if _, err := db.Exec(Schema); err != nil {
		return nil, fmt.Errorf("connectors migrate failed: %w", err)
	}
	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) Upsert(ctx context.Context, c TenantConnector) (TenantConnector, error) {
	if c.Name == "" {
		c.Name = "default"
	}
	c.UpdatedAt = time.Now()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO tenant_connectors (tenant_id, name, erp_type, base_url, db_name, username, password, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 ON CONFLICT (tenant_id, name) DO UPDATE SET
		   erp_type = EXCLUDED.erp_type, base_url = EXCLUDED.base_url,
		   db_name = EXCLUDED.db_name, username = EXCLUDED.username,
		   password = EXCLUDED.password, updated_at = EXCLUDED.updated_at`,
		c.TenantID, c.Name, c.ERPType, c.BaseURL, c.DBName, c.Username, c.Password, c.UpdatedAt)
	return c, err
}

func scanConnector(scan func(...any) error) (TenantConnector, error) {
	var c TenantConnector
	err := scan(&c.TenantID, &c.Name, &c.ERPType, &c.BaseURL, &c.DBName, &c.Username, &c.Password, &c.UpdatedAt)
	return c, err
}

func (s *PostgresStore) List(ctx context.Context, tenantID string) ([]TenantConnector, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT tenant_id, name, erp_type, base_url, db_name, username, password, updated_at
		 FROM tenant_connectors WHERE tenant_id = $1 ORDER BY name`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TenantConnector
	for rows.Next() {
		c, err := scanConnector(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *PostgresStore) Get(ctx context.Context, tenantID, name string) (*TenantConnector, error) {
	if name == "" {
		name = "default"
	}
	c, err := scanConnector(func(dest ...any) error {
		return s.db.QueryRowContext(ctx,
			`SELECT tenant_id, name, erp_type, base_url, db_name, username, password, updated_at
			 FROM tenant_connectors WHERE tenant_id = $1 AND name = $2`, tenantID, name).Scan(dest...)
	})
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *PostgresStore) Delete(ctx context.Context, tenantID, name string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM tenant_connectors WHERE tenant_id = $1 AND name = $2`, tenantID, name)
	return err
}
