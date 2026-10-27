package chathistory

import (
	"context"
	"errors"
	"strings"
	"time"
)

var ErrNotFound = errors.New("chathistory: conversation not found")

type Conversation struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	MessageCount int       `json:"message_count"`
}

type Message struct {
	ID        int64     `json:"id"`
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

type Store interface {
	AppendMessage(ctx context.Context, tenantID, userID, conversationID, role, content string) error
	ListConversations(ctx context.Context, tenantID, userID string, limit int) ([]Conversation, error)
	GetMessages(ctx context.Context, tenantID, userID, conversationID string) ([]Message, error)
	DeleteConversation(ctx context.Context, tenantID, userID, conversationID string) error
}

func conversationTitle(role, content string) string {
	if role != "user" {
		return "New conversation"
	}
	title := strings.Join(strings.Fields(content), " ")
	if title == "" {
		return "New conversation"
	}
	if len([]rune(title)) > 80 {
		title = string([]rune(title)[:80]) + "…"
	}
	return title
}
