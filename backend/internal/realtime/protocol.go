package realtime

// Multiplexed WebSocket protocol (locked Q7).
//
// One socket per session carries two channels:
//   - "chat": live staged events for the conversation (status -> text -> ui -> done).
//   - "tenant-activity": pushed tenant business events (approvals, postings, syncs).
//
// Payloads stay lean: text deltas carry only new text, ui blocks are complete
// component specs (never partial), activity events carry only what the feed needs.

const (
	ChannelChat           = "chat"
	ChannelTenantActivity = "tenant-activity"
)

// Client -> server message types.
const (
	ClientChat = "chat" // {conversation_id, text}
	ClientPing = "ping"
)

// Server -> client event types (chat channel).
const (
	EventStatus = "status" // lean progress note, e.g. "Resolving vendor..."
	EventText   = "text"   // message text (full, not deltas, in v1)
	EventUI     = "ui"     // complete registry blocks for this turn
	EventDone   = "done"   // turn complete
	EventError  = "error"
)

// Server -> client event types (tenant-activity channel).
const (
	EventActivity = "activity"
)

// Registry block types rendered by the hand-built frontend renderer (locked Q1).
const (
	BlockText     = "text"
	BlockTable    = "table"
	BlockChart    = "chart"
	BlockDryRun   = "dry_run"
	BlockCitation = "citation"
	BlockApproval = "approval"
)

// UIBlock is a complete component spec: {type, props}. The frontend registry
// renders it inline in the message stream.
type UIBlock struct {
	Type  string `json:"type"`
	Props any    `json:"props"`
}

// ClientMessage is an inbound frame from the browser.
type ClientMessage struct {
	Type           string `json:"type"`
	ConversationID string `json:"conversation_id"`
	Text           string `json:"text"`
}

// ServerMessage is an outbound frame to the browser.
type ServerMessage struct {
	Channel        string    `json:"channel"`
	Type           string    `json:"type"`
	ConversationID string    `json:"conversation_id,omitempty"`
	Text           string    `json:"text,omitempty"`
	Blocks         []UIBlock `json:"blocks,omitempty"`
	Event          *ActivityEvent `json:"event,omitempty"`
	Error          string    `json:"error,omitempty"`
}

// ActivityEvent is a lean tenant business event for the feed.
type ActivityEvent struct {
	Kind      string `json:"kind"` // "approval_pending", "order_confirmed", "bill_posted", "sync", ...
	Title     string `json:"title"`
	Detail    string `json:"detail,omitempty"`
	RecordID  string `json:"record_id,omitempty"`
	TenantID  string `json:"tenant_id"`
	Timestamp int64  `json:"timestamp"`
}
