// Multiplexed gateway socket client (mirrors backend/internal/realtime/protocol.go).
// One connection carries chat staged events + tenant activity pushes.

export type UIBlock = { type: string; props: Record<string, any> };

export type ActivityEvent = {
  kind: string;
  title: string;
  detail?: string;
  record_id?: string;
  tenant_id: string;
  timestamp: number;
};

export type ServerMessage = {
  channel: "chat" | "tenant-activity";
  type: "status" | "text" | "ui" | "done" | "error" | "activity";
  conversation_id?: string;
  text?: string;
  blocks?: UIBlock[];
  event?: ActivityEvent;
  error?: string;
};

export type ChatTurn = {
  id: number;
  role: "user" | "assistant";
  text: string;
  blocks: UIBlock[];
  status?: string;
  pending: boolean;
  error?: string;
};

export function gatewayWsUrl(): string {
  const configured = process.env.NEXT_PUBLIC_GATEWAY_WS;
  if (configured) return configured;
  if (typeof window !== "undefined") {
    // `next dev` serves the UI on :3000 while the gateway listens on :3001.
    if (
      (window.location.hostname === "localhost" || window.location.hostname === "127.0.0.1") &&
      window.location.port === "3000"
    ) {
      return "ws://localhost:3001/api/v1/ws";
    }
    const protocol = window.location.protocol === "https:" ? "wss:" : "ws:";
    return `${protocol}//${window.location.host}/api/v1/ws`;
  }
  return "ws://localhost:3001/api/v1/ws";
}

export type SocketHandle = {
  close: () => void;
  send: (conversationId: string, text: string) => void;
};

export function openChatSocket(opts: {
  tenantId: string;
  conversationId: string;
  erpType?: string;
  onMessage: (msg: ServerMessage) => void;
  onOpen?: () => void;
  onClose?: () => void;
}): SocketHandle {
  const params = new URLSearchParams({
    tenant_id: opts.tenantId,
    conversation_id: opts.conversationId,
  });
  if (opts.erpType) params.set("erp_type", opts.erpType);

  let ws: WebSocket | null = null;
  let closed = false;
  let retries = 0;

  const connect = () => {
    if (closed) return;
    ws = new WebSocket(`${gatewayWsUrl()}?${params.toString()}`);
    ws.onopen = () => {
      retries = 0;
      opts.onOpen?.();
    };
    ws.onmessage = (ev) => {
      try {
        opts.onMessage(JSON.parse(ev.data) as ServerMessage);
      } catch {
        // ignore malformed frames
      }
    };
    ws.onclose = () => {
      opts.onClose?.();
      if (closed) return;
      // brief backoff reconnect; state (pending/history) lives server-side in Redis
      const delay = Math.min(1000 * 2 ** retries, 10000);
      retries += 1;
      setTimeout(connect, delay);
    };
  };
  connect();

  return {
    close: () => {
      closed = true;
      ws?.close();
    },
    send: (conversationId: string, text: string) => {
      ws?.send(JSON.stringify({ type: "chat", conversation_id: conversationId, text }));
    },
  };
}

export function sendChat(ws: WebSocket | null, conversationId: string, text: string) {
  ws?.send(JSON.stringify({ type: "chat", conversation_id: conversationId, text }));
}
