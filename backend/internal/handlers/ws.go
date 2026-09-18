package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"erpagent-backend/internal/config"
	"erpagent-backend/internal/factory"
	"erpagent-backend/internal/audit"
	"erpagent-backend/internal/connectors"
	"erpagent-backend/internal/realtime"
	"erpagent-backend/middleware"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// Same-origin gateway; tenant auth travels via query params (locked Q6/Q7).
	CheckOrigin: func(r *http.Request) bool { return true },
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
func ChatSocket(cfg *config.Config, store realtime.Store, auditStore audit.Store, connStore connectors.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenantID := r.URL.Query().Get("tenant_id")
		if tenantID == "" {
			tenantID = r.Header.Get("X-Tenant-ID")
		}
		if tenantID == "" {
			if v, _ := r.Context().Value(middleware.TenantIDKey).(string); v != "" {
				tenantID = v
			}
		}
		if tenantID == "" {
			tenantID = "default_tenant"
		}
		conversationID := r.URL.Query().Get("conversation_id")
		if conversationID == "" {
			conversationID = "default"
		}
		erpType := r.URL.Query().Get("erp_type")

		conn, err := wsUpgrader.Upgrade(w, r, nil)
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
			processChatTurn(r.Context(), cfg, store, auditStore, connStore, writer, tenantID, conv, erpType, in.Text)
		}
	}
}

func processChatTurn(ctx context.Context, cfg *config.Config, store realtime.Store, auditStore audit.Store, connStore connectors.Store, writer *wsWriter, tenantID, conversationID, erpType, text string) {
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

	// Conversational approval first: deterministic, no LLM ambiguity on money.
	if isAffirm(text) || isReject(text) {
		if handled := processPendingDecision(ctx, cfg, store, auditStore, connStore, writer, send, tenantID, conversationID, erpType, isAffirm(text)); handled {
			return
		}
		// No pending action: fall through and treat as a normal prompt.
	}

	send(realtime.EventStatus, "Understanding request…", nil)

	parsed, err := callAIEngine(ctx, cfg.AIEngineURL, text, tenantID)
	if err != nil {
		fail(fmt.Sprintf("AI Engine communication error: %v", err))
		return
	}

	// AI-side affirm/reject classification lands on the same pending flow.
	if parsed.IntentType == "AFFIRM" || parsed.IntentType == "REJECT" {
		if handled := processPendingDecision(ctx, cfg, store, auditStore, connStore, writer, send, tenantID, conversationID, erpType, parsed.IntentType == "AFFIRM"); handled {
			return
		}
	}

	connector, err := factory.GetERPConnector(connectors.ResolveTenantConfig(ctx, connStore, tenantID, erpType, factory.TenantConfig{
		BaseURL:  cfg.OdooURL,
		DBName:   cfg.OdooDB,
		Username: cfg.OdooUser,
		Password: cfg.OdooPassword,
	}))
	if err != nil {
		fail(fmt.Sprintf("Failed to initialize ERP Connector: %v", err))
		return
	}

	verb := "Executing…"
	if parsed.ActionType == "READ" {
		verb = "Reading…"
	}
	send(realtime.EventStatus, verb, nil)

	resp, err := executeIntent(ctx, connector, text, parsed)
	if err != nil {
		fail(fmt.Sprintf("ERP action execution failed: %v", err))
		return
	}

	finishChatTurn(ctx, store, auditStore, send, tenantID, conversationID, text, parsed, resp)
}

// processPendingDecision runs the held dry-run (affirm=true, dry_run=false) or
// discards it (affirm=false). Returns false when nothing is pending.
func processPendingDecision(ctx context.Context, cfg *config.Config, store realtime.Store, auditStore audit.Store, connStore connectors.Store, writer *wsWriter, send func(string, string, []realtime.UIBlock), tenantID, conversationID, erpType string, affirm bool) bool {
	pending, err := store.TakePending(ctx, tenantID, conversationID)
	if err != nil || pending == nil {
		return false
	}

	if !affirm {
		_ = store.AppendHistory(ctx, tenantID, conversationID, "assistant", "Discarded the pending action.")
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
		send(realtime.EventError, fmt.Sprintf("ERP action execution failed: %v", err), nil)
		send(realtime.EventDone, "", nil)
		return true
	}

	finishChatTurn(ctx, store, auditStore, send, tenantID, conversationID, pending.Prompt, &parsed, resp)
	return true
}

// finishChatTurn streams text + ui blocks, holds pending approvals in Redis,
// writes the immutable audit row, and publishes lean activity events.
func finishChatTurn(ctx context.Context, store realtime.Store, auditStore audit.Store, send func(string, string, []realtime.UIBlock), tenantID, conversationID, prompt string, parsed *ParsedIntentResponse, resp *ChatResponse) {
	_ = store.AppendHistory(ctx, tenantID, conversationID, "assistant", resp.Message)

	approval := "none"
	target := sniffRecordID(resp.Data)
	if resp.RequiresConfirmation && resp.DryRun != nil {
		approval = "pending"
	} else if resp.DryRun == nil && (resp.Status == "success" || resp.Status == "dry_run_preview") {
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

	if resp.DryRun == nil && (resp.Status == "success" || resp.Status == "dry_run_preview") {
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
