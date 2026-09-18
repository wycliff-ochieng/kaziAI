package realtime

import (
	"context"
	"testing"
	"time"
)

func TestMemoryStore_PendingLifecycle(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	if got, _ := s.TakePending(ctx, "t1", "c1"); got != nil {
		t.Fatalf("expected no pending, got %+v", got)
	}

	if err := s.SavePending(ctx, PendingAction{TenantID: "t1", ConversationID: "c1", Prompt: "confirm PO", Intent: []byte(`{"intent_type":"CONFIRM_PURCHASE_ORDER"}`)}); err != nil {
		t.Fatalf("SavePending failed: %v", err)
	}

	got, err := s.TakePending(ctx, "t1", "c1")
	if err != nil || got == nil {
		t.Fatalf("TakePending failed: %v %+v", err, got)
	}
	if got.Prompt != "confirm PO" {
		t.Errorf("expected prompt back, got %q", got.Prompt)
	}

	// Take is destructive: second take finds nothing.
	if again, _ := s.TakePending(ctx, "t1", "c1"); again != nil {
		t.Errorf("expected pending consumed, got %+v", again)
	}

	// Drop is idempotent.
	if err := s.DropPending(ctx, "t1", "c1"); err != nil {
		t.Errorf("DropPending failed: %v", err)
	}
}

func TestMemoryStore_HistoryWindow(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	for i := 0; i < 25; i++ {
		_ = s.AppendHistory(ctx, "t1", "c1", "user", "msg")
	}
	h, err := s.History(ctx, "t1", "c1", 30)
	if err != nil {
		t.Fatalf("History failed: %v", err)
	}
	if len(h) != 20 {
		t.Errorf("expected windowed history of 20, got %d", len(h))
	}
	last3, _ := s.History(ctx, "t1", "c1", 3)
	if len(last3) != 3 {
		t.Errorf("expected 3 messages, got %d", len(last3))
	}
}

func TestMemoryStore_PubSub(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	ch, unsub, err := s.SubscribeActivity(ctx, "t1")
	if err != nil {
		t.Fatalf("SubscribeActivity failed: %v", err)
	}
	defer unsub()

	ev := ActivityEvent{Kind: "approval_pending", Title: "PO ready", TenantID: "t1"}
	if err := s.PublishActivity(ctx, ev); err != nil {
		t.Fatalf("PublishActivity failed: %v", err)
	}

	select {
	case got := <-ch:
		if got.Kind != "approval_pending" || got.Title != "PO ready" {
			t.Errorf("unexpected event %+v", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for activity event")
	}

	// Other tenants are isolated.
	other, unsubOther, _ := s.SubscribeActivity(ctx, "t2")
	defer unsubOther()
	_ = s.PublishActivity(ctx, ActivityEvent{Kind: "x", TenantID: "t1"})
	select {
	case got := <-other:
		t.Errorf("tenant isolation broken, t2 got %+v", got)
	case <-time.After(100 * time.Millisecond):
	}
}
