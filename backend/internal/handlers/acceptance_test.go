package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"erpagent-backend/internal/audit"
	"erpagent-backend/internal/config"
	"erpagent-backend/internal/connectors"
	"erpagent-backend/internal/realtime"
)

// TestAcceptance_Q10Demo is the locked Q10 acceptance demo on mock (CI):
// "order 10 chairs from ABC" streams live (status -> text -> table ->
// dry-run card), "confirmed" executes, the activity feed pushes, and
// hash-chained audit rows land.
func TestAcceptance_Q10Demo(t *testing.T) {
	aiStub := mockAIEngineStub(t)
	defer aiStub.Close()

	cfg := &config.Config{Port: "3000", AIEngineURL: aiStub.URL}
	chatStore := realtime.NewMemoryStore()
	auditStore := audit.NewMemoryStore()
	connStore := connectors.NewMemoryStore()

	// Observe tenant activity exactly like the frontend feed does.
	actCh, unsub, err := chatStore.SubscribeActivity(context.Background(), "t1")
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	defer unsub()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", ChatSocket(cfg, chatStore, auditStore, connStore))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?tenant_id=t1&conversation_id=demo1&erp_type=MOCK"
	conn := wsDial(t, wsURL)

	// ACT 1: read vendors -> table block streams live.
	chatSend(t, conn, "demo1", "show vendors")
	text, blocks := turnTextAndBlocks(readTurn(t, conn))
	if !strings.Contains(text, "Found") || !blockTypes(blocks)[realtime.BlockTable] {
		t.Fatalf("act 1: expected vendor table, got %q %+v", text, blocks)
	}

	// ACT 2: RFQ draft -> dry-run + approval cards, pending held.
	chatSend(t, conn, "demo1", "create RFQ from ABC for 10 of Chair")
	text, blocks = turnTextAndBlocks(readTurn(t, conn))
	bt := blockTypes(blocks)
	if !bt[realtime.BlockDryRun] || !bt[realtime.BlockApproval] {
		t.Fatalf("act 2: expected dry_run + approval, got %q %+v", text, blocks)
	}

	// ACT 3: conversational approval executes live, no new approval card.
	chatSend(t, conn, "demo1", "confirmed")
	text, blocks = turnTextAndBlocks(readTurn(t, conn))
	if !strings.Contains(strings.ToLower(text), "drafted") {
		t.Fatalf("act 3: expected live execution, got %q", text)
	}
	if bt := blockTypes(blocks); bt[realtime.BlockApproval] || bt[realtime.BlockDryRun] {
		t.Fatalf("act 3: executed action must not re-issue approval, got %+v", blocks)
	}

	// ASSERT: activity feed pushed approval_pending then executed_*.
	deadline := time.Now().Add(5 * time.Second)
	var sawPending, sawExecuted bool
	for !(sawPending && sawExecuted) && time.Now().Before(deadline) {
		select {
		case ev := <-actCh:
			if ev.Kind == "approval_pending" {
				sawPending = true
			}
			if strings.HasPrefix(ev.Kind, "executed_") {
				sawExecuted = true
			}
		case <-time.After(200 * time.Millisecond):
		}
	}
	if !sawPending {
		t.Errorf("expected approval_pending activity push")
	}
	if !sawExecuted {
		t.Errorf("expected executed_* activity push")
	}

	// ASSERT: hash-chained audit rows landed (pending + executed + read).
	rows, err := auditStore.List(context.Background(), "t1", 50)
	if err != nil {
		t.Fatalf("audit list failed: %v", err)
	}
	var approvals []string
	for _, row := range rows {
		approvals = append(approvals, row.Approval)
	}
	has := func(want string) bool {
		for _, a := range approvals {
			if a == want {
				return true
			}
		}
		return false
	}
	if !has("pending") || !has("executed") {
		t.Errorf("expected pending + executed audit rows, got %v", approvals)
	}
	if checked, ok, err := auditStore.Verify(context.Background(), "t1"); err != nil || !ok || checked == 0 {
		t.Errorf("audit chain must verify: %v %v %d", err, ok, checked)
	}
}
