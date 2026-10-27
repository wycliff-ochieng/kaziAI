package chathistory

import (
	"context"
	"database/sql"
	"errors"
)

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
CREATE TABLE IF NOT EXISTS chat_conversations (
    tenant_id TEXT NOT NULL,
    id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    title TEXT NOT NULL DEFAULT 'New conversation',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, id)
);
CREATE INDEX IF NOT EXISTS idx_chat_conversations_user ON chat_conversations(tenant_id, user_id, updated_at DESC);
CREATE TABLE IF NOT EXISTS chat_messages (
    id BIGSERIAL PRIMARY KEY,
    tenant_id TEXT NOT NULL,
    conversation_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (tenant_id, conversation_id) REFERENCES chat_conversations(tenant_id, id) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_chat_messages_conversation ON chat_messages(tenant_id, conversation_id, id);
`)
	return err
}

func (s *PostgresStore) AppendMessage(ctx context.Context, tenantID, userID, conversationID, role, content string) error {
	if conversationID == "" {
		return ErrNotFound
	}
	title := conversationTitle(role, content)
	res, err := s.db.ExecContext(ctx, `
INSERT INTO chat_conversations (tenant_id, id, user_id, title)
VALUES ($1, $2, $3, $4)
ON CONFLICT (tenant_id, id) DO UPDATE
SET updated_at = now(),
    title = CASE
        WHEN chat_conversations.title = '' OR chat_conversations.title = 'New conversation'
        THEN EXCLUDED.title
        ELSE chat_conversations.title
    END
WHERE chat_conversations.user_id = EXCLUDED.user_id`,
		tenantID, conversationID, userID, title)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO chat_messages (tenant_id, conversation_id, role, content)
SELECT $1, $2, $3, $4
WHERE EXISTS (
    SELECT 1 FROM chat_conversations
    WHERE tenant_id = $1 AND id = $2 AND user_id = $5
)`, tenantID, conversationID, role, content, userID)
	return err
}

func (s *PostgresStore) ListConversations(ctx context.Context, tenantID, userID string, limit int) ([]Conversation, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT c.id, c.title, c.created_at, c.updated_at, COUNT(m.id)
FROM chat_conversations c
LEFT JOIN chat_messages m ON m.tenant_id = c.tenant_id AND m.conversation_id = c.id
WHERE c.tenant_id = $1 AND c.user_id = $2
GROUP BY c.id, c.title, c.created_at, c.updated_at
ORDER BY c.updated_at DESC
LIMIT $3`, tenantID, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Conversation, 0)
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.ID, &c.Title, &c.CreatedAt, &c.UpdatedAt, &c.MessageCount); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *PostgresStore) GetMessages(ctx context.Context, tenantID, userID, conversationID string) ([]Message, error) {
	var exists int
	err := s.db.QueryRowContext(ctx, `
SELECT 1 FROM chat_conversations WHERE tenant_id = $1 AND id = $2 AND user_id = $3`,
		tenantID, conversationID, userID).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `
SELECT id, role, content, created_at FROM chat_messages
WHERE tenant_id = $1 AND conversation_id = $2 ORDER BY id ASC`, tenantID, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]Message, 0)
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Role, &m.Content, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *PostgresStore) DeleteConversation(ctx context.Context, tenantID, userID, conversationID string) error {
	res, err := s.db.ExecContext(ctx, `
DELETE FROM chat_conversations WHERE tenant_id = $1 AND id = $2 AND user_id = $3`,
		tenantID, conversationID, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
