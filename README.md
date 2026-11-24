# Kazi AI (ERPAgent)

A chat workspace for ERP operations: ask for customers, orders, invoices, or stock in plain language, and
change ERP data through previews a person approves.

## Why

Finding "who owes us money" or raising a purchase order means knowing which ERP screen holds it. Kazi AI
turns the request into typed, tenant-scoped reads and dry-run writes against the ERP. A write never reaches
the ERP until someone replies `confirmed`, and every turn is hash-chained in Postgres so the trail can be
verified afterwards.

Three services make that work: a Next.js workspace, a Go gateway that owns authentication, ERP access,
approval, and audit, and a Python AI engine that turns language into intents and tool calls.

## Quickstart

### Prerequisites

| Requirement | Notes |
|---|---|
| Docker Engine with Compose v2 | Runs the gateway, AI engine, Postgres, Redis, and Vault. |
| Node.js 20.9 or newer | Next.js 16 requires it. |
| An OpenAI-compatible LLM API key | Groq's free tier works and is the default provider. |

### 1. Clone and configure

```bash
git clone https://github.com/wycliff-ochieng/kaziAI.git
cd kaziAI
cp ai_engine/env.example ai_engine/.env
```

Open `ai_engine/.env` and set `LLM_API_KEY` to your provider key. The placeholder value will not
authenticate; the AI engine reads this file at startup.

### 2. Start the backend stack

```bash
docker compose up -d --build
```

The first build takes a few minutes. Wait until the gateway container is running, then check it:

```bash
curl -s http://localhost:3001/api/v1/health
```

```json
{"service":"ERPAgent Gateway (Gorilla Mux)","status":"up"}
```

### 3. Start the workspace

In a second terminal, from the repository root:

```bash
cd frontend
npm install
npm run dev
```

Open `http://localhost:3000`, register an account, and ask `Show customers`. The answer comes from the
built-in mock ERP because no Odoo credentials are configured yet.

## Usage

Once you are signed in, the workspace opens on **Chat**. It talks to the gateway over one WebSocket and
reads the mock ERP until you configure Odoo.

Ask a question. Type `Show customers`. The agent returns the three mock customers in a table you can switch
to a chart.

Ask for a change. Type `Create RFQ from ABC for 10 of Chair`. The agent replies with a dry-run card: vendor,
product, quantity, totals, a risk level, and a confirmation question. Nothing has been written.

Approve it. Reply `confirmed` in the chat box. The gateway executes the held action, streams the created RFQ,
and records the turn under **Audit Trail**. The message must be a short approval such as `confirmed`, `ok`,
`yes`, `proceed`, or `go ahead`; a sentence that merely contains one of those words is treated as a new
request.

The same data is available over REST without the agent:

```bash
curl -s http://localhost:3001/api/v1/sales/customers -H 'X-Tenant-ID: dev-tenant'
```

```json
{
  "status": "success",
  "count": 3,
  "data": [
    {"id": "10", "name": "Azure Interior", "email": "info@azure.example.com", "phone": "+254 700 123456", "credit_limit": 15000, "balance": 2500, "city": "Nairobi"},
    {"id": "12", "name": "Deco Addict", "email": "contact@decoaddict.com", "phone": "+254 711 987654", "credit_limit": 25000, "balance": 0, "city": "Mombasa"},
    {"id": "15", "name": "Gemini Furniture", "email": "sales@gemini.co.ke", "phone": "+254 722 555444", "credit_limit": 50000, "balance": 12000, "city": "Eldoret"}
  ]
}
```

`X-Tenant-ID` is a development fallback; in production the gateway resolves the tenant from the session
cookie. Point `ODOO_URL` and `ODOO_DB` at a real Odoo instance to read live records. The WebSocket frames,
the approval flow, and the full route list are documented in [INTEGRATION.md](INTEGRATION.md).

## Configuration

The defaults are chosen for local development. For a server, copy `deploy/env.example` to `.env`, fill it
in, and follow [DEPLOYMENT.md](DEPLOYMENT.md).

| Key | Default | What it changes |
|---|---|---|
| `LLM_API_KEY` in `ai_engine/.env` | placeholder | Provider key used by the AI engine. |
| `LLM_PROVIDER` | `groq` | Provider label. |
| `LLM_MODEL` | `llama3-70b-8192` | Model name sent with each turn. |
| `DEFAULT_ERP_TYPE` | `ODOO` in Compose | Connector used when a request names none. Falls back to the mock adapter if `ODOO_URL` or `ODOO_DB` is empty. |
| `ODOO_URL`, `ODOO_DB`, `ODOO_USER`, `ODOO_PASSWORD` | empty | Odoo connection used by the gateway. |
| `ADMIN_EMAIL` | empty | Account allowed to change deployment-wide LLM settings. |
| `INTERNAL_API_TOKEN` | `dev-internal-token` in Compose | Shared secret between the gateway and the AI engine. Required in production. |
| `SETTINGS_ENCRYPTION_KEY` | `dev-only-encryption-key` in Compose | Encrypts LLM keys stored in Postgres. Required in production. |
| `DATABASE_URL`, `REDIS_URL` | Compose service names | Postgres and Redis connection strings. |

LLM settings and ERP connectors can also be changed from the workspace and are stored in Postgres, where
they override the environment fallbacks. The full variable reference, Odoo setup, and verification steps
are in [SETUP.md](SETUP.md).

## Going deeper

| Document | Contents |
|---|---|
| [INTEGRATION.md](INTEGRATION.md) | Architecture, data flows, canonical models, API contract, security boundaries. |
| [SETUP.md](SETUP.md) | Full configuration, Odoo connection, manual verification. |
| [TESTING.md](TESTING.md) | Test suites, sandbox flows, edge cases. |
| [DEPLOYMENT.md](DEPLOYMENT.md) | Docker Compose and Kubernetes production deployment. |
| [RUNBOOK.md](RUNBOOK.md) | Rollout, rollback, backup, and incident procedures. |

## Contributing

Run the suites before sending a patch:

```bash
cd backend && go test ./...
cd ../ai_engine && uv run pytest
cd ../frontend && npm run build
```

The frontend has no automated tests; `npm run build` is its check. Open an issue before a large change, keep
patches focused, and add a test wherever behavior changes. [TESTING.md](TESTING.md) describes the suites and
manual flows.

## License

No license file is present. Treat the code as proprietary until a license is added.
