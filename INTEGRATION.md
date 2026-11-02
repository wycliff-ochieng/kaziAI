# Kazi AI (ERPAgent) Integration

Architecture, data flows, canonical models, API contracts, and operational boundaries for integrators.

## 1. Architecture Overview

```text
                         ┌──────────────────────────────┐
                         │          Browser             │
                         │  Next.js UI + WebSocket      │
                         └──────────────┬───────────────┘
                                        │ HTTPS / WSS
                                        ▼
                         ┌──────────────────────────────┐
                         │            Caddy             │
                         │  TLS + reverse proxy         │
                         └───────┬──────────────┬───────┘
                                 │              │
                      /api/*     │              │  /*
                                 ▼              ▼
                    ┌──────────────────┐  ┌──────────────────┐
                    │   Go Gateway     │  │   Next.js        │
                    │ auth, approval,  │  │   frontend       │
                    │ ERP, audit       │  │   :3000          │
                    └───┬──────┬───┬───┘  └──────────────────┘
                        │      │   │
             ┌──────────┘      │   └────────────┐
             ▼                 ▼                ▼
      ┌────────────┐   ┌────────────┐   ┌────────────┐
      │ PostgreSQL │   │   Redis    │   │ AI Engine  │
      │ durable    │   │ pending,   │   │ FastAPI +  │
      │ records    │   │ history,   │   │ LangGraph  │
      │ + audit    │   │ activity   │   │ :8000      │
      └────────────┘   └────────────┘   └─────┬──────┘
                                              │
                                    ┌─────────┴─────────┐
                                    ▼                   ▼
                             ┌────────────┐      ┌────────────┐
                             │ LLM API    │      │ Go Gateway │
                             │ OpenAI-    │      │ tool calls │
                             │ compatible │      │ via HTTP   │
                             └────────────┘      └─────┬──────┘
                                                       │
                                                       ▼
                                                ┌────────────┐
                                                │   Odoo     │
                                                │  /jsonrpc  │
                                                └────────────┘
```

Numbered flow:

1. The browser loads the Next.js app over HTTPS. Caddy proxies `/api/*` to the Go gateway.
2. The gateway authenticates the session cookie and resolves `tenant_id` and `user_id`.
3. The frontend opens one WebSocket at `/api/v1/ws` for chat and tenant activity.
4. Chat prompts go to the AI engine's `/api/v1/process` endpoint.
5. The AI engine calls backend read tools over HTTP with `X-Internal-Token` and `X-Tenant-ID`.
6. The gateway resolves the tenant connector and reads or writes the ERP.
7. The AI engine returns a typed response and optional dry-run plan.
8. The gateway holds writes in Redis, streams the preview, and waits for approval.
9. Approved intents are executed by the gateway, audited in Postgres, and published as activity.
10. The AI engine pulls the runtime LLM configuration from the gateway at boot.

## 2. Data Flow

### 2.1 Login and Session Establishment

```text
Browser                 Go Gateway                 Postgres
   │                         │                        │
   ├── POST /auth/login ────►│                        │
   │                         ├── GetAccountByEmail ──►│
   │                         │◄── account + hash ─────┤
   │                         ├── verify password      │
   │                         ├── CreateSession ──────►│
   │                         │◄── token hash ─────────┤
   │◄── Set-Cookie ──────────┤                        │
   │    kazi_session         │                        │
```

1. The browser posts email and password.
2. The gateway normalizes the email and loads the account.
3. The password hash is verified with `golang.org/x/crypto`.
4. A random 32-byte token is created. Only its SHA-256 hash is stored in `sessions`.
5. The raw token is set in an HttpOnly, `SameSite=Lax` cookie with a 7-day lifetime.
6. Later requests send the cookie automatically; `middleware.Auth` resolves the user and tenant.

Request:

```json
{
  "email": "ops@example.com",
  "password": "test-password"
}
```

Response:

```json
{
  "status": "success",
  "user": {
    "id": "usr_...",
    "tenant_id": "tenant_...",
    "email": "ops@example.com",
    "name": "Ops User"
  }
}
```

### 2.2 Read Turn

```text
Browser          Gateway              AI Engine            Backend Tool         ERP
  │                │                     │                     │                │
  ├─ WS chat ─────►│                     │                     │                │
  │                ├─ append history     │                     │                │
  │                ├─ POST /process ────►│                     │                │
  │                │                     ├─ tool call ────────►│                │
  │                │                     │                     ├─ read ────────►│
  │                │                     │                     │◄── rows ────────┤
  │                │                     │◄── tool result ─────┤                │
  │                │◄── typed response ──┤                     │                │
  │◄─ status ──────┤                     │                     │                │
  │◄─ text ────────┤                     │                     │                │
  │◄─ ui table ────┤                     │                     │                │
  │◄─ done ────────┤                     │                     │                │
```

1. The browser sends `{"type":"chat","conversation_id":"c1","text":"Show customers"}`.
2. The gateway appends the user message to Redis and Postgres.
3. The gateway calls `POST {AI_ENGINE_URL}/api/v1/process` with `prompt`, `tenant_id`, `thread_id`, and
   `erp_type`.
4. The AI engine runs the LangGraph agent. Read tools call gateway REST endpoints.
5. The gateway resolves the tenant connector and queries the ERP.
6. The AI engine builds a turn plan from the tool calls and results.
7. The gateway streams `status`, `text`, `ui`, and `done` frames.
8. A read turn is audited with `approval="none"`.

Gateway to AI request:

```json
{
  "prompt": "Show customers",
  "tenant_id": "tenant_...",
  "thread_id": "tenant_...:c1",
  "erp_type": "MOCK"
}
```

AI to gateway tool request:

```http
GET /api/v1/sales/customers?search=Azure&limit=20
X-Internal-Token: <shared-secret>
X-Tenant-ID: tenant_...
```

### 2.3 Write Preview and Approval

```text
Browser            Gateway                 Redis              AI Engine          ERP
  │                  │                      │                   │                │
  ├─ "Create RFQ" ──►│                      │                   │                │
  │                  ├─ POST /process ─────────────────────────►│                │
  │                  │                      │                   ├─ dry-run tool ─►│
  │                  │                      │                   │◄─ preview ─────┤
  │                  │◄─ plan + dry_run ────────────────────────┤                │
  │                  ├─ SavePending ───────►│                   │                │
  │◄─ dry_run card ──┤                      │                   │                │
  │                  │                      │                   │                │
  ├─ "confirmed" ───►│                      │                   │                │
  │                  ├─ TakePending ───────►│                   │                │
  │                  │◄─ intent ────────────┤                   │                │
  │                  ├─ executeIntent ──────────────────────────────────────────►│
  │                  │◄─ result ────────────────────────────────────────────────┤
  │◄─ result + done ─┤                      │                   │                │
```

1. The AI engine's write tools always send `dry_run: true`.
2. The AI returns `requires_confirmation: true`, a `dry_run` object, and a `plan`.
3. The gateway stores the parsed intent in `erp:pending:{tenant}:{conversation}` with a 30-minute TTL.
4. The gateway streams `dry_run` and `approval` UI blocks.
5. The user replies with a short approval phrase.
6. The gateway atomically reads and deletes the pending action with Redis `GETDEL`.
7. The gateway sets `dry_run=false` and executes the primary intent, then ordered follow-on steps.
8. The result is audited and published as `executed_<intent>`.

Held intent shape:

```json
{
  "intent_type": "CREATE_RFQ",
  "action_type": "WRITE",
  "parameters": {
    "vendor_name": "ABC",
    "product_name": "Chair",
    "quantity": 10,
    "dry_run": false
  },
  "validation": {
    "requires_user_approval": true,
    "risk_level": "LOW",
    "approval_prompt": "Draft RFQ?"
  }
}
```

### 2.4 AI Tool Calls

```text
AI Engine                      Go Gateway                     Odoo
   │                              │                            │
   ├─ GET /sales/customers ──────►│                            │
   │  X-Internal-Token            ├─ ResolveTenantConfig       │
   │  X-Tenant-ID                 ├─ GetERPConnector           │
   │                              ├─ search_read ─────────────►│
   │                              │◄── records ────────────────┤
   │◄── {"data":[...]} ───────────┤                            │
```

1. `BackendClient` adds `X-Internal-Token` when configured.
2. It adds `X-Tenant-ID` from the per-request context variable.
3. It appends `erp_type` to query parameters when supplied.
4. The gateway's `middleware.Auth` accepts the internal token and uses `X-Tenant-ID`.
5. Tool output is capped at 20 rows per list before it reaches the model.

### 2.5 Runtime LLM Configuration

```text
AI Engine                         Go Gateway                    Postgres
   │                                  │                            │
   ├─ GET /internal/llm-config ──────►│                            │
   │  X-Internal-Token                ├─ Get llm_settings ────────►│
   │                                  │◄── encrypted key ──────────┤
   │                                  ├─ decrypt AES-GCM           │
   │◄── provider, base_url, model ────┤                            │
   │    api_key, max_tokens           │                            │
```

1. The AI engine pulls the config at startup and retries up to eight times with a 2-second delay.
2. The gateway returns the plaintext key only to the internal service.
3. A settings save from the admin UI pushes `POST /api/v1/admin/llm` to the AI engine.
4. The AI engine updates its runtime client without a restart.
5. A failed push does not roll back the saved config; the API returns a warning.

### 2.6 Audit Chain

```text
Turn completes
  │
  ├── Build Event{tenant, actor, channel, prompt, intent, target, approval}
  ├── SELECT hash FROM audit_events WHERE tenant = ? ORDER BY id DESC LIMIT 1 FOR UPDATE
  ├── prev = previous hash or ""
  ├── canonical = JSON(Event without hash)
  ├── hash = SHA-256(prev + canonical)
  ├── INSERT audit_events(prev_hash, hash, ...)
  └── Publish activity if approval is pending or executed
```

1. The audit store locks the tenant's last row before computing the next hash.
2. Events are append-only through the store interface.
3. `/audit/verify` walks the tenant chain and recomputes every hash.
4. `/audit/export` returns the same rows as CSV.

## 3. Model Blueprints

The canonical models are Go structs in `backend/internal/domain`. They are the contract between the
gateway, adapters, and AI tools. There is no Odoo ORM dependency in this repository.

### 3.1 Customer

| Field | Type | Notes |
|---|---|---|
| `id` | string | ERP record ID. |
| `name` | string | Company or person. |
| `email` | string | Contact email. |
| `phone` | string | Contact phone. |
| `credit_limit` | float64 | Approved credit. |
| `balance` | float64 | Outstanding balance. |
| `street`, `city`, `country` | string | Address. |

Relationships: a customer has many sale orders and invoices. `SaleOrder.CustomerID` and
`Invoice.CustomerID` reference this model.

### 3.2 Product

| Field | Type | Notes |
|---|---|---|
| `id` | string | ERP record ID. |
| `name` | string | Product name. |
| `sku` | string | Stock keeping unit. |
| `list_price` | float64 | Sale price. |
| `standard_price` | float64 | Cost. |
| `qty_available` | float64 | On-hand quantity. |
| `unit_of_measure` | string | UoM. |
| `category` | string | Category. |

Relationships: products appear in `SaleOrderLine` and `PurchaseOrderLine` through `ProductID`.

### 3.3 SaleOrder

| Field | Type | Notes |
|---|---|---|
| `id` | string | ERP record ID. |
| `order_no` | string | Human-readable number. |
| `customer_id`, `customer_name` | string | Customer reference. |
| `date_order` | time.Time | Order date. |
| `amount_untaxed`, `amount_tax`, `amount_total` | float64 | Totals. |
| `state` | string | `draft`, `sent`, `sale`, `done`, `cancel`. |
| `lines` | []SaleOrderLine | Product lines. |
| `delivery_state` | string | `pending`, `partially_delivered`, `fully_delivered`. |
| `invoice_state` | string | `no`, `to_invoice`, `invoiced`. |

Relationships: a sale order belongs to one customer, has many lines, can produce invoices, and can have
follow-on confirm, deliver, and invoice actions.

### 3.4 Invoice

| Field | Type | Notes |
|---|---|---|
| `id`, `invoice_no` | string | ERP ID and number. |
| `customer_id`, `customer_name` | string | Customer reference. |
| `order_id` | string | Source sale order. |
| `move_type` | string | `out_invoice` or `out_refund`. |
| `amount_total`, `amount_due` | float64 | Totals. |
| `status` | string | `draft`, `posted`, `paid`, `cancel`. |
| `payment_state` | string | `not_paid`, `partial`, `paid`, `in_payment`. |
| `due_date` | time.Time | Due date. |
| `etims_status` | string | `pending`, `fiscalized`, `failed`. |
| `cu_serial_no`, `irn`, `qr_code_url` | string | KRA eTIMS identifiers. |

Relationships: an invoice belongs to a customer and optionally a sale order. Credit notes use
`move_type="out_refund"`.

### 3.5 Vendor and PurchaseOrder

| Model | Fields |
|---|---|
| `Vendor` | `id`, `name`, `email`, `phone`, `street`, `city`, `country`, `balance`. |
| `PurchaseOrder` | `id`, `order_no`, `vendor_id`, `vendor_name`, `date_order`, totals, `state`, `lines`, `receipt_state`, `billing_state`. |

Relationships: a purchase order belongs to one vendor, has many lines, and can produce a vendor bill.
`state` values are `draft` (RFQ), `sent`, `purchase` (confirmed), `done` (received), and `cancel`.

### 3.6 Persistence Relationships

```text
users 1 ──── * sessions
users 1 ──── * chat_conversations
chat_conversations 1 ──── * chat_messages
tenant_connectors: (tenant_id, name) unique
audit_events: tenant-scoped hash chain
llm_settings: singleton row
```

| Table | Relationship or key |
|---|---|
| `users` | `tenant_id` is unique; each registration currently creates one tenant. |
| `sessions` | `user_id` references `users.id`; `token_hash` is unique. |
| `chat_conversations` | Unique on `(tenant_id, id)`; scoped to `user_id`. |
| `chat_messages` | Belongs to `(tenant_id, conversation_id)`; ordered by `id`. |
| `audit_events` | Tenant-scoped chain; `prev_hash` points to the previous row. |
| `tenant_connectors` | Primary key `(tenant_id, name)`. |
| `llm_settings` | Single row enforced by a boolean primary key. |

## 4. API Contract

### 4.1 Endpoint Summary

| Area | Endpoints |
|---|---|
| Health | `GET /api/v1/health` |
| Auth | `POST /auth/register`, `POST /auth/login`, `POST /auth/logout`, `GET /auth/me`, `PATCH /auth/profile`, `POST /auth/password` |
| Chats | `GET /chats`, `GET /chats/{id}`, `DELETE /chats/{id}` |
| AI | `POST /chat`, `POST /intents`, `GET /ws` |
| Audit | `GET /audit`, `GET /audit/export`, `GET /audit/verify` |
| Connectors | `GET /connectors`, `POST /connectors`, `POST /connectors/test`, `DELETE /connectors/{name}`, `POST /connectors/{name}/test` |
| LLM settings | `GET /settings/llm`, `POST /settings/llm`, `POST /settings/llm/test`, `GET /internal/llm-config` |
| Sales | `/sales/customers`, `/sales/products`, `/sales/orders`, `/sales/invoices` |
| Purchase | `/purchase/vendors`, `/purchase/orders`, `/purchase/bills` |

### 4.2 Chat Request

| Field | Type | Required | Description |
|---|---|---|---|
| `prompt` | string | Yes | Natural-language request. |
| `tenant_id` | string | No | Ignored when session auth supplies a tenant. |
| `conversation_id` | string | No | Defaults to `default`. |
| `erp_type` | string | No | `ODOO` or `MOCK`. |

```json
{
  "prompt": "Show overdue invoices",
  "conversation_id": "c1",
  "erp_type": "MOCK"
}
```

### 4.3 AI Process Response

| Field | Type | Description |
|---|---|---|
| `status` | string | `success`. |
| `prompt` | string | Original prompt. |
| `response` | string | Assistant text. |
| `thread_id` | string | `tenant:conversation` or the supplied thread ID. |
| `intent_type` | string | Canonical intent. |
| `action_type` | string | `READ`, `WRITE`, `SIMULATION`, or `CONVERSATION`. |
| `data` | any | Rows or a single record. |
| `count` | integer | Row count. |
| `requires_confirmation` | boolean | True for write previews. |
| `confirmation_prompt` | string | Human approval question. |
| `plan` | object | `{"steps":[...]}` for write plans. |
| `dry_run` | object | Preview details. |
| `tool_calls` | array | Executed tool calls. |
| `tool_results` | array | Tool outputs. |

```json
{
  "status": "success",
  "prompt": "Create RFQ from ABC for 10 of Chair",
  "response": "I prepared an RFQ preview.",
  "thread_id": "tenant_...:c1",
  "intent_type": "CREATE_RFQ",
  "action_type": "WRITE",
  "requires_confirmation": true,
  "confirmation_prompt": "Draft this RFQ?",
  "plan": {
    "steps": [
      {
        "intent_type": "CREATE_RFQ",
        "parameters": {
          "vendor_name": "ABC",
          "product_name": "Chair",
          "quantity": 10,
          "dry_run": false
        }
      }
    ]
  },
  "dry_run": {
    "is_dry_run": true,
    "operation": "CREATE_RFQ",
    "impact_summary": "Draft RFQ for 10 Chair from ABC.",
    "risk_level": "LOW",
    "requires_confirmation": true,
    "confirmation_prompt": "Draft this RFQ?"
  }
}
```

### 4.4 Gateway Chat Response

| Field | Type | Description |
|---|---|---|
| `status` | string | `success` or `dry_run_preview`. |
| `prompt` | string | Original prompt. |
| `parsed_intent` | object | Intent, action, parameters, validation, steps. |
| `message` | string | Assistant text. |
| `requires_confirmation` | boolean | True for previews. |
| `confirmation_prompt` | string | Approval question. |
| `data` | any | Result rows or preview. |
| `dry_run` | object | Present for previews. |

```json
{
  "status": "dry_run_preview",
  "prompt": "Create RFQ from ABC for 10 of Chair",
  "parsed_intent": {
    "intent_type": "CREATE_RFQ",
    "action_type": "WRITE",
    "summary": "I prepared an RFQ preview."
  },
  "message": "I prepared an RFQ preview.",
  "requires_confirmation": true,
  "confirmation_prompt": "Draft this RFQ?",
  "dry_run": {
    "is_dry_run": true,
    "operation": "CREATE_RFQ",
    "risk_level": "LOW"
  }
}
```

### 4.5 WebSocket Frames

Client to server:

| Field | Type | Required | Description |
|---|---|---|---|
| `type` | string | Yes | `chat` or `ping`. |
| `conversation_id` | string | No | Conversation key. |
| `text` | string | Chat | Prompt text. |

```json
{
  "type": "chat",
  "conversation_id": "c1",
  "text": "Show vendors"
}
```

Server to client:

| Field | Type | Description |
|---|---|---|
| `channel` | string | `chat` or `tenant-activity`. |
| `type` | string | `status`, `text`, `ui`, `done`, `error`, or `activity`. |
| `conversation_id` | string | Present on chat frames. |
| `text` | string | Text payload. |
| `blocks` | array | UI blocks `{type, props}`. |
| `event` | object | Activity event. |
| `error` | string | Error detail. |

```json
{"channel":"chat","type":"text","conversation_id":"c1","text":"Found 3 vendors."}
{"channel":"chat","type":"ui","conversation_id":"c1","blocks":[{"type":"table","props":{}}]}
{"channel":"tenant-activity","type":"activity","event":{"kind":"approval_pending","title":"RFQ"}}
```

### 4.6 Connector Request

| Field | Type | Required | Description |
|---|---|---|---|
| `name` | string | No | Defaults to `default`. |
| `erp_type` | string | Yes | `MOCK`, `ODOO`, `SAP`, `DYNAMICS`. |
| `base_url` | string | Odoo | Odoo base URL. |
| `db_name` | string | Odoo | Odoo database. |
| `username` | string | Odoo | Odoo user. |
| `password` | string | Odoo | Password or API key. |

```json
{
  "name": "default",
  "erp_type": "ODOO",
  "base_url": "http://10.0.0.10:8069",
  "db_name": "erp",
  "username": "api@example.com",
  "password": "replace-me"
}
```

### 4.7 LLM Settings Contract

| Field | Type | Description |
|---|---|---|
| `provider` | string | Provider label. |
| `base_url` | string | Absolute OpenAI-compatible URL. |
| `model` | string | Model name. |
| `api_key` | string | Write-only; empty keeps the saved key. |
| `max_tokens` | integer | `1..8192`. |
| `has_api_key` | boolean | Read-only redaction flag. |
| `updated_at` | timestamp | Last save time. |

```json
{
  "provider": "groq",
  "base_url": "https://api.groq.com/openai/v1",
  "model": "llama3-70b-8192",
  "max_tokens": 400
}
```

## 5. Security & Network Considerations

- Caddy is the only public entry point. Postgres, Redis, the gateway, and the AI engine stay on `erp_net`.
- The gateway authenticates browsers with an HttpOnly `kazi_session` cookie and sets `Secure` in production.
- Internal tool calls use a shared `X-Internal-Token`; compare with constant-time equality.
- The AI engine never receives ERP credentials. It calls the gateway, which resolves the tenant connector.
- Tenant identity is derived from the session, not from a browser-supplied `X-Tenant-ID`, in production.
- Connector passwords are redacted from responses and stored in Postgres in v1. Move them to Vault before
  multi-customer production.
- LLM API keys are encrypted with AES-GCM using `SETTINGS_ENCRYPTION_KEY`; reads expose only
  `has_api_key`.
- WebSocket origins are checked against the request host. Localhost is allowed only outside production.
- CORS allows same-host origins and, outside production, localhost.
- Audit rows are hash-chained. `/audit/verify` detects modified or deleted rows.
- Rotate any ERP credentials that were committed to repository history before public release.
- Use TLS for every public and internal hop that crosses a host boundary. Odoo should be reachable only
  from the gateway network.

## 6. Fault Tolerance & Edge Cases

| Condition | Behavior |
|---|---|
| AI engine timeout | Gateway uses a 180-second client timeout and returns a communication error. |
| AI engine `413` or rate limit | AI engine maps the provider error to HTTP `429` with a token-budget hint. |
| Backend tool network error | `BackendClient` retries 3 times with exponential backoff. |
| Backend tool HTTP `4xx` | The JSON error payload is returned to the agent instead of raising. |
| Tool result too large | List payloads are capped at 20 rows with a `*_truncated` marker. |
| Prompt too long | `/process` truncates to 2000 characters before the model call. |
| Duplicate approval | Redis `GETDEL` consumes the pending action; the second reply is a normal prompt. |
| Stale pending action | Redis pending keys expire after 30 minutes. |
| Gateway restart | Redis restores pending actions and history; Postgres restores durable records. |
| Postgres outage in production | Gateway refuses to start; no silent in-memory fallback. |
| Redis outage in production | Gateway refuses to start; no silent in-memory chat state. |
| Odoo missing field | `resilientSearchRead` retries up to 2 times and removes the failing field. |
| Odoo auth failure | Adapter attempts a bounded UID probe, then returns an explicit error. |
| Odoo URL or DB missing | Factory falls back to the shared mock adapter. |
| Unimplemented ERP type | Factory returns an explicit error; no mock fallback for SAP or Dynamics. |
| Duplicate tool calls | Turn plan de-duplicates identical write steps and sorts them into execution order. |
| Compound request | A create-and-deliver request becomes an ordered plan; follow-on steps run only after the
  primary write succeeds. |
| Partial chain failure | The gateway stops the chain, audits `failed`, and streams an error. |
| Missing session | Protected REST routes return `401`; WebSocket upgrade returns `401`. |
| Audit chain mismatch | Verification returns `valid=false` and an error naming the broken event. |
| LLM config push failure | Settings are saved and a warning is returned; the operator can retry. |
