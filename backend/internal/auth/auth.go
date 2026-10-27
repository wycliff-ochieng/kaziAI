package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

var (
	ErrNotFound           = errors.New("auth: user not found")
	ErrEmailTaken         = errors.New("auth: email already registered")
	ErrInvalidCredentials = errors.New("auth: invalid credentials")
)

const SessionCookieName = "kazi_session"

// User is the safe, public representation of an account.
type User struct {
	ID        string    `json:"id"`
	TenantID  string    `json:"tenant_id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Account carries the password hash and must never be serialized.
type Account struct {
	User
	PasswordHash string
}

type Store interface {
	CreateUser(ctx context.Context, user User, passwordHash string) error
	GetAccountByEmail(ctx context.Context, email string) (*Account, error)
	GetAccountByID(ctx context.Context, id string) (*Account, error)
	GetUserByID(ctx context.Context, id string) (*User, error)
	UpdateProfile(ctx context.Context, id, name, email string) (*User, error)
	UpdatePassword(ctx context.Context, id, passwordHash string) error
	CreateSession(ctx context.Context, userID string, ttl time.Duration) (string, error)
	GetUserBySession(ctx context.Context, token string) (*User, error)
	DeleteSession(ctx context.Context, token string) error
	DeleteUserSessions(ctx context.Context, userID string) error
}

// NewID returns a cryptographically random, prefixed identifier.
func NewID(prefix string) string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return prefix + "_" + hex.EncodeToString([]byte(time.Now().Format(time.RFC3339Nano)))
	}
	return prefix + "_" + hex.EncodeToString(buf)
}

// NewSessionToken returns a URL-safe bearer token and its storage hash.
func NewSessionToken() (string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	return token, hashToken(token), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func ValidEmail(email string) bool {
	email = NormalizeEmail(email)
	at := strings.IndexByte(email, '@')
	return at > 0 && at < len(email)-1 && !strings.ContainsAny(email, " \t\r\n")
}

func ValidPassword(password string) bool {
	return len(password) >= 8
}
