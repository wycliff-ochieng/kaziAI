package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/lib/pq"
)

// PostgresStore persists accounts and sessions in the shared Postgres pool.
type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore(db *sql.DB) (*PostgresStore, error) {
	s := &PostgresStore{db: db}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *PostgresStore) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    tenant_id TEXT NOT NULL UNIQUE,
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS sessions (
    token_hash TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at ON sessions(expires_at);
`)
	return err
}

func (s *PostgresStore) CreateUser(ctx context.Context, user User, passwordHash string) error {
	now := time.Now().UTC()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = now
	}
	user.UpdatedAt = now
	user.Email = NormalizeEmail(user.Email)
	_, err := s.db.ExecContext(ctx, `
INSERT INTO users (id, tenant_id, email, name, password_hash, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		user.ID, user.TenantID, user.Email, user.Name, passwordHash, user.CreatedAt, user.UpdatedAt)
	if isUniqueViolation(err) {
		return ErrEmailTaken
	}
	return err
}

func (s *PostgresStore) GetAccountByEmail(ctx context.Context, email string) (*Account, error) {
	return s.getAccount(ctx, `WHERE lower(email) = lower($1)`, NormalizeEmail(email))
}

func (s *PostgresStore) GetAccountByID(ctx context.Context, id string) (*Account, error) {
	return s.getAccount(ctx, `WHERE id = $1`, id)
}

func (s *PostgresStore) getAccount(ctx context.Context, where string, arg any) (*Account, error) {
	var acct Account
	err := s.db.QueryRowContext(ctx, `
SELECT id, tenant_id, email, name, password_hash, created_at, updated_at
FROM users `+where, arg).Scan(
		&acct.ID, &acct.TenantID, &acct.Email, &acct.Name, &acct.PasswordHash, &acct.CreatedAt, &acct.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &acct, nil
}

func (s *PostgresStore) GetUserByID(ctx context.Context, id string) (*User, error) {
	acct, err := s.GetAccountByID(ctx, id)
	if err != nil {
		return nil, err
	}
	user := acct.User
	return &user, nil
}

func (s *PostgresStore) UpdateProfile(ctx context.Context, id, name, email string) (*User, error) {
	email = NormalizeEmail(email)
	var user User
	err := s.db.QueryRowContext(ctx, `
UPDATE users SET name = $2, email = $3, updated_at = now()
WHERE id = $1
RETURNING id, tenant_id, email, name, created_at, updated_at`,
		id, name, email).Scan(&user.ID, &user.TenantID, &user.Email, &user.Name, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if isUniqueViolation(err) {
		return nil, ErrEmailTaken
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *PostgresStore) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1`, id, passwordHash)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PostgresStore) CreateSession(ctx context.Context, userID string, ttl time.Duration) (string, error) {
	token, tokenHash, err := NewSessionToken()
	if err != nil {
		return "", err
	}
	_, _ = s.db.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < now()`)
	_, err = s.db.ExecContext(ctx, `
INSERT INTO sessions (token_hash, user_id, expires_at) VALUES ($1, $2, $3)`,
		tokenHash, userID, time.Now().Add(ttl))
	if err != nil {
		return "", err
	}
	return token, nil
}

func (s *PostgresStore) GetUserBySession(ctx context.Context, token string) (*User, error) {
	var user User
	err := s.db.QueryRowContext(ctx, `
SELECT u.id, u.tenant_id, u.email, u.name, u.created_at, u.updated_at
FROM sessions s JOIN users u ON u.id = s.user_id
WHERE s.token_hash = $1 AND s.expires_at > now()`, hashToken(token)).Scan(
		&user.ID, &user.TenantID, &user.Email, &user.Name, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *PostgresStore) DeleteSession(ctx context.Context, token string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hashToken(token))
	return err
}

func (s *PostgresStore) DeleteUserSessions(ctx context.Context, userID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	return err
}

func isUniqueViolation(err error) bool {
	var pqErr *pq.Error
	return errors.As(err, &pqErr) && pqErr.Code == "23505"
}
