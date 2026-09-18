package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"erpagent-backend/internal/config"
	"erpagent-backend/internal/audit"
	"erpagent-backend/internal/connectors"
	"erpagent-backend/internal/domain"
	"erpagent-backend/internal/realtime"
)

func TestAffirmRejectMatcher(t *testing.T) {
	affirms := []string{"okay", "OK", "ok.", "yes", "confirmed", "go on", "go ahead", "proceed", "do it", "approved", "looks good", "yeah"}
	for _, s := range affirms {
		if !isAffirm(s) {
			t.Errorf("expected affirm match for %q", s)
		}
		if isReject(s) {
			t.Errorf("unexpected reject match for %q", s)
		}
	}

	rejects := []string{"no", "nope", "reject", "cancel that", "stop", "don't", "discard", "never mind"}
	for _, s := range rejects {
		if !isReject(s) {
			t.Errorf("expected reject match for %q", s)
		}
		if isAffirm(s) {
			t.Errorf("unexpected affirm match for %q", s)
		}
	}

	// Long prompts must never trigger: money actions stay deterministic.
	long := []string{
		"yes, order 10 chairs from ABC and confirm it",
		"okay so show me all vendors and their balances please confirm",
		"note the outstanding orders for follow up",
	}
	for _, s := range long {
		if isAffirm(s) || isReject(s) {
			t.Errorf("long prompt must not match approval: %q", s)
		}
	}
}

func TestUIBlocksForChat(t *testing.T) {
	// Dry-run response yields dry_run + approval blocks.
	dryResp := &ChatResponse{
		Status:               "dry_run_preview",
		Message:              "RFQ preview",
		RequiresConfirmation: true,
		ConfirmationPrompt:   "Draft?",
		DryRun: &domain.DryRunResult{
			Operation:          "CREATE_RFQ",
			ImpactSummary:      "impact",
			RiskLevel:          domain.RiskLow,
			RequiresConfirmation: true,
			ConfirmationPrompt: "Draft?",
		},
	}
	blocks := uiBlocksForChat(dryResp)
	types := map[string]bool{}
	for _, b := range blocks {
		types[b.Type] = true
	}
	if !types[realtime.BlockDryRun] || !types[realtime.BlockApproval] {
		t.Errorf("expected dry_run + approval blocks, got %+v", blocks)
	}

	// Slice data yields a table block.
	tableResp := &ChatResponse{
		Status:  "success",
		Message: "Found 2 vendor(s).",
		Data: []domain.Vendor{
			{ID: "20", Name: "ABC Ltd"},
			{ID: "21", Name: "Thika Suppliers"},
		},
	}
	blocks = uiBlocksForChat(tableResp)
	if len(blocks) == 0 || blocks[0].Type != realtime.BlockTable {
		t.Errorf("expected table block first, got %+v", blocks)
	}

	// Single record yields table + citation.
	singleResp := &ChatResponse{
		Status:  "success",
		Message: "Details retrieved.",
		Data:    domain.SaleOrder{ID: "501", OrderNo: "SO001", CustomerName: "Azure"},
	}
	blocks = uiBlocksForChat(singleResp)
	types = map[string]bool{}
	for _, b := range blocks {
		types[b.Type] = true
	}
	if !types[realtime.BlockTable] || !types[realtime.BlockCitation] {
		t.Errorf("expected table + citation blocks, got %+v", blocks)
	}
}

// mockAIEngineStub classifies the smoke-test prompts deterministically.
func mockAIEngineStub(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		prompt, _ := req["prompt"].(string)
		lower := strings.ToLower(prompt)

		intent := "UNKNOWN"
		params := map[string]any{}
		validation := map[string]any{"requires_user_approval": false}
		action := "READ"

		switch {
		case strings.Contains(lower, "vendor"):
			intent = "GET_VENDORS"
		case strings.Contains(lower, "chair"):
			intent = "CREATE_RFQ"
			action = "WRITE"
			params = map[string]any{
				"vendor_name":  "ABC",
				"product_name": "Chair",
				"quantity":     10.0,
				"lines":        []any{map[string]any{"product_name": "Chair", "quantity": 10.0}},
				"dry_run":      true,
			}
			validation = map[string]any{"requires_user_approval": true, "risk_level": "LOW", "approval_prompt": "Draft RFQ?"}
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"intent_type": intent,
			"confidence":  0.95,
			"action_type": action,
			"summary":     "stub",
			"parameters":  params,
			"validation":  validation,
		})
	}))
}

func wsDial(t *testing.T, url string) *websocket.Conn {
	t.Helper()
	dialer := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, _, err := dialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("ws dial failed: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// readTurn collects chat-channel messages until done (activity may interleave).
func readTurn(t *testing.T, conn *websocket.Conn) []realtime.ServerMessage {
	t.Helper()
	var out []realtime.ServerMessage
	for {
		_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
		var msg realtime.ServerMessage
		if err := conn.ReadJSON(&msg); err != nil {
			t.Fatalf("ws read failed: %v", err)
		}
		if msg.Channel != realtime.ChannelChat {
			continue
		}
		out = append(out, msg)
		if msg.Type == realtime.EventDone || msg.Type == realtime.EventError {
			return out
		}
	}
}

func chatSend(t *testing.T, conn *websocket.Conn, conv, text string) {
	t.Helper()
	if err := conn.WriteJSON(realtime.ClientMessage{Type: realtime.ClientChat, ConversationID: conv, Text: text}); err != nil {
		t.Fatalf("ws write failed: %v", err)
	}
}

func turnTextAndBlocks(turn []realtime.ServerMessage) (string, []realtime.UIBlock) {
	var text string
	var blocks []realtime.UIBlock
	for _, m := range turn {
		if m.Type == realtime.EventText && text == "" {
			text = m.Text
		}
		if m.Type == realtime.EventUI {
			blocks = append(blocks, m.Blocks...)
		}
	}
	return text, blocks
}

func blockTypes(blocks []realtime.UIBlock) map[string]bool {
	m := map[string]bool{}
	for _, b := range blocks {
		m[b.Type] = true
	}
	return m
}

// TestChatSocket_ConversationalApprovalFlow covers lane (a) end to end:
// read (table ui) -> RFQ dry-run (dry_run + approval ui, pending held) ->
// "okay" executes live -> stray "nope" falls through safely.
func TestChatSocket_ConversationalApprovalFlow(t *testing.T) {
	aiStub := mockAIEngineStub(t)
	defer aiStub.Close()

	cfg := &config.Config{Port: "3000", AIEngineURL: aiStub.URL}
	store := realtime.NewMemoryStore()

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", ChatSocket(cfg, store, audit.NewMemoryStore(), connectors.NewMemoryStore()))
	srv := httptest.NewServer(mux)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws?tenant_id=t1&conversation_id=c1&erp_type=MOCK"
	conn := wsDial(t, wsURL)

	// STEP 1: read vendors -> table block.
	chatSend(t, conn, "c1", "show vendors")
	text, blocks := turnTextAndBlocks(readTurn(t, conn))
	if !strings.Contains(text, "Found") {
		t.Errorf("expected vendor text, got %q", text)
	}
	if !blockTypes(blocks)[realtime.BlockTable] {
		t.Errorf("expected table block, got %+v", blocks)
	}

	// STEP 2: RFQ dry-run -> held pending, dry_run + approval blocks.
	chatSend(t, conn, "c1", "create RFQ from ABC for 10 of Chair")
	text, blocks = turnTextAndBlocks(readTurn(t, conn))
	bt := blockTypes(blocks)
	if !bt[realtime.BlockDryRun] || !bt[realtime.BlockApproval] {
		t.Errorf("expected dry_run + approval blocks, got text=%q blocks=%+v", text, blocks)
	}

	// STEP 3: "okay" executes the held action live (no new approval card).
	chatSend(t, conn, "c1", "okay")
	text, blocks = turnTextAndBlocks(readTurn(t, conn))
	if !strings.Contains(strings.ToLower(text), "drafted") {
		t.Errorf("expected live RFQ execution text, got %q", text)
	}
	if bt := blockTypes(blocks); bt[realtime.BlockApproval] || bt[realtime.BlockDryRun] {
		t.Errorf("executed action must not re-issue approval blocks, got %+v", blocks)
	}

	// STEP 4: stray rejection with nothing pending falls through, no crash.
	chatSend(t, conn, "c1", "nope")
	text, _ = turnTextAndBlocks(readTurn(t, conn))
	if !strings.Contains(text, "Could not map") {
		t.Errorf("expected unknown-intent fallthrough, got %q", text)
	}
}
