package chathistory

import (
	"context"
	"testing"
)

func TestMemoryStoreConversationLifecycle(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()

	if err := store.AppendMessage(ctx, "tenant_a", "user_1", "conv_1", "user", "Show my open orders"); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendMessage(ctx, "tenant_a", "user_1", "conv_1", "assistant", "Here are your orders."); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendMessage(ctx, "tenant_a", "user_1", "conv_2", "user", "Show vendors"); err != nil {
		t.Fatal(err)
	}

	list, err := store.ListConversations(ctx, "tenant_a", "user_1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 conversations, got %d", len(list))
	}

	msgs, err := store.GetMessages(ctx, "tenant_a", "user_1", "conv_1")
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 2 || msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Fatalf("unexpected messages: %#v", msgs)
	}
	if list[0].Title == "" || list[0].Title == "New conversation" {
		t.Fatalf("expected a title, got %q", list[0].Title)
	}

	if _, err := store.GetMessages(ctx, "tenant_a", "user_2", "conv_1"); err != ErrNotFound {
		t.Fatalf("cross-user read err = %v, want ErrNotFound", err)
	}
	if err := store.DeleteConversation(ctx, "tenant_a", "user_1", "conv_1"); err != nil {
		t.Fatal(err)
	}
	if _, err := store.GetMessages(ctx, "tenant_a", "user_1", "conv_1"); err != ErrNotFound {
		t.Fatalf("deleted conversation err = %v, want ErrNotFound", err)
	}
}
