# Kazi AI (ERPAgent) Testing

Unit, integration, sandbox, and manual verification for the gateway, AI engine, and workspace.

## 1. Prerequisites

| Requirement | Version | Notes |
|---|---|---|
| Go | `1.26.5` | Gateway tests. |
| Python | `3.14` | AI engine tests. |
| uv | Current | `uv run pytest`. |
| Node.js | `22` recommended | Frontend build and lint checks. |
| Docker Compose | v2 | Integration stack for Postgres, Redis, and services. |
| `curl` | Current | HTTP checks. |
| `websocat` | Current | Optional WebSocket checks. |
| `jq` | Current | Optional JSON assertions. |

Start from the repository root:

```bash
cd /path/to/ERPAgent
```

## 2. Running Unit Tests

### Gateway

```bash
cd backend
go test ./...
```

Run one package:

```bash
go test ./internal/handlers -v
```

Run one test:

```bash
go test ./internal/handlers -run TestAcceptance_Q10Demo -v
```

Run with the race detector:

```bash
go test -race ./...
```

Static checks:

```bash
go vet ./...
```

### AI Engine

```bash
cd ai_engine
uv sync
uv run pytest
```

Run one file:

```bash
uv run pytest tests/test_process_route.py -v
```

Run one test:

```bash
uv run pytest tests/test_process_route.py::test_process_write_returns_preview_only_plan -v
```

Run live-provider tests:

```bash
RUN_LLM_TESTS=1 uv run pytest -m integration
```

### Frontend

There are no frontend unit or end-to-end test files in this repository. Use the build and lint gates:

```bash
cd frontend
npm run build
npm run lint
```

### Test Class Reference

| File or package | Focus |
|---|---|
| `backend/internal/handlers/acceptance_test.go` | End-to-end mock demo: read, RFQ preview, approval, activity, audit. |
| `backend/internal/handlers/ws_test.go` | WebSocket turn flow, approval matcher, UI blocks. |
| `backend/internal/handlers/intent_ir_test.go` | Query IR, filters, limits, count semantics. |
| `backend/internal/handlers/query_test.go` | Name resolution and row verification. |
| `backend/internal/handlers/auth_test.go` | Register, login, profile, password, duplicate handling. |
| `backend/internal/handlers/settings_test.go` | Admin-only LLM settings and redaction. |
| `backend/internal/handlers/lanec_test.go` | Audit and connector endpoints. |
| `backend/internal/audit/audit_test.go` | Hash chain append and verify. |
| `backend/internal/chathistory/chathistory_test.go` | Conversation lifecycle and tenant isolation. |
| `backend/internal/connectors/connectors_test.go` | Connector CRUD and precedence. |
| `backend/internal/realtime/store_test.go` | Pending actions, history window, pub/sub. |
| `backend/internal/adapters/mock/mock_adapter_test.go` | Mock ERP lifecycle and dry-run behavior. |
| `backend/internal/adapters/odoo/odoo_adapter_test.go` | Odoo mapping and dry-run requests. |
| `backend/middleware/middleware_test.go` | Session, internal token, and dev tenant fallback. |
| `backend/tests/sales_e2e_test.go` | Sales acceptance against a live or mock stack. |
| `backend/tests/purchase_e2e_test.go` | Purchase acceptance against a live or mock stack. |
| `ai_engine/tests/test_api_routes.py` | AI health and public endpoint behavior. |
| `ai_engine/tests/test_process_route.py` | `/process` read and write preview turns. |
| `ai_engine/tests/test_intents.py` | Intent classification and parameters. |
| `ai_engine/tests/test_intent_ir.py` | Query IR and count behavior. |
| `ai_engine/tests/test_filter_compiler.py` | Allowlisted domain compilation. |
| `ai_engine/tests/test_dialogue_guards.py` | Conversational and confirmation guards. |
| `ai_engine/tests/test_token_guards.py` | Prompt and tool-output limits. |
| `ai_engine/tests/test_tool_context.py` | Tenant and ERP type propagation. |
| `ai_engine/tests/test_workflow.py` | Graph compilation and guard nodes. |

## 3. Manual Testing Flows

### Flow 1: Account and Session

Prerequisites: gateway and Postgres running.

```bash
curl -s -c /tmp/kazi-cookies.txt -X POST http://localhost:3001/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"name":"Test User","email":"test@example.com","password":"test-password"}'
```

Expected response:

```json
{
  "status": "success",
  "user": {
    "id": "usr_...",
    "tenant_id": "tenant_...",
    "email": "test@example.com",
    "name": "Test User"
  }
}
```

Verify in the UI:

1. Open `http://localhost:3000/app`.
2. Confirm the workspace loads and the header shows **Connected**.
3. Open **Profile** and confirm the name and email match.

### Flow 2: Read Customers Through the AI

Prerequisites: mock connector saved as `default`; AI engine running.

```bash
curl -s -b /tmp/kazi-cookies.txt -X POST http://localhost:3001/api/v1/chat \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"Show customers","conversation_id":"manual-read-1"}'
```

Expected: HTTP `200` with `status: "success"`, an intent such as `GET_CUSTOMERS`, and a `data` array.

Verify in the UI:

1. Open **Chat**.
2. Send `Show customers`.
3. Confirm a table appears and the activity feed stays connected.
4. Open **Chat history** and confirm `manual-read-1` is listed.

### Flow 3: RFQ Preview and Approval

Prerequisites: mock connector; a product named `Chair` and a vendor named `ABC` in the mock catalog.

Connect the WebSocket:

```bash
websocat 'ws://localhost:3001/api/v1/ws?conversation_id=manual-rfq-1&erp_type=MOCK' \
  -H 'Cookie: kazi_session=<session-token>'
```

Send:

```json
{"type":"chat","conversation_id":"manual-rfq-1","text":"Create RFQ from ABC for 10 of Chair"}
```

Expected frames:

```json
{"channel":"chat","type":"text","conversation_id":"manual-rfq-1","text":"Draft RFQ preview..."}
{"channel":"chat","type":"ui","conversation_id":"manual-rfq-1","blocks":[{"type":"dry_run","props":{}}]}
{"channel":"chat","type":"done","conversation_id":"manual-rfq-1"}
```

Then send:

```json
{"type":"chat","conversation_id":"manual-rfq-1","text":"confirmed"}
```

Expected: a result frame without a new approval card, plus an `executed_*` activity event.

Verify in the UI:

1. Open **Approvals** and confirm the dry-run card is present.
2. Reply `confirmed` in chat.
3. Confirm the result text appears and the card is no longer executable.
4. Open **Activity** and confirm `approval_pending` and `executed_*` entries.
5. Open **Audit Trail** and confirm rows with `pending` and `executed`.

### Flow 4: Rejection

Prerequisites: a pending RFQ preview.

Send:

```json
{"type":"chat","conversation_id":"manual-rfq-1","text":"no"}
```

Expected: `Discarded. Nothing was executed` and an audit row with `approval="rejected"`.

Verify in the UI:

1. Confirm no ERP record was created.
2. Confirm the approval card is superseded or gone.
3. Confirm the audit trail contains the rejection.

### Flow 5: Chat History

```bash
curl -s -b /tmp/kazi-cookies.txt http://localhost:3001/api/v1/chats
```

Expected: `status: "success"` and a `data` array with at least one conversation.

```bash
curl -s -b /tmp/kazi-cookies.txt \
  http://localhost:3001/api/v1/chats/manual-rfq-1
```

Expected: messages in order, with `role` of `user` or `assistant`.

Verify in the UI: open **Chat history**, select the conversation, confirm the messages reload.

### Flow 6: Audit Verification

```bash
curl -s -b /tmp/kazi-cookies.txt http://localhost:3001/api/v1/audit/verify
```

Expected:

```json
{
  "status": "verified",
  "checked": 4,
  "ok": true
}
```

Export:

```bash
curl -s -b /tmp/kazi-cookies.txt \
  http://localhost:3001/api/v1/audit/export \
  -o audit.csv
```

Expected: a CSV file with one row per event.

### Flow 7: Connector Test

```bash
curl -s -b /tmp/kazi-cookies.txt -X POST \
  http://localhost:3001/api/v1/connectors/default/test
```

Expected for mock:

```json
{
  "ok": true,
  "message": "MOCK connector readable"
}
```

Expected for Odoo: `ok: true` with an Odoo version and `vendor catalog readable`.

### Flow 8: LLM Settings

Prerequisites: signed in as `ADMIN_EMAIL`.

```bash
curl -s -b /tmp/kazi-cookies.txt http://localhost:3001/api/v1/settings/llm
```

Expected: provider, base URL, model, max tokens, `has_api_key`, and no `api_key`.

```bash
curl -s -b /tmp/kazi-cookies.txt -X POST http://localhost:3001/api/v1/settings/llm/test \
  -H 'Content-Type: application/json' \
  -d '{}'
```

Expected: `{"ok": true, "message": "Provider reachable; model configured as ..."}`.

## 4. Sandbox / Mock Server Testing

The mock adapter runs inside the gateway. There is no separate mock server process.

Start the stack:

```bash
docker compose up -d --build
```

Create a mock connector:

```bash
curl -s -c /tmp/kazi-cookies.txt -X POST http://localhost:3001/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"name":"Sandbox","email":"sandbox@example.com","password":"sandbox-pass"}'

curl -s -b /tmp/kazi-cookies.txt -X POST http://localhost:3001/api/v1/connectors \
  -H 'Content-Type: application/json' \
  -d '{"name":"default","erp_type":"MOCK"}'
```

| Capability | Mock behavior |
|---|---|
| Customers | Seeded catalog with search and ID lookup. |
| Products | Seeded catalog with SKU and search filters. |
| Sale orders | Draft, confirm, deliver, cancel, invoice. |
| Invoices | List, post, credit note, eTIMS, print. |
| Vendors | Seeded vendor catalog. |
| Purchase orders | RFQ, confirm, receive, cancel, vendor bill. |
| Dry runs | Do not persist; live calls do. |

Example read:

```bash
curl -s -b /tmp/kazi-cookies.txt -X POST http://localhost:3001/api/v1/chat \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"Show vendors","conversation_id":"sandbox-read"}'
```

Example write preview:

```bash
curl -s -b /tmp/kazi-cookies.txt -X POST http://localhost:3001/api/v1/chat \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"Create RFQ from ABC for 10 of Chair","conversation_id":"sandbox-write"}'
```

The HTTP endpoint returns the preview but does not hold an approval or execute it. Use the WebSocket or
the UI to complete the approval flow.

## 5. Edge Cases

Empty prompt:

```bash
curl -s -b /tmp/kazi-cookies.txt -X POST http://localhost:3001/api/v1/chat \
  -H 'Content-Type: application/json' \
  -d '{"prompt":"   "}'
```

Expected: `400` with `{"error":"Prompt cannot be empty"}`.

Malformed JSON:

```bash
curl -s -b /tmp/kazi-cookies.txt -X POST http://localhost:3001/api/v1/chat \
  -H 'Content-Type: application/json' \
  -d '{"prompt":'
```

Expected: `400` with `{"error":"Invalid request body"}`.

Duplicate registration:

```bash
curl -s -X POST http://localhost:3001/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"name":"Duplicate","email":"sandbox@example.com","password":"sandbox-pass"}'
```

Expected: `409` with `{"error":"An account with that email already exists"}`.

Weak password:

```bash
curl -s -X POST http://localhost:3001/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"name":"Weak","email":"weak@example.com","password":"short"}'
```

Expected: `400` with `{"error":"Password must be at least 8 characters"}`.

Unauthenticated access:

```bash
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:3001/api/v1/chats
```

Expected: `401`.

Unsupported connector type:

```bash
curl -s -b /tmp/kazi-cookies.txt -X POST http://localhost:3001/api/v1/connectors \
  -H 'Content-Type: application/json' \
  -d '{"name":"bad","erp_type":"ORACLE"}'
```

Expected: `400` with `erp_type must be one of MOCK, ODOO, SAP, DYNAMICS`.

Saved but unimplemented connector:

```bash
curl -s -b /tmp/kazi-cookies.txt -X POST http://localhost:3001/api/v1/connectors \
  -H 'Content-Type: application/json' \
  -d '{"name":"sap","erp_type":"SAP"}'

curl -s -b /tmp/kazi-cookies.txt -X POST http://localhost:3001/api/v1/connectors/sap/test
```

Expected: `ok: false` with an explicit “not implemented” message. The gateway does not fake success.

Duplicate approval:

1. Create a pending RFQ.
2. Send `confirmed` twice.
3. Expected: the first executes; the second is treated as a normal prompt because the pending slot is gone.

Long prompt:

```bash
python3 - <<'PY'
print("x" * 2500)
PY
```

Send the output as the prompt. Expected: the AI engine truncates to 2000 characters before the model call.

## 6. Test Matrix

| # | Scenario | Flow | How to Test | Expected Result |
|---|---|---|---|---|
| 1 | Register account | Auth | `POST /auth/register` with valid fields | `201`, session cookie, user JSON. |
| 2 | Reject duplicate email | Auth | Register the same email twice | `409`, no second account. |
| 3 | Login and logout | Auth | `POST /auth/login`, then `/auth/logout` | `200`; `/auth/me` fails after logout. |
| 4 | Profile update | Auth | `PATCH /auth/profile` | `200`, new name/email persisted. |
| 5 | Read customers | Chat | UI or `/chat` prompt `Show customers` | Table/JSON with customer rows. |
| 6 | Count records | Chat | Prompt `How many customers?` | `COUNT_RECORDS` intent and exact count. |
| 7 | RFQ preview | Chat | Prompt `Create RFQ from ABC for 10 of Chair` | `dry_run` and approval blocks; no persisted order. |
| 8 | Approve RFQ | Chat | Reply `confirmed` | Order created; audit `executed`; activity event. |
| 9 | Reject RFQ | Chat | Reply `no` | No order; audit `rejected`. |
| 10 | Long approval sentence | Chat | `yes, order 10 chairs and confirm it` | Does not match approval; treated as a new prompt. |
| 11 | History persistence | Chat | Reload `/app` or `GET /chats` | Conversation and messages restored. |
| 12 | Tenant isolation | Data | Two accounts, each with chats | Each sees only its own chats and audit. |
| 13 | Audit chain | Audit | `GET /audit/verify` | `status: verified`, `ok: true`, checked count > 0. |
| 14 | Connector test | Connector | `POST /connectors/default/test` | `ok: true` for mock or valid Odoo. |
| 15 | Unsupported ERP | Connector | Save `ORACLE` | `400` validation error. |
| 16 | Unimplemented ERP | Connector | Save and test `SAP` | `ok: false`; explicit error, no mock fallback. |
| 17 | AI rate limit | Chat | Use a very long conversation | `429` with token-budget hint. |
| 18 | Session expiry | Auth | Delete session row or expire cookie | Protected routes return `401`. |

## 7. Troubleshooting

| Symptom | Likely Cause | Fix |
|---|---|---|
| `go test ./...` cannot download modules | No network or proxy. | Check `GOPROXY`, run `go mod download`. |
| AI tests fail on imports | Dependencies not synced. | Run `uv sync` in `ai_engine`. |
| Live AI tests are skipped | `RUN_LLM_TESTS` is unset. | Set `RUN_LLM_TESTS=1` and configure a key. |
| WebSocket test times out | Gateway cannot reach the AI stub or engine. | Check `AI_ENGINE_URL` and logs. |
| `websocat` gets `401` | Missing or expired session cookie. | Pass a fresh `kazi_session` cookie. |
| Approval test finds no pending action | Prompt did not produce a write preview. | Use a write intent such as `Create RFQ...`. |
| Audit verify fails after a test | Test reused a mutated in-memory chain. | Use a fresh store or tenant per test. |
| Connector test fails against Odoo | URL, DB, user, or key is wrong. | Test from the gateway container; use an API key. |
| Frontend build fails | Node version or lockfile mismatch. | Use Node 22 and `npm ci`. |
| No frontend tests run | None are present. | Rely on build, lint, and manual flows. |
