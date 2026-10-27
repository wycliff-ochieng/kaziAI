package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"erpagent-backend/internal/audit"
	"erpagent-backend/internal/auth"
	"erpagent-backend/internal/chathistory"
	"erpagent-backend/internal/config"
	"erpagent-backend/internal/connectors"
	"erpagent-backend/internal/domain"
	"erpagent-backend/internal/factory"
	"erpagent-backend/internal/realtime"
	"erpagent-backend/middleware"
)

func websocketUpgrader(cfg *config.Config) websocket.Upgrader {
	return websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true
			}
			parsed, err := url.Parse(origin)
			if err != nil {
				return false
			}
			if parsed.Host == r.Host {
				return true
			}
			// Local development may run the frontend and gateway on different ports.
			if cfg.Environment != "production" && (parsed.Hostname() == "localhost" || parsed.Hostname() == "127.0.0.1") {
				return true
			}
			return false
		},
	}
}

// Conversational approval matchers (deterministic Go-side mirror of the AI
// AFFIRM/REJECT intents). Full-message match on short inputs only: money
// actions must never trigger on a substring of a longer prompt.
var (
	affirmRe = regexp.MustCompile(`^(ok(ay)?|yes( please)?|confirmed?|go( ahead| on)?|proceed|do it|approved?|looks good|yep|yeah|sure|please proceed)$`)
	rejectRe = regexp.MustCompile(`^(no(pe)?|reject(ed)?|cancel( that| it)?|stop|don't|do not|discard|never mind|not now)$`)
)

func normalizeShortPrompt(s string) string {
	t := strings.ToLower(strings.TrimSpace(s))
	t = strings.Trim(t, ".! ")
	if len(strings.Fields(t)) > 6 {
		return ""
	}
	return t
}

func isAffirm(s string) bool {
	t := normalizeShortPrompt(s)
	return t != "" && affirmRe.MatchString(t)
}

func isReject(s string) bool {
	t := normalizeShortPrompt(s)
	return t != "" && rejectRe.MatchString(t)
}

// wsWriter serializes concurrent socket writes (chat turns + activity pump).
type wsWriter struct {
	mu   sync.Mutex
	conn *websocket.Conn
}

func (w *wsWriter) send(msg realtime.ServerMessage) {
	w.mu.Lock()
	defer w.mu.Unlock()
	_ = w.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	if err := w.conn.WriteJSON(msg); err != nil {
		log.Printf("ws write failed: %v", err)
	}
}

// ChatSocket upgrades to the single multiplexed socket (locked Q7):
// chat channel (staged status -> text -> ui -> done) + tenant-activity channel.
// Tenant/conversation travel via query params (browsers can't set WS headers).
// auditStore and connStore may be nil (writes/resolution fall back safely).
func ChatSocket(cfg *config.Config, store realtime.Store, auditStore audit.Store, connStore connectors.Store, authStore auth.Store, historyStore chathistory.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID, userID, ok := middleware.ResolveTenant(r, authStore, cfg.InternalAPIToken, cfg.Environment)
		if !ok {
			http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
			return
		}
		conversationID := r.URL.Query().Get("conversation_id")
		if conversationID == "" {
			conversationID = "default"
		}
		erpType := r.URL.Query().Get("erp_type")

		upgrader := websocketUpgrader(cfg)
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("ws upgrade failed: %v", err)
			return
		}
		defer conn.Close()

		writer := &wsWriter{conn: conn}

		// Tenant activity fan-out for this connection.
		actCh, unsub, err := store.SubscribeActivity(r.Context(), tenantID)
		if err != nil {
			writer.send(realtime.ServerMessage{Channel: realtime.ChannelTenantActivity, Type: realtime.EventError, Error: err.Error()})
		} else {
			defer unsub()
			go func() {
				for ev := range actCh {
					writer.send(realtime.ServerMessage{
						Channel: realtime.ChannelTenantActivity,
						Type:    realtime.EventActivity,
						Event:   &ev,
					})
				}
			}()
		}

		// Process turns on a worker so the read loop keeps servicing WebSocket
		// pings while a slow LLM/tool turn is in flight. Without this, the
		// client's keepalive times out and the socket drops mid-answer.
		type chatJob struct{ conversationID, text string }
		jobs := make(chan chatJob, 32)
		done := make(chan struct{})
		go func() {
			defer close(done)
			for job := range jobs {
				processChatTurn(r.Context(), cfg, store, auditStore, connStore, historyStore, writer, tenantID, userID, job.conversationID, erpType, job.text)
			}
		}()
		defer func() {
			close(jobs)
			<-done
		}()

		for {
			var in realtime.ClientMessage
			if err := conn.ReadJSON(&in); err != nil {
				return
			}
			if in.Type == realtime.ClientPing {
				writer.send(realtime.ServerMessage{Channel: realtime.ChannelChat, Type: realtime.EventDone, ConversationID: conversationID})
				continue
			}
			if in.Type != realtime.ClientChat {
				continue
			}
			conv := in.ConversationID
			if conv == "" {
				conv = conversationID
			}
			jobs <- chatJob{conversationID: conv, text: in.Text}
		}
	}
}

func processChatTurn(ctx context.Context, cfg *config.Config, store realtime.Store, auditStore audit.Store, connStore connectors.Store, historyStore chathistory.Store, writer *wsWriter, tenantID, userID, conversationID, erpType, text string) {
	send := func(msgType, txt string, blocks []realtime.UIBlock) {
		writer.send(realtime.ServerMessage{
			Channel:        realtime.ChannelChat,
			Type:           msgType,
			ConversationID: conversationID,
			Text:           txt,
			Blocks:         blocks,
		})
	}
	fail := func(detail string) {
		writer.send(realtime.ServerMessage{
			Channel:        realtime.ChannelChat,
			Type:           realtime.EventError,
			ConversationID: conversationID,
			Error:          detail,
		})
		send(realtime.EventDone, "", nil)
	}

	if strings.TrimSpace(text) == "" {
		fail("Prompt cannot be empty")
		return
	}
	_ = store.AppendHistory(ctx, tenantID, conversationID, "user", text)
	if historyStore != nil && userID != "" && userID != "dev" && userID != "service" {
		_ = historyStore.AppendMessage(ctx, tenantID, userID, conversationID, "user", text)
	}

	// Conversational approval first: deterministic, no LLM ambiguity on money.
	if isAffirm(text) || isReject(text) {
		if handled := processPendingDecision(ctx, cfg, store, auditStore, connStore, historyStore, writer, send, tenantID, userID, conversationID, erpType, isAffirm(text)); handled {
			return
		}
		// No pending action: fall through and treat as a normal prompt.
	}

	// One brain: the agent loop executes read tools and returns previews for
	// writes. Go only executes a plan after an explicit approval below.
	threadID := tenantID + ":" + conversationID
	proc, err := callAIProcess(ctx, cfg.AIEngineURL, text, tenantID, threadID, erpType)
	if err != nil {
		fail(fmt.Sprintf("AI Engine process communication error: %v", err))
		return
	}

	resp, parsed := chatResponseFromProcess(proc, text)
	finishChatTurn(ctx, store, auditStore, historyStore, send, tenantID, userID, conversationID, text, parsed, resp)
}

// chatResponseFromProcess converts the agent's typed turn into the gateway's
// chat response. A write plan is always a preview: execution is a separate,
// server-side approval step.
func chatResponseFromProcess(proc *ProcessResponse, prompt string) (*ChatResponse, *ParsedIntentResponse) {
	parsed := &ParsedIntentResponse{
		IntentType: proc.IntentType,
		Confidence: 0.8,
		ActionType: strings.ToUpper(strings.TrimSpace(proc.ActionType)),
		Summary:    proc.Response,
	}
	if parsed.IntentType == "" {
		parsed.IntentType = "CONVERSATION"
	}
	if parsed.ActionType == "" {
		parsed.ActionType = "READ"
	}

	hasPlan := proc.Plan != nil && len(proc.Plan.Steps) > 0
	if hasPlan && proc.RequiresConfirmation {
		primary := proc.Plan.Steps[0]
		parsed.IntentType = primary.IntentType
		parsed.ActionType = "WRITE"
		parsed.Parameters = primary.Parameters
		parsed.Steps = append([]IntentStep(nil), proc.Plan.Steps[1:]...)
		parsed.Validation = ValidationRequirement{
			RequiresUserApproval: true,
			RiskLevel:            "MEDIUM",
			ApprovalPrompt:       proc.ConfirmationPrompt,
			ImpactSummary:        proc.Response,
		}
		for i := range parsed.Steps {
			if parsed.Steps[i].Parameters.OrderID == "" {
				parsed.Steps[i].Parameters.OrderID = primary.Parameters.OrderID
			}
		}

		risk := domain.RiskMedium
		if proc.DryRun != nil && proc.DryRun.RiskLevel != "" {
			risk = proc.DryRun.RiskLevel
		}
		dry := proc.DryRun
		if dry == nil {
			dry = &domain.DryRunResult{
				IsDryRun:             true,
				Operation:            primary.IntentType,
				ImpactSummary:        proc.Response,
				RiskLevel:            risk,
				RequiresConfirmation: true,
				ConfirmationPrompt:   proc.ConfirmationPrompt,
			}
		}
		// Show the thing being proposed, not incidental lookups the agent did
		// to resolve names during the same turn.
		previewData := any(nil)
		if dry.SimulatedOrderDetails != nil {
			previewData = dry.SimulatedOrderDetails
		} else if dry.SimulatedInvoice != nil {
			previewData = dry.SimulatedInvoice
		}

		return &ChatResponse{
			Status:               "dry_run_preview",
			Prompt:               prompt,
			ParsedIntent:         parsed,
			Message:              proc.Response,
			RequiresConfirmation: true,
			ConfirmationPrompt:   proc.ConfirmationPrompt,
			Data:                 previewData,
			DryRun:               dry,
		}, parsed
	}

	return &ChatResponse{
		Status:       "success",
		Prompt:       prompt,
		ParsedIntent: parsed,
		Message:      proc.Response,
		Data:         proc.Data,
	}, parsed
}

// processPendingDecision runs the held dry-run (affirm=true, dry_run=false) or
// discards it (affirm=false). Returns false when nothing is pending.
func processPendingDecision(ctx context.Context, cfg *config.Config, store realtime.Store, auditStore audit.Store, connStore connectors.Store, historyStore chathistory.Store, writer *wsWriter, send func(string, string, []realtime.UIBlock), tenantID, userID, conversationID, erpType string, affirm bool) bool {
	pending, err := store.TakePending(ctx, tenantID, conversationID)
	if err != nil || pending == nil {
		return false
	}

	if !affirm {
		_ = store.AppendHistory(ctx, tenantID, conversationID, "assistant", "Discarded the pending action.")
		if historyStore != nil && userID != "" && userID != "dev" && userID != "service" {
			_ = historyStore.AppendMessage(ctx, tenantID, userID, conversationID, "assistant", "Discarded the pending action.")
		}
		appendAudit(ctx, auditStore, tenantID, conversationID, pending.Prompt, heldIntentType(pending), "", "rejected")
		send(realtime.EventText, "Discarded. Nothing was executed — tell me what you'd like instead.", nil)
		send(realtime.EventDone, "", nil)
		return true
	}

	var parsed ParsedIntentResponse
	if err := json.Unmarshal(pending.Intent, &parsed); err != nil {
		send(realtime.EventError, "Held approval is unreadable; please re-issue the request.", nil)
		send(realtime.EventDone, "", nil)
		return true
	}
	parsed.Parameters.DryRun = false
	for i := range parsed.Steps {
		parsed.Steps[i].Parameters.DryRun = false
	}

	connector, err := factory.GetERPConnector(connectors.ResolveTenantConfig(ctx, connStore, tenantID, erpType, factory.TenantConfig{
		BaseURL:  cfg.OdooURL,
		DBName:   cfg.OdooDB,
		Username: cfg.OdooUser,
		Password: cfg.OdooPassword,
	}))
	if err != nil {
		send(realtime.EventError, fmt.Sprintf("Failed to initialize ERP Connector: %v", err), nil)
		send(realtime.EventDone, "", nil)
		return true
	}

	send(realtime.EventStatus, "Confirmed — executing…", nil)
	resp, err := executeIntent(ctx, connector, pending.Prompt, &parsed)
	if err != nil {
		appendAudit(ctx, auditStore, tenantID, conversationID, pending.Prompt, heldIntentType(pending), "", "failed")
		send(realtime.EventError, fmt.Sprintf("ERP action execution failed: %v", err), nil)
		send(realtime.EventDone, "", nil)
		return true
	}

	finishChatTurn(ctx, store, auditStore, historyStore, send, tenantID, userID, conversationID, pending.Prompt, &parsed, resp)
	return true
}

// finishChatTurn streams text + ui blocks, holds pending approvals in Redis,
// writes the immutable audit row, and publishes lean activity events.
func finishChatTurn(ctx context.Context, store realtime.Store, auditStore audit.Store, historyStore chathistory.Store, send func(string, string, []realtime.UIBlock), tenantID, userID, conversationID, prompt string, parsed *ParsedIntentResponse, resp *ChatResponse) {
	_ = store.AppendHistory(ctx, tenantID, conversationID, "assistant", resp.Message)
	if historyStore != nil && userID != "" && userID != "dev" && userID != "service" {
		_ = historyStore.AppendMessage(ctx, tenantID, userID, conversationID, "assistant", resp.Message)
	}

	approval := "none"
	target := sniffRecordID(resp.Data)
	// Dialogue-only turns are not ERP executions and must not be audited as one.
	conversational := conversationalIntents[parsed.IntentType]
	if resp.RequiresConfirmation && resp.DryRun != nil {
		approval = "pending"
	} else if !conversational && resp.DryRun == nil && (resp.Status == "success" || resp.Status == "dry_run_preview") {
		approval = "executed"
	}
	appendAudit(ctx, auditStore, tenantID, conversationID, prompt, parsed.IntentType, target, approval)

	if resp.RequiresConfirmation && resp.DryRun != nil {
		intentRaw, _ := json.Marshal(parsed)
		_ = store.SavePending(ctx, realtime.PendingAction{
			TenantID:       tenantID,
			ConversationID: conversationID,
			Prompt:         prompt,
			Intent:         intentRaw,
		})
		_ = store.PublishActivity(ctx, realtime.ActivityEvent{
			Kind:     "approval_pending",
			Title:    parsed.Summary,
			Detail:   resp.ConfirmationPrompt,
			TenantID: tenantID,
		})
	}

	send(realtime.EventText, resp.Message, nil)
	if blocks := uiBlocksForChat(resp); len(blocks) > 0 {
		send(realtime.EventUI, "", blocks)
	}

	if !conversational && resp.DryRun == nil && (resp.Status == "success" || resp.Status == "dry_run_preview") {
		if recID := sniffRecordID(resp.Data); recID != "" || parsed.ActionType == "WRITE" {
			_ = store.PublishActivity(ctx, realtime.ActivityEvent{
				Kind:     "executed_" + strings.ToLower(parsed.IntentType),
				Title:    resp.Message,
				RecordID: recID,
				TenantID: tenantID,
			})
		}
	}

	send(realtime.EventDone, "", nil)
}

// appendAudit writes one immutable row; nil store skips silently (dev/test).
func appendAudit(ctx context.Context, auditStore audit.Store, tenantID, conversationID, prompt, intent, target, approval string) {
	if auditStore == nil {
		return
	}
	_, _ = auditStore.Append(ctx, audit.Event{
		TenantID: tenantID,
		Actor:    "ws-user:" + conversationID,
		Channel:  "ws-chat",
		Prompt:   prompt,
		Intent:   intent,
		Target:   target,
		Approval: approval,
	})
}

// heldIntentType recovers the intent name from a held pending action.
func heldIntentType(pending *realtime.PendingAction) string {
	if pending == nil {
		return "UNKNOWN"
	}
	var decoded struct {
		IntentType string `json:"intent_type"`
	}
	if err := json.Unmarshal(pending.Intent, &decoded); err != nil {
		return "UNKNOWN"
	}
	if decoded.IntentType == "" {
		return "UNKNOWN"
	}
	return decoded.IntentType
}

// uiBlocksForChat derives hand-built registry blocks (locked Q1) from a ChatResponse.
func uiBlocksForChat(resp *ChatResponse) []realtime.UIBlock {
	if resp == nil {
		return nil
	}
	var blocks []realtime.UIBlock

	if rows, single, ok := tabularData(resp.Data); ok {
		if single != nil {
			blocks = append(blocks, realtime.UIBlock{Type: realtime.BlockTable, Props: map[string]any{"title": "", "rows": rows}})
			if rec := sniffRecord(single); rec != nil {
				blocks = append(blocks, realtime.UIBlock{Type: realtime.BlockCitation, Props: rec})
			}
		} else {
			blocks = append(blocks, realtime.UIBlock{Type: realtime.BlockTable, Props: map[string]any{"title": "", "rows": rows}})
		}
	} else if rec := sniffRecord(resp.Data); rec != nil {
		blocks = append(blocks, realtime.UIBlock{Type: realtime.BlockCitation, Props: rec})
	}

	if resp.DryRun != nil {
		blocks = append(blocks, realtime.UIBlock{
			Type: realtime.BlockDryRun,
			Props: map[string]any{
				"operation":           resp.DryRun.Operation,
				"impact_summary":      resp.DryRun.ImpactSummary,
				"risk_level":          string(resp.DryRun.RiskLevel),
				"confirmation_prompt": resp.DryRun.ConfirmationPrompt,
				"confirmation_token":  resp.DryRun.ConfirmationToken,
			},
		})
	}

	// Approval affordance only when a dry-run is genuinely pending.
	if resp.DryRun != nil && resp.RequiresConfirmation && resp.ConfirmationPrompt != "" {
		blocks = append(blocks, realtime.UIBlock{
			Type:  realtime.BlockApproval,
			Props: map[string]any{"confirmation_prompt": resp.ConfirmationPrompt},
		})
	}

	if resp.PrintResult != nil {
		blocks = append(blocks, realtime.UIBlock{
			Type:  realtime.BlockCitation,
			Props: map[string]any{"label": resp.PrintResult.DocumentNo, "url": resp.PrintResult.DownloadURL},
		})
	}

	if resp.ETIMSResult != nil {
		blocks = append(blocks, realtime.UIBlock{
			Type: realtime.BlockCitation,
			Props: map[string]any{
				"label": resp.ETIMSResult.InvoiceNo,
				"irn":   resp.ETIMSResult.IRN,
				"cu":    resp.ETIMSResult.CUSerialNo,
				"url":   resp.ETIMSResult.QRCodeURL,
			},
		})
	}

	return blocks
}

// tabularData reports whether Data is a slice of row objects (table) or a
// single row object (table + citation).
func tabularData(data any) (rows []any, single map[string]any, ok bool) {
	if data == nil {
		return nil, nil, false
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, nil, false
	}
	var arr []any
	if err := json.Unmarshal(raw, &arr); err == nil && len(arr) > 0 {
		return arr, nil, true
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err == nil && len(obj) > 0 {
		return []any{obj}, obj, true
	}
	return nil, nil, false
}

// sniffRecord extracts an ERP citation from a single record object.
func sniffRecord(data any) map[string]any {
	_, single, ok := tabularData(data)
	if !ok || single == nil {
		return nil
	}
	id, _ := single["id"].(string)
	for _, key := range []string{"order_no", "invoice_no", "bill_no", "receipt_no", "delivery_no", "name", "invoiceNo"} {
		if v, _ := single[key].(string); v != "" {
			return map[string]any{"id": id, "record": v}
		}
	}
	return nil
}

// sniffRecordID returns just the record number for activity events.
func sniffRecordID(data any) string {
	if rec := sniffRecord(data); rec != nil {
		if v, _ := rec["record"].(string); v != "" {
			return v
		}
	}
	return ""
}
