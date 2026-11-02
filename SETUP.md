# Kazi AI (ERPAgent) Setup

Install, configure, and verify a local or production ERPAgent deployment.

## 1. Prerequisites

| Requirement | Version | Notes |
|---|---|---|
| Go | `1.26.5` | Gateway build and tests. |
| Node.js | `22` recommended, `20+` minimum | Frontend build and dev server. |
| Python | `3.14` | AI engine runtime. |
| uv | Current | Installs the AI engine from `pyproject.toml` and `uv.lock`. |
| Docker Engine | Current | Optional, but recommended for Postgres, Redis, and full-stack runs. |
| Docker Compose | v2 | `docker compose` command, not legacy `docker-compose`. |
| Postgres | 15+ with pgvector | The compose image is `ankane/pgvector:v0.5.1`. |
| Redis | 7+ | Pending approvals, history window, activity fan-out. |
| Odoo | Any version with `/jsonrpc` | Optional. Use `MOCK` for local work. |
| LLM provider key | OpenAI-compatible | Groq is the default. |

Ports used in development:

| Port | Service |
|---|---|
| `3000` | Next.js frontend. |
| `3001` | Go gateway. |
| `8000` | AI engine FastAPI. |
| `50051` | AI engine gRPC reservation. |
| `5432` | Postgres. |
| `6379` | Redis. |
| `8200` | Vault dev server. |

## 2. Configuration Steps

### Step 1: Clone the Repository

```bash
git clone https://github.com/wycliff-ochieng/kaziAI.git
cd kaziAI
```

### Step 2: Create Environment Files

```bash
cp deploy/.env.example .env
cp ai_engine/.env.example ai_engine/.env
```

Edit `.env` for the gateway and compose stack. Edit `ai_engine/.env` for direct AI engine runs. Never commit
either file.

| File | Used by | Required |
|---|---|---|
| `.env` | Docker Compose and gateway environment | Yes for Docker. |
| `ai_engine/.env` | AI engine when run directly, and the dev compose `env_file` | Yes for direct runs. |
| `frontend/.env.local` | Frontend overrides | No. |

### Step 3: Configure Core Gateway Values

| Variable | Required | Example | Notes |
|---|---|---|---|
| `PORT` | No | `3000` | Gateway listen port. |
| `ENVIRONMENT` | Yes | `development` | `production` enables fail-fast and secure cookies. |
| `ADMIN_EMAIL` | Production | `admin@example.com` | Must match the account that manages LLM settings. |
| `DATABASE_URL` | Yes | `postgres://postgres:postgres@postgres:5432/erpagent?sslmode=disable` | Use the compose service name inside Docker. |
| `REDIS_URL` | Yes | `redis:6379` | Use `localhost:6379` outside Docker. |
| `AI_ENGINE_URL` | Yes | `http://ai_engine:8000` | Use `http://localhost:8000` outside Docker. |
| `AI_ENGINE_GRPC` | No | `ai_engine:50051` | Reserved. |
| `DEFAULT_ERP_TYPE` | No | `ODOO` | `MOCK` is safest for a first run. |
| `INTERNAL_API_TOKEN` | Production | `openssl rand -hex 32` output | Shared gateway-to-AI secret. |
| `SETTINGS_ENCRYPTION_KEY` | Production | `openssl rand -base64 32` output | Encrypts stored LLM keys. |

Generate production secrets:

```bash
openssl rand -hex 32   # INTERNAL_API_TOKEN
openssl rand -base64 32 # SETTINGS_ENCRYPTION_KEY
```

### Step 4: Configure the ERP Connector

Use a mock connector first. This requires no Odoo instance.

| Field | Required | Value |
|---|---|---|
| `name` | No | `default` |
| `erp_type` | Yes | `MOCK` |
| `base_url` | No | Empty |
| `db_name` | No | Empty |
| `username` | No | Empty |
| `password` | No | Empty |

For Odoo:

| Field | Required | Example |
|---|---|---|
| `name` | No | `default` |
| `erp_type` | Yes | `ODOO` |
| `base_url` | Yes | `http://odoo:8069` |
| `db_name` | Yes | `erp` |
| `username` | Yes | `api@example.com` |
| `password` | Yes | API key or password |

Use a network address reachable from the gateway container. Do not use `host.docker.internal` in
production. Prefer an Odoo API key over a human password.

You can configure the connector from the workspace UI or with the API:

```bash
curl -s -X POST http://localhost:3001/api/v1/connectors \
  -H 'Content-Type: application/json' \
  -H 'X-Tenant-ID: dev-tenant' \
  -d '{
    "name": "default",
    "erp_type": "ODOO",
    "base_url": "http://10.0.0.10:8069",
    "db_name": "erp",
    "username": "api@example.com",
    "password": "replace-me"
  }'
```

The `X-Tenant-ID` fallback only works outside production. In production, sign in first and use the session
cookie.

### Step 5: Configure the LLM

The AI engine uses an OpenAI-compatible provider. Groq is the default.

| Variable | Required | Default | Notes |
|---|---|---|---|
| `LLM_PROVIDER` | No | `groq` | Provider label. |
| `LLM_BASE_URL` | No | `https://api.groq.com/openai/v1` | Must be an absolute URL. |
| `LLM_MODEL` | No | `llama3-70b-8192` | Provider model name. |
| `LLM_API_KEY` | Yes | empty | Provider key. |
| `LLM_MAX_TOKENS` | No | `400` | Clamped to `1..8192`. |

The UI-managed values in Postgres override these environment defaults after the first save. Sign in with
the `ADMIN_EMAIL` account, open the workspace, and select **Settings** in the left icon rail.

| UI field | Required | Notes |
|---|---|---|
| Provider | No | Free-form label. |
| Base URL | Yes | Example: `https://api.groq.com/openai/v1`. |
| Model | Yes | Example: `llama3-70b-8192`. |
| API key | No | Empty keeps the existing saved key. |
| Max tokens | No | Use `400` for small-context models. |

Use **Test** to probe the provider before saving. Saving pushes the configuration to the AI engine; a push
failure returns a warning but keeps the saved value.

### Step 6: Choose a Run Mode

#### Option A: Docker Development Stack

```bash
docker compose up -d --build
```

This starts Postgres, Redis, Vault, the gateway on `3001`, and the AI engine on `8001`. It does **not**
start the frontend.

Check health:

```bash
docker compose ps
curl -s http://localhost:3001/api/v1/health
curl -s http://localhost:8001/health
```

#### Option B: Local Processes

Start Postgres and Redis first:

```bash
docker compose up -d postgres redis
```

Gateway:

```bash
cd backend
go mod download
PORT=3000 \
ENVIRONMENT=development \
DATABASE_URL='postgres://postgres:postgres@localhost:5432/erpagent?sslmode=disable' \
REDIS_URL='localhost:6379' \
AI_ENGINE_URL='http://localhost:8000' \
DEFAULT_ERP_TYPE='MOCK' \
go run ./cmd/main.go
```

AI engine:

```bash
cd ai_engine
uv sync
.venv/bin/fastapi run src/main.py --port 8000
```

The AI engine pulls `/api/v1/internal/llm-config` from the gateway at boot and retries eight times. Start
the gateway first if you want the saved model configuration to apply immediately.

Frontend:

```bash
cd frontend
npm install
npm run dev
```

Open `http://localhost:3000`.

### Step 7: Create the First Account

1. Open `http://localhost:3000/auth/register`.
2. Enter a name, email, and password of at least 8 characters.
3. Submit. The gateway creates a new tenant and sets the `kazi_session` cookie.
4. The workspace opens at `/app`.

If `ADMIN_EMAIL` is set, register with that exact email to manage LLM settings. Existing accounts can
update their email from **Profile** in the workspace.

### Step 8: Verify the Workspace

1. Confirm the **Connected** badge appears in the workspace header.
2. Ask `Show customers`.
3. Confirm a response appears and a conversation is listed under **Chat history**.
4. Ask `Create RFQ from ABC for 10 of Chair`.
5. Confirm a dry-run card appears under **Approvals**.
6. Reply `confirmed` and verify the result and the **Activity** entry.
7. Open **Audit Trail** and confirm the turn is listed.

### Step 9: Production Deployment

1. Point a DNS `A`/`AAAA` record at the server.
2. Open ports `80` and `443`.
3. Copy `deploy/.env.example` to `.env` on the server.
4. Set `DOMAIN`, `ADMIN_EMAIL`, `DATABASE_URL`, `POSTGRES_PASSWORD`, `REDIS_URL`, `ODOO_*`,
   `INTERNAL_API_TOKEN`, and `SETTINGS_ENCRYPTION_KEY`.
5. Start the stack.

   ```bash
   docker compose -f docker-compose.prod.yml up -d --build
   ```

6. Confirm Caddy obtained a certificate and the site loads at `https://${DOMAIN}`.

Production behavior:

- Caddy routes `/api/*` to `go_backend:3000` and everything else to `frontend:3000`.
- Only Caddy publishes ports. Postgres, Redis, the gateway, and the AI engine stay on `erp_net`.
- The gateway refuses to start if Postgres or Redis is unavailable.
- `INTERNAL_API_TOKEN` and `SETTINGS_ENCRYPTION_KEY` are required by compose interpolation.
- Cookies are `Secure` because `ENVIRONMENT=production`.

### Step 10: Back Up and Verify

```bash
# Database backup
docker compose -f docker-compose.prod.yml exec -T postgres \
  pg_dump -U postgres erpagent > erpagent-$(date +%F).sql

# Audit chain verification
curl -s https://${DOMAIN}/api/v1/audit/verify \
  -H "Cookie: kazi_session=<session-token>"
```

Back up Postgres and the Caddy data volume. Test a restore before relying on the backup.

## 3. Running Tests

Gateway:

```bash
cd backend
go test ./...
go vet ./...
```

AI engine:

```bash
cd ai_engine
uv run pytest
```

Run live LLM integration tests only when a provider key is configured:

```bash
cd ai_engine
RUN_LLM_TESTS=1 uv run pytest -m integration
```

Frontend:

```bash
cd frontend
npm run build
npm run lint
```

Compose validation:

```bash
docker compose config
docker compose -f docker-compose.prod.yml config
```

Container builds:

```bash
docker build -t erpagent-backend ./backend
docker build -t erpagent-ai ./ai_engine
docker build -t erpagent-frontend ./frontend
```

## 4. Troubleshooting

| Symptom | Likely Cause | Fix |
|---|---|---|
| `docker compose up` fails on `ai_engine/.env` | The env file is missing. | `cp ai_engine/.env.example ai_engine/.env`. |
| Gateway exits in production | Postgres or Redis unavailable. | Check `docker compose ps` and logs; restore dependencies. |
| `ADMIN_EMAIL` settings disabled | No admin email is configured. | Set `ADMIN_EMAIL` to the signed-in account email and restart. |
| LLM settings save returns a warning | AI engine did not accept the push. | Check `AI_ENGINE_URL`, `INTERNAL_API_TOKEN`, and AI logs. |
| Odoo connector test fails | Wrong URL, DB, user, or key. | Test from the gateway container: `docker compose exec go_backend wget -qO- http://odoo:8069/web/database/list`. |
| Odoo actions use mock data | `ODOO_URL` or `ODOO_DB` is empty. | Set both or save a complete `default` connector. |
| Frontend gets `Failed to fetch` | Gateway is not on `3001`, or WS URL is wrong. | Start the gateway on `3001` or set `NEXT_PUBLIC_GATEWAY_WS`. |
| Login succeeds but `/app` redirects | Session cookie is not being sent. | Use the same origin; check `Secure` and proxy headers. |
| Caddy cannot issue a certificate | DNS or ports are wrong. | Verify DNS, ports `80`/`443`, and Caddy logs. |
| Postgres password changed but app fails | `DATABASE_URL` still has the old password. | Update `DATABASE_URL` and restart the gateway. |
