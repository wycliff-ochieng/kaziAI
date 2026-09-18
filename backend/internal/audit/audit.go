package audit

import (
	"context"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// Event is one immutable audit row: every prompt, intent, target and approval.
// Hash-chained (locked Q8): hash = sha256(prev_hash + canonical fields).
type Event struct {
	ID         int64     `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	TenantID   string    `json:"tenant_id"`
	Actor      string    `json:"actor"`
	Channel    string    `json:"channel"` // "ws-chat"
	Prompt     string    `json:"prompt"`
	Intent     string    `json:"intent"`
	Target     string    `json:"target,omitempty"`
	Approval   string    `json:"approval"` // "none" | "pending" | "approved" | "rejected" | "executed"
	PrevHash   string    `json:"prev_hash"`
	Hash       string    `json:"hash"`
}

// Store is append-only. MemoryStore serves tests/dev; PostgresStore is production.
type Store interface {
	Append(ctx context.Context, e Event) (Event, error)
	List(ctx context.Context, tenantID string, limit int) ([]Event, error)
	Verify(ctx context.Context, tenantID string) (checked int64, ok bool, err error)
}

func canonical(e Event) string {
	ts := e.Timestamp.UTC().Format(time.RFC3339Nano)
	return strings.Join([]string{
		e.PrevHash, ts, e.TenantID, e.Actor, e.Channel,
		e.Prompt, e.Intent, e.Target, e.Approval,
	}, "\x1f")
}

// Chain computes the row hash over prev hash + canonical fields.
func Chain(prevHash string, e Event) string {
	sum := sha256.Sum256([]byte(canonical(Event{
		PrevHash:   prevHash,
		Timestamp:  e.Timestamp,
		TenantID:   e.TenantID,
		Actor:      e.Actor,
		Channel:    e.Channel,
		Prompt:     e.Prompt,
		Intent:     e.Intent,
		Target:     e.Target,
		Approval:   e.Approval,
	})))
	return hex.EncodeToString(sum[:])
}

// ExportCSV renders events for auditors (newest last).
func ExportCSV(events []Event) (string, error) {
	var sb strings.Builder
	w := csv.NewWriter(&sb)
	if err := w.Write([]string{"id", "timestamp", "tenant", "actor", "channel", "prompt", "intent", "target", "approval", "prev_hash", "hash"}); err != nil {
		return "", err
	}
	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
		if err := w.Write([]string{
			fmt.Sprintf("%d", e.ID),
			e.Timestamp.UTC().Format(time.RFC3339),
			e.TenantID, e.Actor, e.Channel, e.Prompt,
			e.Intent, e.Target, e.Approval, e.PrevHash, e.Hash,
		}); err != nil {
			return "", err
		}
	}
	w.Flush()
	return sb.String(), w.Error()
}
