package llmsettings

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Config is the runtime LLM configuration. APIKey is never serialized to
// clients; use Redacted for API responses.
type Config struct {
	Provider  string    `json:"provider"`
	BaseURL   string    `json:"base_url"`
	Model     string    `json:"model"`
	APIKey    string    `json:"-"`
	MaxTokens int       `json:"max_tokens"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (c Config) Redacted() map[string]any {
	return map[string]any{
		"provider":    c.Provider,
		"base_url":    c.BaseURL,
		"model":       c.Model,
		"max_tokens":  c.MaxTokens,
		"has_api_key": c.APIKey != "",
		"updated_at":  c.UpdatedAt,
	}
}

func (c Config) WithDefaults() Config {
	if strings.TrimSpace(c.Provider) == "" {
		c.Provider = "groq"
	}
	if strings.TrimSpace(c.BaseURL) == "" {
		c.BaseURL = "https://api.groq.com/openai/v1"
	}
	if strings.TrimSpace(c.Model) == "" {
		c.Model = "llama3-70b-8192"
	}
	if c.MaxTokens <= 0 {
		c.MaxTokens = 400
	}
	if c.MaxTokens > 8192 {
		c.MaxTokens = 8192
	}
	return c
}

type Store interface {
	Get(ctx context.Context) (*Config, error)
	Save(ctx context.Context, c Config) (Config, error)
}

// MemoryStore is used when Postgres is unavailable (local dev/tests).
type MemoryStore struct {
	mu   sync.RWMutex
	data *Config
}

func NewMemoryStore() *MemoryStore { return &MemoryStore{} }

func (m *MemoryStore) Get(ctx context.Context) (*Config, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.data == nil {
		return nil, nil
	}
	c := *m.data
	return &c, nil
}

func (m *MemoryStore) Save(ctx context.Context, c Config) (Config, error) {
	c = c.WithDefaults()
	c.UpdatedAt = time.Now().UTC()
	m.mu.Lock()
	defer m.mu.Unlock()
	if c.APIKey == "" && m.data != nil {
		c.APIKey = m.data.APIKey
	}
	m.data = &c
	return c, nil
}

const Schema = `
CREATE TABLE IF NOT EXISTS llm_settings (
	id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
	provider TEXT NOT NULL DEFAULT 'groq',
	base_url TEXT NOT NULL DEFAULT '',
	model TEXT NOT NULL DEFAULT '',
	api_key_enc TEXT NOT NULL DEFAULT '',
	max_tokens INTEGER NOT NULL DEFAULT 400,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
`

type PostgresStore struct {
	db  *sql.DB
	box *cipherBox
}

func NewPostgresStore(db *sql.DB, encryptionSecret string) (*PostgresStore, error) {
	if _, err := db.Exec(Schema); err != nil {
		return nil, fmt.Errorf("llm settings migrate failed: %w", err)
	}
	return &PostgresStore{db: db, box: newCipherBox(encryptionSecret)}, nil
}

func (s *PostgresStore) Get(ctx context.Context) (*Config, error) {
	var c Config
	var enc string
	err := s.db.QueryRowContext(ctx,
		`SELECT provider, base_url, model, api_key_enc, max_tokens, updated_at
		 FROM llm_settings WHERE id = TRUE`).Scan(
		&c.Provider, &c.BaseURL, &c.Model, &enc, &c.MaxTokens, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	key, err := s.box.decrypt(enc)
	if err != nil {
		return nil, fmt.Errorf("decrypt llm api key: %w", err)
	}
	c.APIKey = key
	return &c, nil
}

func (s *PostgresStore) Save(ctx context.Context, c Config) (Config, error) {
	c = c.WithDefaults()
	c.UpdatedAt = time.Now().UTC()

	// Empty api_key means "keep the existing secret"; this lets the UI update
	// model/base URL without forcing the operator to retype the key.
	if c.APIKey == "" {
		var existing string
		err := s.db.QueryRowContext(ctx, `SELECT api_key_enc FROM llm_settings WHERE id = TRUE`).Scan(&existing)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return c, err
		}
		if existing != "" {
			key, err := s.box.decrypt(existing)
			if err != nil {
				return c, err
			}
			c.APIKey = key
		}
	}

	enc, err := s.box.encrypt(c.APIKey)
	if err != nil {
		return c, err
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO llm_settings (id, provider, base_url, model, api_key_enc, max_tokens, updated_at)
		 VALUES (TRUE,$1,$2,$3,$4,$5,$6)
		 ON CONFLICT (id) DO UPDATE SET
		   provider = EXCLUDED.provider, base_url = EXCLUDED.base_url,
		   model = EXCLUDED.model, api_key_enc = EXCLUDED.api_key_enc,
		   max_tokens = EXCLUDED.max_tokens, updated_at = EXCLUDED.updated_at`,
		c.Provider, c.BaseURL, c.Model, enc, c.MaxTokens, c.UpdatedAt)
	return c, err
}

type cipherBox struct {
	aead cipher.AEAD
}

func newCipherBox(secret string) *cipherBox {
	if strings.TrimSpace(secret) == "" {
		// Dev fallback. Production deployments must set SETTINGS_ENCRYPTION_KEY.
		secret = "erpagent-dev-only-change-me"
	}
	key := sha256.Sum256([]byte(secret))
	block, _ := aes.NewCipher(key[:])
	aead, _ := cipher.NewGCM(block)
	return &cipherBox{aead: aead}
}

func (b *cipherBox) encrypt(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	out := b.aead.Seal(nonce, nonce, []byte(plain), nil)
	return base64.RawStdEncoding.EncodeToString(out), nil
}

func (b *cipherBox) decrypt(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil {
		return "", err
	}
	ns := b.aead.NonceSize()
	if len(raw) < ns {
		return "", errors.New("ciphertext too short")
	}
	plain, err := b.aead.Open(nil, raw[:ns], raw[ns:], nil)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
