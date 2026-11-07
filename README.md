# Obedman portfolio

Astro/React/Tailwind portfolio with a Go HTTP server and PostgreSQL guestbook. Existing portfolio content and teal/slate theme are retained. The Go server serves the static build and a same-origin API; new messages are private until a host operator approves them.

## Runtime

Use **Node 24.21.0**, **Go 1.27.3** (see `.node-version`, `dev/package.json`, `api/go.mod`) and PostgreSQL 16. Dependencies are locked in `dev/package-lock.json` and `api/go.sum`. No latency, uptime or production security guarantee is inferred from this setup.

## Run locally

Create a dedicated development database and role. Copy the configuration keys in [.env.example](.env.example) into your shell or process manager; Go deliberately does not auto-load dotenv files. Never put real credentials in source or commands saved in shell history.

```sh
# Bash; use your secret manager to supply DATABASE_URL.
export APP_ENV=development
export DATABASE_URL   # PostgreSQL URL, explicit sslmode; see .env.example
cd dev
npm ci
npm run build
cd ../api
go run . migrate       # Explicit schema migration, using a DDL-capable role.
go run .               # Default 127.0.0.1:3030; run as the restricted runtime role.
```

In PowerShell set `$env:APP_ENV='development'` and supply `$env:DATABASE_URL` securely; use `npm.cmd` if the PowerShell execution policy blocks npm's wrapper. Run all Go files with `go run .`, not `go run main.go`.

For frontend editing, from `dev/` run `npm run dev -- --background`. The Astro Vite proxy forwards `/api` to `127.0.0.1:3030`; stop it with `npm exec astro dev stop`. Production uses one origin through Go, so permissive CORS is unnecessary. The Go static directory defaults to `../dev/dist` relative to its working directory; set `STATIC_DIR` explicitly in services.

## API contract

| Route | Behavior |
| --- | --- |
| `GET /api/live` | Process liveness; no database dependency |
| `GET /api/ready` | Database query and expected schema available; 503 otherwise |
| `GET /api/guestbook?limit=20&cursor=…` | Approved entries only, `{items: [], next_cursor: ""}`; limit 1–50 |
| `POST /api/guestbook` | JSON `{name, message}` only; 201 after insert, server ID/time and `pending` status |

Unknown fields, including `id`, `created_at` and `status`, are rejected. Names are 1–80 Unicode code points and messages 1–2000 after trimming. The body limit is 16 KiB. Errors use `{error: {code, message}}`, with a request ID response header. Internal database errors and message bodies are not logged.

The public API format intentionally changes from an unbounded array to a page envelope. Deploy the matching frontend and backend together. No public admin endpoint exists. See [operations](handbook/operations.md) for moderation, roles, migration, deployment, rollback and restore.

## Verify

```sh
cd dev && npm ci && npm audit --audit-level=low && npm run build
cd ../api
gofmt -l .
go vet ./...
go test ./...
# For the real PostgreSQL suite, set TEST_DATABASE_URL to a disposable database ending in _test.
go test -race -count=1 ./...
go build -o ../.local/obedman-api .
go install golang.org/x/vuln/cmd/govulncheck@v1.9.0
govulncheck ./...
```

Browser tests require the built site served by Go on `127.0.0.1:3030`:

```sh
cd dev
npx --no-install playwright install chromium
npm test
```

Enable the actual database/browser test with `RUN_DATABASE_E2E=1`, `API_BINARY` pointing to the built binary and identical `DATABASE_URL`/`TEST_DATABASE_URL` targeting the disposable `_test` database. Without that flag two real-flow browser cases are explicitly skipped. PostgreSQL integration tests also explicitly skip when their test URL is absent; a passing unit-only run is not database evidence.

From the repository root, run `python scripts/scan-secrets.py` and `python scripts/gitleaks-check.py` (Gitleaks 8.30.1 on PATH). For a synthetic restore drill set loopback `PGHOST`, `PGPORT`, `PGUSER`, optional `PGPASSFILE`, and `API_BINARY`, then run `python scripts/restore-drill.py`. It creates and removes only its own unique databases. CI repeats these checks on an ephemeral PostgreSQL service.

## Evidence and remaining work

- [Verification record](handbook/verification.md): actual local results and limitations.
- [Engineering case study](handbook/case-study.md): problem, decisions, trade-offs and evidence.
- [Security status](SECURITY.md): **historical database credential remains exposed; rotation is unverified**. Removing it from active source does not revoke it.
- [Architecture and threats](handbook/architecture.md), [operations](handbook/operations.md).

Production deployment, production TLS/role verification, credential rotation, off-host backup delivery and Git hosting history cleanup require the owner's infrastructure access. They have not been claimed as completed.
