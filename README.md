# Web project template

Starting point for a web project: a **Go API** (Gin, Postgres via pgx, goose migrations) and a **static SvelteKit frontend** (SvelteKit 3, Svelte 5 runes, `adapter-static`) that calls it from the browser.

It ships two working examples, so both sides have a pattern to copy:

| Example         | Backend                                                                                                                      | Frontend                                                                          |
| --------------- | ---------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------- |
| Health check    | `routes/health.go`, no database                                                                                              | `src/lib/health/health.ts`, `src/lib/components/HealthStatus.svelte`              |
| Visitor counter | `counter` (SQL store), `countcache`, `events` (SSE hub), `limiter`, `routes/visitors.go`, `db/migrations/*_create_counters.sql` | `src/lib/visitors/visitors.svelte.ts`, `src/lib/components/VisitorCounter.svelte` |

The counter is the reference for adding a database-backed resource: a store over Postgres, an in-memory cache, live updates over Server-Sent Events, per-client rate limits and connection caps, with tests at every layer. Delete it once your own resources follow the same shape.

## Starting a new project

1. Copy this repository into a new folder and start a fresh git history there.
2. Rename:
   - [ ] Go module `app/backend` in `backend/go.mod` and in every import:
     ```bash
     grep -rl "app/backend" backend --include=*.go | xargs sed -i 's#app/backend#myproject/backend#g'
     ```
   - [ ] Binary `/server` in `backend/Dockerfile` and `backend/.gitignore`
   - [ ] Database name in `backend/.env.example`, and `app_test` in `backend/counter/counter_test.go`
   - [ ] `name` in `frontend/package.json` (then `npm install` to update the lock file)
   - [ ] Page title and text in `frontend/src/routes/+page.svelte`, icon in `frontend/src/lib/assets/favicon.svg`
   - [ ] This README
3. Remove the visitor counter when you no longer need it as a reference: the files in the table above, their tests, and `frontend/tests/counter.spec.ts`.

## Prerequisites

- Go (version in `backend/go.mod`)
- Node.js and npm
- A running Postgres. The database named in `DATABASE_URL` must exist, e.g. `psql -U postgres -c "CREATE DATABASE app;"`
- Docker, only for the `counter` tests (testcontainers) and the backend image

## Configuration

Each side reads a git-ignored `.env`; copy its `.env.example`.

**Backend** (`backend/.env`; real environment variables take precedence; migrations run on startup):

| Variable             | Required | Description                                                                             |
| -------------------- | -------- | --------------------------------------------------------------------------------------- |
| `DATABASE_URL`       | yes      | Postgres connection string; use `sslmode=verify-full` for a remote database             |
| `ALLOWED_ORIGINS`    | yes      | Comma-separated browser origins allowed cross-origin, e.g. `http://localhost:5173`      |
| `TRUSTED_PROXIES`    | no       | Proxy IPs/CIDRs whose `X-Forwarded-For` is trusted for the client IP; empty trusts none |
| `PORT`               | no       | Listen port, default `8080`                                                             |
| `LOG_CLIENT_HEADERS` | no       | `true` logs full client addresses and forwarding headers, to work out `TRUSTED_PROXIES` |

**Frontend** (`frontend/.env`; read at build time and baked into the static pages):

| Variable         | Required | Description                                                                          |
| ---------------- | -------- | ------------------------------------------------------------------------------------ |
| `PUBLIC_API_URL` | yes      | Base URL of the backend, e.g. `http://localhost:8080`; also the CSP's `connect-src` |

## API

All under `/api/v1`.

| Method | Path               | Description                                                                                             |
| ------ | ------------------ | ------------------------------------------------------------------------------------------------------- |
| GET    | `/health`          | `200 {"message":"Server OK"}` while the process serves requests (liveness, no database call)            |
| GET    | `/visitors`        | `200 {"count": n}` from the in-memory cache                                                             |
| POST   | `/visitors`        | Increments and returns `{"count": n}`; `429` within 10 min per client; `403` from a non-allowed origin |
| GET    | `/visitors/stream` | `text/event-stream` with a `count` event on connect and after every increment                          |

Non-GET requests need an allowed `Origin` header. The event hub, cache and limiters live in process memory, which suits a single instance.

## Run and test

Commands are in [backend/README.md](backend/README.md) and [frontend/README.md](frontend/README.md). The frontend's Playwright tests mock the API, so they need no backend.
