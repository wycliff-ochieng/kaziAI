package audit

import (
	"context"
	"strings"
	"testing"
)

func TestMemoryStore_ChainAndVerify(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	first, err := s.Append(ctx, Event{TenantID: "t1", Actor: "ops", Channel: "ws-chat", Prompt: "show vendors", Intent: "GET_VENDORS", Approval: "none"})
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}
	if first.ID != 1 || first.Hash == "" {
		t.Errorf("expected chained row, got %+v", first)
	}
	if first.PrevHash != "" {
		t.Errorf("genesis prev_hash must be empty, got %q", first.PrevHash)
	}

	second, err := s.Append(ctx, Event{TenantID: "t1", Actor: "ops", Channel: "ws-chat", Prompt: "okay", Intent: "AFFIRM", Approval: "executed", Target: "PO00501"})
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}
	if second.PrevHash != first.Hash {
		t.Errorf("chain link broken: %q != %q", second.PrevHash, first.Hash)
	}

	list, err := s.List(ctx, "t1", 10)
	if err != nil || len(list) != 2 {
		t.Fatalf("List failed: %v %d", err, len(list))
	}
	if list[0].ID != 2 {
		t.Errorf("expected newest-first, got ID %d", list[0].ID)
	}

	checked, ok, err := s.Verify(ctx, "t1")
	if err != nil || !ok || checked != 2 {
		t.Errorf("Verify failed: %v %v %d", err, ok, checked)
	}

	// Tamper detection: mutate through the exported path is impossible
	// (append-only), so corrupt a copy's hash expectation instead.
	bad := second
	bad.Approval = "forged"
	if Chain(second.PrevHash, bad) == second.Hash {
		t.Errorf("forged event must not verify against stored hash")
	}
}

func TestMemoryStore_TenantIsolation(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	_, _ = s.Append(ctx, Event{TenantID: "t1", Prompt: "a", Intent: "X"})
	_, _ = s.Append(ctx, Event{TenantID: "t2", Prompt: "b", Intent: "Y"})

	list, _ := s.List(ctx, "t2", 10)
	if len(list) != 1 || list[0].Prompt != "b" {
		t.Errorf("tenant isolation broken: %+v", list)
	}
}

func TestExportCSV(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryStore()

	_, _ = s.Append(ctx, Event{TenantID: "t1", Actor: "ops", Prompt: "a,b", Intent: "X", Approval: "none"})
	events, _ := s.List(ctx, "t1", 10)
	out, err := ExportCSV(events)
	if err != nil {
		t.Fatalf("ExportCSV failed: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Errorf("expected header + 1 row, got %d lines", len(lines))
	}
	if !strings.HasPrefix(lines[0], "id,timestamp,tenant") {
		t.Errorf("bad header: %q", lines[0])
	}
}
