package database

import (
	"context"
	"database/sql"
	"time"

	_ "github.com/lib/pq"
	"erpagent-backend/internal/config"
)

// OpenPostgres opens the shared Postgres pool (pgvector image in compose).
// Callers fall back to in-memory stores when this errors (local dev without PG).
func OpenPostgres(cfg *config.Config) (*sql.DB, error) {
	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}
