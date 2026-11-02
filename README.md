# Kazi AI (ERPAgent)

[![Go](https://img.shields.io/badge/Go-1.26-00ADD8?logo=go)](https://go.dev)
[![Next.js](https://img.shields.io/badge/Next.js-16.3-000000?logo=nextdotjs)](https://nextjs.org)
[![Python](https://img.shields.io/badge/Python-3.14-3776AB?logo=python)](https://python.org)
[![License](https://img.shields.io/badge/license-proprietary-lightgrey)](#license)

A natural-language operations workspace that lets teams read and change ERP data through chat, with
server-side approval, audit, and tenant isolation.

## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [Architecture & Design Decisions](#architecture--design-decisions)
- [Complete Processing Flow](#complete-processing-flow)
- [API Reference](#api-reference)
- [Model / Audit Log](#model--audit-log)
- [Error Scenarios & Recovery](#error-scenarios--recovery)
- [Configuration](#configuration)
- [Sandbox Testing](#sandbox-testing)
- [Security](#security)
- [Dependencies](#dependencies)
- [Installation](#installation)
- [Performance Characteristics](#performance-characteristics)
- [Development & Testing](#development--testing)
- [Troubleshooting](#troubleshooting)
- [License](#license)

## Overview

| Capability | How it works |
|---|---|
| Conversational ERP access | A LangGraph agent reads ERP tools and returns typed intents and data. |
| Safe writes | Writes are returned as dry-run previews and execute only after explicit approval. |
| Multi-ERP connectors | A canonical Go domain model maps Odoo today; the mock adapter supports local work. |
| Live activity | One multiplexed WebSocket carries chat events and tenant business events. |
| Account isolation | Session cookies resolve a user and tenant; chats and connectors are tenant-scoped. |
| Audit trail | Every turn is hash-chained in Postgres and can be exported or verified. |

Kazi AI is a three-service application. The Next.js frontend renders the workspace, the Go gateway owns
authentication, ERP access, approval, history, and audit, and the Python AI engine turns natural language
into typed intents and tool calls.

The gateway is intentionally the only component that executes ERP writes. The AI engine may read through
backend tools, but a proposed mutation is held as a pending action and replayed by the gateway after the
user replies with an approval phrase such as `confirmed`.

## Quick Start

1. Copy and edit the deployment environment file.

   ```bash
   cp deploy/.env.example .env
   cp ai_engine/.env.example ai_engine/.env
   # Edit .env and ai_engine/.env. Set ODOO_* or use DEFAULT_ERP_TYPE=MOCK.
   ```

2. Start Postgres, Redis, the gateway, and the AI engine.

   ```bash
   docker compose up -d --build
   ```

3. Start the frontend in a second terminal.

   ```bash
   cd frontend
   npm install
   npm run dev
   ```

4. Open `http://localhost:3000`, register an account, and ask `Show customers`.

The development compose file does not include the frontend service. The frontend dev server proxies
`/api/v1` to `http://localhost:3001`, which is the gateway's published port.

## Architecture & Design Decisions

### Why One Go Gateway Owns Execution

The gateway is the trust boundary. It resolves the signed-in user and tenant, loads the tenant connector,
holds pending approvals in Redis, executes the approved intent, and writes the audit row. The AI engine
never receives ERP credentials and never writes to the ERP directly.

This keeps authorization and side effects in one language and one process. It also means the AI engine can
be replaced or upgraded without moving the approval and audit rules.

```go
// The gateway rehydrates the held intent and executes it only after approval.
parsed.Parameters.DryRun = false
connector, err := factory.GetERPConnector(connectors.ResolveTenantConfig(...))
resp, err := executeIntent(ctx, connector, pending.Prompt, &parsed)
```

### Why Writes Are Previews First

Every write path is modelled as a dry run by default. The AI engine returns a `dry_run` object with an
impact summary, risk level, and confirmation prompt. The gateway stores the parsed intent under the tenant
and conversation, streams a `dry_run` UI block, and waits.

Approval is deterministic. Short messages matching `confirmed`, `ok`, `yes`, `proceed`, or `go ahead`
execute the held action; `no`, `reject`, `cancel`, or `stop` discard it. The match is anchored to the whole
short message so a longer sentence containing `ok` cannot approve a money action accidentally.

### Why One Multiplexed WebSocket

Browsers cannot set custom headers during a WebSocket handshake. The gateway therefore authenticates the
socket with the session cookie and accepts `tenant_id`, `conversation_id`, and `erp_type` as query
parameters. One socket carries two logical channels:

- `chat`: `status`, `text`, `ui`, `done`, and `error` frames for the active conversation.
- `tenant-activity`: `activity` frames for approvals, executions, and other tenant events.

The frontend reconnects with exponential backoff. Pending approvals and recent turn history live in Redis,
so a reconnect does not lose the approval slot.

### Why Canonical Domain Models

The Go `domain` package defines portable `Customer`, `Product`, `SaleOrder`, `Invoice`, `Vendor`, and
`PurchaseOrder` types. Adapters translate those types to Odoo records. Handlers and the AI tool layer never
build Odoo-specific payloads.

This is the seam that keeps the API stable if a second ERP adapter is added. It also keeps the mock adapter
useful for tests and local demos.

### Why Redis and Postgres Have Different Jobs

Redis holds volatile coordination state: the pending approval slot, a 20-message window of conversation
memory, and tenant activity fan-out. Postgres holds durable records: users, sessions, chat conversations
and messages, tenant connectors, LLM settings, and the audit chain.

The gateway refuses to start in production if Postgres or Redis is unavailable. In development it falls
back to in-memory stores so a single process can be run without infrastructure.

### Why Runtime LLM Settings Live in the Gateway

The deployment administrator can change the provider, base URL, model, and token limit from the workspace.
The gateway encrypts the API key with AES-GCM before storing it in `llm_settings`, exposes only a redacted
view, and pushes the new config to the AI engine's internal admin endpoint.

The AI engine also pulls `/api/v1/internal/llm-config` at boot. The gateway is therefore the source of truth
and the AI engine can restart without losing the operator's saved model choice.

## Complete Processing Flow

### WebSocket Chat Turn

```text
Browser sends {type:"chat", conversation_id, text}
  │
  ├── Resolve session cookie -> tenant_id, user_id
  ├── Append user message to Redis window and Postgres history
  ├── Is the text a short approval or rejection?
  │     ├── Yes -> take pending action -> execute or discard
  │     └── No  -> continue
  ├── POST {AI_ENGINE_URL}/api/v1/process
  │     ├── AI agent reads ERP data through backend tools
  │     ├── AI returns typed response, plan, and optional dry_run
  │     └── Timeout is 180 seconds
  ├── If a write plan requires confirmation:
  │     ├── Save parsed intent as PendingAction in Redis
  │     ├── Publish approval_pending activity
  │     └── Stream dry_run UI block
  ├── Stream text and UI blocks
  ├── Write audit row
  └── Stream done
```

1. The socket is authenticated by `middleware.ResolveTenant`.
2. The gateway appends the user message to the Redis history window and durable chat history.
3. A deterministic approval matcher runs before any LLM call.
4. Normal prompts go to the AI engine's `/api/v1/process` endpoint.
5. The AI engine returns the assistant text, typed intent, plan, and optional dry-run result.
6. A pending write is stored under `tenant_id` and `conversation_id`.
7. The gateway streams `text`, then any `ui` blocks, then `done`.
8. A non-conversational turn writes an audit event with approval state `none`, `pending`, or `executed`.

### Approval Execution

```text
User sends "confirmed"
  │
  ├── Short-message matcher returns true
  ├── TakePending(tenant_id, conversation_id)
  │     ├── No pending -> treat as a normal prompt
  │     └── Pending found -> continue
  ├── Resolve tenant connector (name, erp_type, env fallback)
  ├── Set dry_run=false on primary intent and follow-on steps
  ├── executeIntent -> adapter method -> ERP
  ├── Stream result text and UI blocks
  ├── Append audit row with approval="executed"
  └── Publish executed_<intent> activity
```

1. `TakePending` removes the action from Redis, so a double `confirmed` cannot execute twice.
2. The connector is resolved from the tenant's saved connector, then the requested `erp_type`, then
   `DEFAULT_ERP_TYPE`, then the mock adapter when Odoo configuration is incomplete.
3. The primary intent runs first. Follow-on steps run only if the primary action actually executed.
4. A failed execution writes `approval="failed"` and streams an error frame.

### HTTP Chat Request

```text
POST /api/v1/chat
  │
  ├── Session auth middleware
  ├── POST AI_ENGINE_URL/api/v1/process
  ├── Convert ProcessResponse to ChatResponse
  └── Return JSON
```

The HTTP endpoint is a thin AI proxy. It does not append chat history, save a pending approval, execute an
ERP action, or write an audit row. Use the WebSocket for the complete operational flow.

### Connector Resolution

```text
ResolveTenantConfig(tenant_id, requested_erp_type)
  │
  ├── Named tenant connector if one matches
  ├── Otherwise default tenant connector
  ├── Otherwise requested erp_type
  ├── Otherwise DEFAULT_ERP_TYPE
  └── GetERPConnector
        ├── ODOO with URL + DB -> OdooAdapter
        ├── ODOO without URL/DB -> shared mock adapter
        ├── MOCK or empty -> shared mock adapter
        └── SAP, DYNAMICS, POSTGRES, MYSQL -> explicit error
```

### Audit Chain

```text
New event
  │
  ├── Read previous hash for tenant with row lock
  ├── Canonical JSON of event fields
  ├── SHA-256(prev_hash + canonical JSON)
  ├── Insert audit_events row
  └── Verify walks rows and recomputes every hash
```

## API Reference

All routes are under `{base_url}/api/v1`. `{base_url}` is `http://localhost:3001` in local development and
the public origin in production.

Authentication is a `kazi_session` HttpOnly cookie created by `/auth/login` or `/auth/register`. Protected
routes also accept `X-Internal-Token` for service calls. Outside production, `X-Tenant-ID` is accepted as a
development fallback.

| Method | Route | Auth | Purpose |
|---|---|---|---|
| `GET` | `/health` | Public | Gateway health. |
| `POST` | `/auth/register` | Public | Create account and session. |
| `POST` | `/auth/login` | Public | Start session. |
| `POST` | `/auth/logout` | Public | Revoke session. |
| `GET` | `/auth/me` | Session | Current account. |
| `PATCH` | `/auth/profile` | Session | Update name and email. |
| `POST` | `/auth/password` | Session | Change password. |
| `GET` | `/chats` | Session | List conversations. |
| `GET` | `/chats/{id}` | Session | Read messages. |
| `DELETE` | `/chats/{id}` | Session | Delete conversation. |
| `POST` | `/chat` | Session | AI proxy; no persistence or execution. |
| `POST` | `/intents` | Session | Alias of `/chat`. |
| `GET` | `/ws` | Session or query | Multiplexed chat and activity socket. |
| `GET` | `/audit` | Session | List audit events. |
| `GET` | `/audit/export` | Session | Export audit CSV. |
| `GET` | `/audit/verify` | Session | Verify hash chain. |
| `GET` | `/connectors` | Session | List tenant connectors. |
| `POST` | `/connectors` | Session | Create or replace connector. |
| `POST` | `/connectors/test` | Session | Test unsaved connector. |
| `DELETE` | `/connectors/{name}` | Session | Delete connector. |
| `POST` | `/connectors/{name}/test` | Session | Test saved connector. |
| `GET` | `/settings/llm` | Admin | Read redacted LLM settings. |
| `POST` | `/settings/llm` | Admin | Save and push LLM settings. |
| `POST` | `/settings/llm/test` | Admin | Probe provider. |
| `GET` | `/internal/llm-config` | Internal token | AI engine startup config. |

### `GET /health`

Returns gateway liveness. It does not check Postgres, Redis, or the AI engine.

```json
{
  "status": "up",
  "service": "ERPAgent Gateway (Gorilla Mux)"
}
```

### `POST /auth/register`

Creates a tenant-scoped account and sets the session cookie. Every registration creates a new tenant ID.

| Name | Type | Required | Description |
|---|---|---|---|
| `name` | string | Yes | Display name. |
| `email` | string | Yes | Unique, normalized email. |
| `password` | string | Yes | Minimum 8 characters. |

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

| Error | Status | Body |
|---|---|---|
| Invalid payload | `400` | `{"error":"Invalid registration payload"}` |
| Missing name | `400` | `{"error":"Name is required"}` |
| Invalid email | `400` | `{"error":"A valid email is required"}` |
| Short password | `400` | `{"error":"Password must be at least 8 characters"}` |
| Duplicate email | `409` | `{"error":"An account with that email already exists"}` |

### `POST /auth/login`

Verifies credentials and sets a new 7-day session cookie.

| Name | Type | Required | Description |
|---|---|---|---|
| `email` | string | Yes | Account email. |
| `password` | string | Yes | Account password. |

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

| Error | Status | Body |
|---|---|---|
| Invalid payload | `400` | `{"error":"Invalid login payload"}` |
| Bad credentials | `401` | `{"error":"Invalid email or password"}` |

### `POST /auth/logout`

Revokes the current session and clears the cookie. No request body is required.

```json
{
  "status": "success"
}
```

### `GET /auth/me`

Returns the signed-in account. Requires a real user session; service and development identities are
rejected.

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

### `PATCH /auth/profile`

Updates the signed-in user's name and email. Omitted fields keep their current values.

| Name | Type | Required | Description |
|---|---|---|---|
| `name` | string | No | New display name; cannot be empty. |
| `email` | string | No | New unique email. |

```json
{
  "name": "Operations Lead",
  "email": "lead@example.com"
}
```

| Error | Status | Body |
|---|---|---|
| Invalid payload | `400` | `{"error":"Invalid profile payload"}` |
| Empty name | `400` | `{"error":"Name cannot be empty"}` |
| Duplicate email | `409` | `{"error":"That email is already in use"}` |

### `POST /auth/password`

Replaces the password after verifying the current one. Existing sessions are not revoked.

| Name | Type | Required | Description |
|---|---|---|---|
| `current_password` | string | Yes | Current password. |
| `new_password` | string | Yes | Minimum 8 characters. |

```json
{
  "status": "success"
}
```

| Error | Status | Body |
|---|---|---|
| Short new password | `400` | `{"error":"New password must be at least 8 characters"}` |
| Wrong current password | `401` | `{"error":"Current password is incorrect"}` |

### `GET /chats`

Lists the signed-in user's conversations in descending update order.

```json
{
  "status": "success",
  "data": [
    {
      "id": "conv-...",
      "title": "Show overdue invoices",
      "created_at": "2026-11-02T10:00:00Z",
      "updated_at": "2026-11-02T10:02:00Z",
      "message_count": 4
    }
  ]
}
```

### `GET /chats/{id}`

Returns the messages for one conversation owned by the signed-in user.

```json
{
  "status": "success",
  "data": [
    {
      "id": 1,
      "role": "user",
      "content": "Show vendors",
      "created_at": "2026-11-02T10:00:00Z"
    }
  ]
}
```

| Error | Status | Body |
|---|---|---|
| Not found or not owned | `404` | `{"error":"Conversation not found"}` |

### `DELETE /chats/{id}`

Deletes the conversation and its messages.

```json
{
  "status": "success"
}
```

### `POST /chat`

Sends a prompt to the AI engine and returns the typed response. This endpoint is a proxy only. It does not
persist history, hold an approval, execute an ERP action, or write audit data.

| Name | Type | Required | Description |
|---|---|---|---|
| `prompt` | string | Yes | Natural-language request. |
| `conversation_id` | string | No | AI thread suffix; defaults to `default`. |
| `tenant_id` | string | No | Ignored when session auth supplies a tenant. |
| `erp_type` | string | No | Connector hint such as `ODOO` or `MOCK`. |

```json
{
  "status": "success",
  "prompt": "Show overdue invoices",
  "parsed_intent": {
    "intent_type": "GET_OVERDUE_INVOICES",
    "action_type": "READ",
    "summary": "Found 2 overdue invoices."
  },
  "message": "Found 2 overdue invoices.",
  "data": []
}
```

| Error | Status | Body |
|---|---|---|
| Empty prompt | `400` | `{"error":"Prompt cannot be empty"}` |
| AI unavailable | `500` | `{"error":"AI Engine process communication error","details":"..."}` |

### `GET /ws`

Upgrades to the multiplexed WebSocket. Query parameters supply the conversation and optional ERP hint.

| Name | Type | Required | Description |
|---|---|---|---|
| `conversation_id` | string | No | Conversation key; defaults to `default`. |
| `tenant_id` | string | No | Development fallback when no session cookie exists. |
| `erp_type` | string | No | Connector hint such as `ODOO` or `MOCK`. |

Client frame:

```json
{
  "type": "chat",
  "conversation_id": "conv-...",
  "text": "Create RFQ from ABC for 10 of Chair"
}
```

Server frames:

```json
{"channel":"chat","type":"status","conversation_id":"conv-...","text":"Resolving vendor..."}
{"channel":"chat","type":"text","conversation_id":"conv-...","text":"I prepared a preview."}
{"channel":"chat","type":"ui","conversation_id":"conv-...","blocks":[{"type":"dry_run","props":{}}]}
{"channel":"chat","type":"done","conversation_id":"conv-..."}
{"channel":"tenant-activity","type":"activity","event":{"kind":"approval_pending","title":"..."}}
```

### `GET /audit`

Lists audit events for the caller's tenant, newest first. Query parameter `limit` caps the result.

```json
{
  "status": "success",
  "count": 1,
  "data": [
    {
      "id": 42,
      "timestamp": "2026-11-02T10:02:00Z",
      "tenant_id": "tenant_...",
      "actor": "conv-...",
      "channel": "ws-chat",
      "prompt": "Create RFQ from ABC for 10 of Chair",
      "intent": "CREATE_RFQ",
      "target": "",
      "approval": "pending",
      "prev_hash": "...",
      "hash": "..."
    }
  ]
}
```

### `GET /audit/export`

Returns the tenant audit trail as `text/csv`. The response is a downloadable attachment.

### `GET /audit/verify`

Walks the tenant chain and recomputes every hash.

```json
{
  "status": "verified",
  "checked": 42,
  "ok": true
}
```

A broken chain returns `status: "tampered"`, `ok: false`, and an `error` message.

### `GET /connectors`

Lists tenant connectors with secrets redacted.

```json
{
  "status": "success",
  "count": 1,
  "data": [
    {
      "tenant_id": "tenant_...",
      "name": "default",
      "erp_type": "ODOO",
      "base_url": "http://odoo:8069",
      "db_name": "erp",
      "username": "api@example.com",
      "has_secret": true,
      "updated_at": "2026-11-02T10:00:00Z"
    }
  ]
}
```

### `POST /connectors`

Creates or replaces a tenant connector. `name` defaults to `default`. Accepted `erp_type` values are
`MOCK`, `ODOO`, `SAP`, and `DYNAMICS`. Only `MOCK` and `ODOO` have server implementations.

| Name | Type | Required | Description |
|---|---|---|---|
| `name` | string | No | Connector name; defaults to `default`. |
| `erp_type` | string | Yes | `MOCK`, `ODOO`, `SAP`, or `DYNAMICS`. |
| `base_url` | string | Conditional | Odoo base URL. |
| `db_name` | string | Conditional | Odoo database name. |
| `username` | string | Conditional | Odoo username. |
| `password` | string | Conditional | Odoo password or API key. |

```json
{
  "name": "default",
  "erp_type": "ODOO",
  "base_url": "http://odoo:8069",
  "db_name": "erp",
  "username": "api@example.com",
  "password": "change-me"
}
```

### `POST /connectors/test`

Tests an unsaved connector without persisting it. The response is produced by
`connectors.TestConnection`.

### `DELETE /connectors/{name}`

Deletes a tenant connector by name.

### `POST /connectors/{name}/test`

Tests a saved connector by name.

### `GET /settings/llm`

Returns the runtime LLM settings with the API key redacted. Only the deployment administrator
(`ADMIN_EMAIL`) can call this route.

```json
{
  "status": "success",
  "data": {
    "provider": "groq",
    "base_url": "https://api.groq.com/openai/v1",
    "model": "llama3-70b-8192",
    "max_tokens": 400,
    "has_api_key": true,
    "updated_at": "2026-11-02T10:00:00Z"
  }
}
```

### `POST /settings/llm`

Saves LLM settings and pushes them to the AI engine. An empty `api_key` keeps the saved key. A failed push
still saves the configuration and returns a `warning`.

| Name | Type | Required | Description |
|---|---|---|---|
| `provider` | string | No | Provider name. |
| `base_url` | string | No | Absolute OpenAI-compatible base URL. |
| `model` | string | No | Model name. |
| `api_key` | string | No | New key; empty keeps the existing key. |
| `max_tokens` | integer | No | Clamped to `1..8192`. |

```json
{
  "status": "success",
  "data": {
    "provider": "groq",
    "base_url": "https://api.groq.com/openai/v1",
    "model": "llama3-70b-8192",
    "max_tokens": 400,
    "has_api_key": true
  },
  "applied": true
}
```

### `POST /settings/llm/test`

Probes the provider's `/models` endpoint through the AI engine without changing runtime config.

```json
{
  "ok": true,
  "message": "Provider reachable; model configured as llama3-70b-8192",
  "model": "llama3-70b-8192"
}
```

### `GET /internal/llm-config`

Internal route consumed by the AI engine at boot. Requires `X-Internal-Token` when the token is set.
Returns the plaintext key to the trusted internal service only.

### Sales REST Endpoints

| Method | Route | Purpose |
|---|---|---|
| `GET` | `/sales/customers` | List customers. |
| `GET` | `/sales/customers/count` | Count customers. |
| `GET` | `/sales/products` | List products. |
| `GET` | `/sales/products/count` | Count products. |
| `GET` | `/sales/orders` | List sale orders. |
| `GET` | `/sales/orders/count` | Count sale orders. |
| `POST` | `/sales/orders` | Create sale order. |
| `GET` | `/sales/orders/{id}` | Read sale order. |
| `POST` | `/sales/orders/{id}/confirm` | Confirm order. |
| `POST` | `/sales/orders/{id}/cancel` | Cancel order. |
| `POST` | `/sales/orders/{id}/deliver` | Deliver order. |
| `POST` | `/sales/orders/{id}/invoice` | Create invoice. |
| `GET` | `/sales/invoices` | List invoices. |
| `GET` | `/sales/invoices/count` | Count invoices. |
| `POST` | `/sales/invoices/{id}/post` | Post invoice. |

### Purchase REST Endpoints

| Method | Route | Purpose |
|---|---|---|
| `GET` | `/purchase/vendors` | List vendors. |
| `GET` | `/purchase/vendors/count` | Count vendors. |
| `GET` | `/purchase/orders` | List purchase orders. |
| `POST` | `/purchase/orders` | Create RFQ. |
| `GET` | `/purchase/orders/{id}` | Read purchase order. |
| `POST` | `/purchase/orders/{id}/confirm` | Confirm purchase order. |
| `POST` | `/purchase/orders/{id}/cancel` | Cancel purchase order. |
| `POST` | `/purchase/orders/{id}/receive` | Receive purchase order. |
| `POST` | `/purchase/orders/{id}/bill` | Create vendor bill. |
| `GET` | `/purchase/bills` | List vendor bills. |
| `POST` | `/purchase/bills/{id}/post` | Post vendor bill. |

## Model / Audit Log

There is no Odoo ORM in this repository. The “models” are Go domain structs and Postgres tables. The
canonical structs are defined in `backend/internal/domain/models.go`; persistence schemas live beside their
stores.

### Customer

| Field | Type | Purpose |
|---|---|---|
| `id` | string | ERP record ID. |
| `name` | string | Customer name. |
| `email` | string | Contact email. |
| `phone` | string | Contact phone. |
| `credit_limit` | float64 | Approved credit. |
| `balance` | float64 | Outstanding balance. |
| `street`, `city`, `country` | string | Address fields. |

### Product

| Field | Type | Purpose |
|---|---|---|
| `id` | string | ERP record ID. |
| `name` | string | Product name. |
| `sku` | string | Stock keeping unit. |
| `list_price` | float64 | Sale price. |
| `standard_price` | float64 | Cost. |
| `qty_available` | float64 | On-hand quantity. |
| `unit_of_measure` | string | UoM name. |
| `category` | string | Product category. |

### SaleOrder

| Field | Type | Purpose |
|---|---|---|
| `id` | string | ERP record ID. |
| `order_no` | string | Human-readable order number. |
| `customer_id`, `customer_name` | string | Customer reference. |
| `date_order` | time.Time | Order date. |
| `amount_untaxed`, `amount_tax`, `amount_total` | float64 | Totals. |
| `state` | string | `draft`, `sent`, `sale`, `done`, `cancel`. |
| `lines` | []SaleOrderLine | Product lines. |
| `delivery_state`, `invoice_state` | string | Fulfilment and billing state. |

### Invoice

| Field | Type | Purpose |
|---|---|---|
| `id`, `invoice_no` | string | ERP ID and number. |
| `customer_id`, `customer_name` | string | Customer reference. |
| `order_id` | string | Source order. |
| `move_type` | string | `out_invoice` or `out_refund`. |
| `amount_total`, `amount_due` | float64 | Totals. |
| `status` | string | `draft`, `posted`, `paid`, `cancel`. |
| `payment_state` | string | `not_paid`, `partial`, `paid`, `in_payment`. |
| `due_date` | time.Time | Payment due date. |
| `etims_status`, `cu_serial_no`, `irn`, `qr_code_url` | string | KRA eTIMS fields. |

### Vendor and PurchaseOrder

| Model | Key fields |
|---|---|
| `Vendor` | `id`, `name`, `email`, `phone`, `balance`, address fields. |
| `PurchaseOrder` | `id`, `order_no`, `vendor_id`, totals, `state`, `lines`, receipt and billing state. |

### Persistence Tables

| Table | Key columns | Store interface |
|---|---|---|
| `users` | `id`, `tenant_id`, `email`, `name`, `password_hash` | `auth.Store` |
| `sessions` | `token_hash`, `user_id`, `expires_at` | `auth.Store` |
| `chat_conversations` | `tenant_id`, `id`, `user_id`, `title` | `chathistory.Store` |
| `chat_messages` | `tenant_id`, `conversation_id`, `role`, `content` | `chathistory.Store` |
| `audit_events` | `id`, `tenant_id`, `actor`, `prompt`, `intent`, `approval`, `prev_hash`, `hash` | `audit.Store` |
| `tenant_connectors` | `tenant_id`, `name`, `erp_type`, `base_url`, `db_name`, `username`, `password` | `connectors.Store` |
| `llm_settings` | singleton row, provider, base URL, model, encrypted key, max tokens | `llmsettings.Store` |

Key methods:

| Interface | Methods |
|---|---|
| `auth.Store` | `CreateUser`, `GetAccountByEmail`, `GetAccountByID`, `GetUserByID`, `UpdateProfile`, `UpdatePassword`, `CreateSession`, `GetUserBySession`, `DeleteSession`, `DeleteUserSessions` |
| `chathistory.Store` | `AppendMessage`, `ListConversations`, `GetMessages`, `DeleteConversation` |
| `audit.Store` | `Append`, `List`, `Verify` |
| `connectors.Store` | `Upsert`, `List`, `Get`, `Delete` |
| `llmsettings.Store` | `Get`, `Save` |
| `realtime.Store` | `SavePending`, `TakePending`, `DropPending`, `AppendHistory`, `History`, `PublishActivity`, `SubscribeActivity` |

Uniqueness and integrity rules:

- `users.email` is unique and normalized to lowercase.
- `sessions.token_hash` is the SHA-256 of the bearer token; the raw token is only in the cookie.
- `chat_conversations` is unique on `(tenant_id, id)` and indexed by `(tenant_id, user_id, updated_at)`.
- `audit_events` is append-only in practice; `Verify` checks `prev_hash` and `hash` for a tenant.
- `tenant_connectors` is keyed by `(tenant_id, name)`.
- `llm_settings` is a singleton row enforced by `id BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id)`.

## Error Scenarios & Recovery

| Scenario | What happens | Recovery |
|---|---|---|
| Postgres unavailable in production | Gateway calls `log.Fatalf`; no in-memory fallback. | Restore Postgres, then restart the gateway. |
| Redis unavailable in production | Gateway refuses to start with in-memory chat state. | Restore Redis, then restart the gateway. |
| AI engine unreachable | WebSocket streams `AI Engine process communication error`; HTTP returns `500`. | Check `AI_ENGINE_URL`, AI container health, and network. |
| LLM provider rate limit | AI engine returns `429` with a token-budget hint. | Start a new conversation or shorten the prompt. |
| Unknown connector type | Factory returns an explicit error; no fake success. | Save `MOCK` or `ODOO`, or implement the adapter. |
| Odoo URL or DB missing | Factory falls back to the shared mock adapter. | Set `ODOO_URL` and `ODOO_DB`, or accept mock behavior for demos. |
| Approval already consumed | `TakePending` returns nil; prompt is treated normally. | Re-issue the original request and approve again. |
| Duplicate email | `409` from register or profile update. | Use a different email or sign in to the existing account. |
| Expired session | Protected route returns `401`. | Sign in again. |
| Audit chain mismatch | `/audit/verify` returns `status=tampered` and an error. | Treat the trail as tampered; investigate database access. |

## Configuration

### Environment Variables

| Key | Default | Description |
|---|---|---|
| `PORT` | `3000` | Gateway listen port. |
| `ENVIRONMENT` | `development` | `production` enables fail-fast stores and secure cookies. |
| `ADMIN_EMAIL` | empty | Account allowed to manage deployment-wide LLM settings. |
| `DATABASE_URL` | local Postgres DSN | Application database. |
| `REDIS_URL` | `localhost:6379` | Pending approvals, history window, activity fan-out. |
| `AI_ENGINE_URL` | `http://localhost:8000` | AI engine HTTP base URL. |
| `AI_ENGINE_GRPC` | `localhost:50051` | Reserved gRPC endpoint. |
| `VAULT_ADDR` | `http://localhost:8200` | Vault address; not yet used for connector secrets. |
| `ODOO_URL` | empty | Default Odoo base URL. |
| `ODOO_DB` | empty | Default Odoo database. |
| `ODOO_USER` | empty | Default Odoo username. |
| `ODOO_PASSWORD` | empty | Default Odoo password or API key. |
| `DEFAULT_ERP_TYPE` | unset in Go, `ODOO` in compose | Fallback ERP type. |
| `LLM_PROVIDER` | `groq` | Fallback provider. |
| `LLM_BASE_URL` | `https://api.groq.com/openai/v1` | Fallback base URL. |
| `LLM_MODEL` | `llama3-70b-8192` | Fallback model. |
| `LLM_API_KEY` | empty | Fallback API key. |
| `LLM_MAX_TOKENS` | `400` | Fallback token limit. |
| `INTERNAL_API_TOKEN` | empty | Shared gateway-to-AI secret. Required in production. |
| `SETTINGS_ENCRYPTION_KEY` | empty | AES key material for stored LLM keys. Required in production. |

### Runtime LLM Settings

| Field | Value | Notes |
|---|---|---|
| `provider` | `groq`, `openai`, etc. | Stored with the model choice. |
| `base_url` | absolute URL | Must include scheme and host. |
| `model` | model name | Required. |
| `api_key` | secret | Encrypted at rest; never returned. |
| `max_tokens` | `1..8192` | Values above the cap are clamped. |

### Connector Fields

| Field | Required | Notes |
|---|---|---|
| `name` | No | Defaults to `default`. |
| `erp_type` | Yes | `MOCK`, `ODOO`, `SAP`, `DYNAMICS`. |
| `base_url` | Odoo | Example: `http://odoo:8069`. |
| `db_name` | Odoo | Odoo database. |
| `username` | Odoo | API user. |
| `password` | Odoo | Persisted in Postgres in v1. |

## Sandbox Testing

The mock adapter is the sandbox. It is stateful for the life of the gateway process and requires no ERP.

Start the gateway and AI engine:

```bash
docker compose up -d --build
```

Create a mock connector:

```bash
curl -s -X POST http://localhost:3001/api/v1/connectors \
  -H 'Content-Type: application/json' \
  -H 'X-Tenant-ID: dev-tenant' \
  -d '{"name":"default","erp_type":"MOCK"}'
```

Ask a read question:

```bash
curl -s -X POST http://localhost:3001/api/v1/chat \
  -H 'Content-Type: application/json' \
  -H 'X-Tenant-ID: dev-tenant' \
  -d '{"prompt":"Show customers","conversation_id":"sandbox-1"}'
```

The mock adapter supports the same canonical interface as Odoo: customers, products, sale orders,
invoices, vendors, purchase orders, and vendor bills. It is not a faithful ERP simulator; use it for
plumbing and UI checks.

## Security

| Layer | Mechanism |
|---|---|
| Browser session | `kazi_session` HttpOnly cookie, `SameSite=Lax`, `Secure` in production. |
| Password storage | `golang.org/x/crypto` password hashing. |
| Service calls | `X-Internal-Token` constant-time comparison. |
| Tenant isolation | Session-derived `tenant_id` on chats, connectors, audit, and history. |
| Connector secrets | Redacted from API responses; stored in Postgres in v1. |
| LLM API keys | AES-GCM encrypted at rest; redacted from reads. |
| WebSocket origin | Same-host check; localhost allowed outside production. |
| Audit integrity | SHA-256 hash chain with previous-hash linkage. |
| CORS | Same-host only; localhost allowed outside production. |

Production considerations:

- Rotate any Odoo credentials that were ever committed to repository history.
- Move connector passwords to Vault or another secret manager before multi-customer production.
- Set `INTERNAL_API_TOKEN` and `SETTINGS_ENCRYPTION_KEY`; do not use the development fallbacks.
- Terminate TLS at Caddy and keep the gateway and AI engine on the private Docker network.
- Back up Postgres and test `/audit/verify` as part of operations.

## Dependencies

| Dependency | Purpose |
|---|---|
| Go 1.26 | Gateway runtime. |
| `gorilla/mux` | HTTP routing. |
| `gorilla/websocket` | Multiplexed chat and activity socket. |
| `lib/pq` | Postgres driver. |
| `redis/go-redis` | Redis client. |
| `golang.org/x/crypto` | Password hashing. |
| Next.js 16.3 | Workspace UI and same-origin proxy. |
| React 19.2 | UI runtime. |
| Tailwind CSS 3.4 | Styling. |
| Python 3.14 | AI engine runtime. |
| FastAPI | AI HTTP API and admin endpoints. |
| LangGraph / LangChain | Agent workflow and tool loop. |
| Postgres + pgvector image | Durable application data; vector extension available. |
| Redis 7 | Pending actions, history window, activity fan-out. |
| Caddy 2 | TLS termination and reverse proxy. |

## Installation

```text
ERPAgent/
├── ai_engine/              FastAPI + LangGraph service
│   ├── src/api/            HTTP and admin routes
│   ├── src/core/           settings, runtime LLM, logging
│   ├── src/graph/          agent workflow and tools
│   ├── src/services/       intent, memory, guards
│   └── tests/              pytest suite
├── backend/                Go gateway
│   ├── cmd/main.go         route wiring and stores
│   ├── internal/           auth, chat, audit, connectors, adapters
│   └── middleware/         session, tenant, logging, recovery
├── frontend/               Next.js workspace
│   ├── src/app/            landing, auth, workspace routes
│   ├── src/components/     registry, admin, profile, tables
│   └── src/lib/            REST, WebSocket, auth clients
├── deploy/                 Caddyfile and production env template
├── docker-compose.yml      development stack
└── docker-compose.prod.yml production stack
```

Local development:

```bash
# Gateway
cd backend
go mod download
go run ./cmd/main.go

# AI engine
cd ai_engine
uv sync
.venv/bin/fastapi run src/main.py --port 8000

# Frontend
cd frontend
npm install
npm run dev
```

Docker development:

```bash
docker compose up -d --build
```

Production:

```bash
cp deploy/.env.example .env
# Fill DOMAIN, ADMIN_EMAIL, DATABASE_URL, INTERNAL_API_TOKEN, SETTINGS_ENCRYPTION_KEY.
docker compose -f docker-compose.prod.yml up -d --build
```

## Performance Characteristics

| Metric | Typical value | Notes |
|---|---|---|
| HTTP server read/write timeout | 15 seconds | Gateway `http.Server`. |
| AI process timeout | 180 seconds | ReAct turns can make several LLM calls. |
| WebSocket write deadline | 10 seconds | Per frame. |
| Conversation memory window | 20 messages | Redis history per tenant/conversation. |
| Tool result row cap | 20 rows | AI engine tool context. |
| Prompt guard | 2000 characters | Truncated before the LLM call. |
| Session lifetime | 7 days | `kazi_session`. |
| Audit chain verification | Linear in event count | Per tenant. |

Primary bottlenecks are LLM latency and ERP response time. The gateway keeps the WebSocket read loop
separate from turn processing so pings continue while a slow turn is in flight.

## Development & Testing

```bash
# Gateway
cd backend
go test ./...
go vet ./...

# AI engine
cd ai_engine
uv run pytest

# Frontend
cd frontend
npm run build
npm run lint
```

| Test area | Files | Coverage |
|---|---|---|
| Gateway handlers | `backend/internal/handlers/*_test.go` | Auth, chat, WS, settings, intent IR, acceptance. |
| Gateway stores | `backend/internal/{audit,chathistory,connectors,realtime}/*_test.go` | Persistence and chain behavior. |
| Adapters | `backend/internal/adapters/{mock,odoo}/*_test.go` | Canonical model mapping. |
| Middleware/config | `backend/middleware`, `backend/internal/config` | Auth resolution and env parsing. |
| AI API | `ai_engine/tests/test_api_routes.py`, `test_process_route.py` | HTTP behavior and turn plan. |
| AI intent layer | `ai_engine/tests/test_intents.py`, `test_intent_ir.py`, `test_filter_compiler.py` | Classification and query IR. |
| AI guards | `test_dialogue_guards.py`, `test_token_guards.py`, `test_tool_context.py` | Safety and context. |
| Live acceptance | `backend/tests/*_test.go` | Sales, purchase, and live ERP paths. |

Known limitations:

- There are no frontend unit or end-to-end test files in this repository.
- AI integration tests that need a live provider are skipped unless `RUN_LLM_TESTS=1`.
- `SAP` and `DYNAMICS` are accepted by the connector API but have no Go implementation.
- The mock adapter is process-local and resets on gateway restart.

## Troubleshooting

| Symptom | Likely Cause | Solution |
|---|---|---|
| `Failed to fetch` in the workspace | Gateway or AI engine is down, or WS URL is wrong. | Check `docker compose ps`, gateway logs, and `NEXT_PUBLIC_GATEWAY_WS`. |
| Login returns `401` | Wrong credentials or expired session. | Re-enter credentials; clear the session cookie. |
| Chat returns `AI Engine process communication error` | AI engine unreachable or crashed. | Check `AI_ENGINE_URL` and AI container logs. |
| `429` from AI engine | Conversation exceeded the model token budget. | Start a new chat or shorten the prompt. |
| Approval does nothing | Pending action was consumed or superseded. | Re-issue the request and reply `confirmed` to the latest preview. |
| Odoo actions hit mock data | `ODOO_URL` or `ODOO_DB` is empty. | Set both and restart; verify with `/connectors/{name}/test`. |
| Connector test fails | Wrong URL, database, or credentials. | Test from inside the gateway network namespace. |
| Audit verify fails | Chain row was modified or deleted. | Investigate database access; restore from backup. |
| Frontend cannot reach API | Dev proxy target is wrong. | Set `NEXT_PUBLIC_GATEWAY_HTTP` or run the gateway on `3001`. |

## License

No license file is present in this repository. Treat the code as proprietary until a license is added.
