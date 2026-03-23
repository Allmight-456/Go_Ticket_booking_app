# TicketFlow — Production REST API

> **Transformation:** CLI learning exercise → concurrent event-booking backend in Go
> JWT-based RBAC · Conflict detection · Version-controlled audit trail · Rate limiting · Batch operations

---

## What Changed

| Before | After |
|--------|-------|
| CLI, single goroutine, in-memory | REST API on Chi router |
| `[]UserData` slice | PostgreSQL 16 (pgx v5) + Redis 7 |
| No auth | JWT HS256 + bcrypt, admin/user RBAC |
| No concurrency safety | Redis lock + `SELECT FOR UPDATE` + optimistic version |
| No history | Immutable JSONB audit log with field-level diff |
| No rate limiting | Redis sliding-window Lua script (100 req/min) |
| Single `main.go` | Clean Architecture — 25 files across 7 layers |

The old CLI files (`main.go`, `helper.go`, `validate/`) are **dead code** — the new entry point is `cmd/api/main.go`. They can be safely deleted.

---

## Tech Stack

| Concern | Choice | Why |
|---------|--------|-----|
| Router | `go-chi/chi/v5` | stdlib `http.Handler`, no custom context wrapper |
| DB driver | `jackc/pgx/v5` | Native UUID/JSONB/ENUM, pgxpool, Batch API |
| Cache + locks | `redis/go-redis/v9` | Atomic Lua scripts (lock release, rate limit) |
| Auth | `golang-jwt/jwt/v5` + `bcrypt` | Industry standard, constant-time password compare |
| Migrations | `golang-migrate/migrate/v4` | Versioned SQL files, auto-applied at startup |
| Logging | `rs/zerolog` | Zero-alloc structured JSON (pretty in dev) |
| Config | `joho/godotenv` + env vars | Twelve-factor, fails fast on missing required vars |
| Container | Docker multi-stage | ~15 MB final image (build: `golang:1.23-alpine` → run: `alpine:3.20`) |

---

## Quick Start

> **This project's dev workflow:** PostgreSQL + Redis run as Docker containers.
> The Go app runs natively on your machine. Containerise the app only when ready for deployment.

### Step 1 — Infrastructure containers

```bash
# Ports used: postgres→5433, redis→6380
# (avoids conflicts with any existing native postgres/redis on default ports)
cd docker
docker compose up postgres redis -d

# Confirm both are healthy before continuing:
docker compose ps
```

Expected output:
```
NAME                IMAGE              STATUS
docker-postgres-1   postgres:16-alpine Up (healthy)   0.0.0.0:5433->5432/tcp
docker-redis-1      redis:7-alpine     Up (healthy)   0.0.0.0:6380->6379/tcp
```

### Step 2 — Configure environment

```bash
# From project root:
cp .env.example .env
```

The only values you must set (the rest have working defaults):

```env
DATABASE_URL=postgres://ticketflow:secret@localhost:5433/ticketflow?sslmode=disable
REDIS_URL=redis://localhost:6380
JWT_SECRET=<run: openssl rand -hex 32>
```

### Step 3 — Run the Go app

```bash
# First time: download all dependencies (generates go.sum)
go mod tidy

# Start the server (migrations apply automatically on first run)
go run ./cmd/api/...
```

Expected log output:
```
INF postgres pool connected
INF database migrations applied
INF server starting addr=:8080
```

Smoke test:
```bash
curl localhost:8080/health
# {"status":"ok"}
```

### Step 4 — Fully containerised (deployment)

```bash
cd docker && docker compose up --build
# Starts api + postgres + redis, all communicating over Docker's internal network
```

---

## Project Structure

```
ticketflow/
│
├── cmd/api/main.go              ← Entry point: wires config → DB → Redis → services → HTTP server
│
├── internal/                    ← Compiler-enforced: nothing outside this module can import these
│   ├── domain/                  ← Layer 1 — pure structs, sentinel errors, zero logic
│   │   ├── user.go              │  User, Role, UserClaims (JWT claims embed)
│   │   ├── event.go             │  Event, EventStatus enum
│   │   ├── booking.go           │  Booking, BookingStatus enum
│   │   └── audit.go             └  AuditLog, DiffEntry
│   │
│   ├── repository/              ← Layer 2 — data access only, no business rules
│   │   ├── postgres/
│   │   │   ├── db.go            │  pgxpool init + golang-migrate runner
│   │   │   ├── user_repo.go     │  INSERT/SELECT users
│   │   │   ├── event_repo.go    │  CRUD + soft-delete + BatchCreate (pgx Batch)
│   │   │   ├── booking_repo.go  │  CreateInTx, Cancel (with ticket restore), BeginTx
│   │   │   └── audit_repo.go    └  Log, LogInTx (writes inside caller's transaction)
│   │   └── cache/
│   │       └── redis_repo.go    ← AcquireLock/ReleaseLock (Lua) + sliding-window Allow (Lua)
│   │
│   ├── service/                 ← Layer 3 — business logic, orchestrates repos
│   │   ├── auth_service.go      │  Register, Login, bcrypt, JWT issue/validate
│   │   ├── event_service.go     │  CRUD, BatchCreate, cache invalidation, audit
│   │   ├── booking_service.go   │  BookTicket (10-step conflict-safe flow), CancelBooking
│   │   └── audit_service.go     └  Log, LogInTx, ComputeDiff, ToMap
│   │
│   ├── handler/                 ← Layer 4 — HTTP only: decode request, call service, render JSON
│   │   ├── helper.go            │  render(), renderError(), decode()
│   │   ├── auth_handler.go      │  POST /auth/register, POST /auth/login
│   │   ├── event_handler.go     │  CRUD /events, POST /events/batch
│   │   ├── booking_handler.go   │  POST /bookings, DELETE /bookings/:id, POST /bookings/batch
│   │   └── audit_handler.go     └  GET /audit/:resource_type/:resource_id
│   │
│   ├── middleware/
│   │   ├── auth.go              │  Parse Bearer JWT → inject UserClaims into context
│   │   ├── rbac.go              │  RequireAdmin / RequireRole
│   │   └── rate_limiter.go      └  Redis sliding-window, 100 req/min per IP
│   │
│   └── router/router.go         ← Chi router: public / auth / admin route groups
│
├── migrations/                  ← Plain SQL, run once at startup
│   ├── 001_create_users.up.sql
│   ├── 002_create_events.up.sql
│   ├── 003_create_bookings.up.sql
│   └── 004_create_audit_logs.up.sql
│
├── docker/
│   ├── Dockerfile               ← Multi-stage build
│   └── docker-compose.yml       ← App + postgres (5433) + redis (6380) with health checks
│
├── .env / .env.example
├── go.mod / go.sum
├── MEMORY.md                    ← Project context for AI agents
└── API_GUIDE.md                 ← Step-by-step API walkthrough + troubleshooting
```

---

## Database

### Schema

```
users ──────────┐
                │ FK: created_by
events ─────────┤
                │ FK: event_id, user_id
bookings ───────┤
                │
audit_logs ─────┘  (actor_id nullable — system events have no user)
```

### Key design decisions

| Decision | Why |
|----------|-----|
| UUID primary keys | No sequential ID guessing; safe to expose in URLs |
| `TIMESTAMPTZ` everywhere | Always UTC; eliminates timezone ambiguity |
| Price as `INT` cents (never `FLOAT`) | Floating-point arithmetic on money causes rounding errors |
| Soft-delete on events (`deleted_at`) | Historical bookings keep their FK reference |
| `version INT` on events + bookings | Optimistic locking — concurrent updates detected without table locks |
| Partial unique index on bookings | One active booking per user per event; re-booking allowed after cancellation |
| `JSONB` audit columns + GIN index | Fast containment queries: `diff @> '{"price_cents": {}}'` |

### `.up.sql` — why that filename?

`golang-migrate` uses versioned pairs:
```
001_create_users.up.sql    ← applied when migrating UP (first run, CI)
001_create_users.down.sql  ← applied when rolling BACK (not created here)
```

A `schema_migrations` table is auto-created in your DB. Running the app a second time is safe — already-applied migrations are skipped.

### Connecting to the DB directly

```bash
# Dev (container on port 5433):
docker compose exec postgres psql -U ticketflow -d ticketflow

# Useful queries:
\dt                                       -- list tables
SELECT email, role FROM users;
SELECT name, remaining_tickets, version FROM events;
SELECT * FROM audit_logs ORDER BY created_at DESC LIMIT 10;
```

---

## Booking Conflict Detection — The 10-Step Flow

When multiple users race to book the last ticket, this is what happens:

```
N requests → POST /bookings
      │
      ▼
[Rate Limiter]         Redis sliding-window: 100 req/min per IP
      │
      ▼
[JWT Auth]             Validate token → inject UserClaims into context
      │
      ▼
AcquireLock()          Redis SET NX "lock:event:<id>" <token> PX 5000
      │                Only ONE goroutine proceeds. Others → 503 immediately.
      │
      ▼ (1 goroutine)
BeginTx()              PostgreSQL transaction
      │
      ▼
SELECT FOR UPDATE      Row-level lock on the event row — serialises any
      │                goroutines that slipped past the Redis lock
      ▼
Validate               remaining_tickets ≥ requested AND status = 'published'
      │
      ▼
INSERT booking         Partial unique index blocks duplicate active bookings
UPDATE events          remaining -= N, version = version+1 WHERE version = $old
INSERT audit_log       Same TX — log commits or rolls back with the booking
      │
      ▼
COMMIT
      │
      ▼
ReleaseLock() Lua      Only releases if stored token = our token (atomic)
InvalidateCache()      Forces next GET /events/:id to re-read from DB
```

---

## Go Patterns — C and Python Analogies

| Go feature | C analogy | Python analogy |
|-----------|-----------|----------------|
| `defer tx.Rollback()` | `goto cleanup` at function end | `with` statement / `__exit__` |
| Multiple return `(val, err)` | Output pointer `int fn(T* out)` | Tuple return `return val, err` |
| Interface implicit satisfaction | Manual vtable (fn pointers in struct) | Duck typing — but compile-time |
| `context.Context` propagation | `select()` / `poll()` on fd | `asyncio` task cancellation |
| `fmt.Errorf("wrap: %w", err)` | `errno` (loses context) | `raise X from Y` chaining |
| Struct tags `json:"field"` | Manual serialisation code | Pydantic `Field()` / `dataclass` |
| `internal/` directory | `static` (file-scoped) | `_module.py` (convention only) |
| Goroutines | `pthread_create` (heavier, fixed stack) | `threading.Thread` (GIL limits true parallelism) |

---

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | ✅ | — | `postgres://ticketflow:secret@localhost:5433/ticketflow?sslmode=disable` |
| `JWT_SECRET` | ✅ | — | ≥ 32 chars — `openssl rand -hex 32` |
| `REDIS_URL` | — | `redis://localhost:6379` | Update to `6380` for local dev |
| `PORT` | — | `8080` | HTTP server port |
| `JWT_EXPIRATION` | — | `24h` | Go duration string: `1h`, `7d`, etc. |
| `APP_ENV` | — | `development` | Set `production` for JSON (non-pretty) logs |
| `MIGRATIONS_PATH` | — | `migrations` | Override migrations directory path |

---

## Promoting a User to Admin

Newly registered users are always `role=user`. Promote via psql:

```bash
docker compose exec postgres psql -U ticketflow -d ticketflow \
  -c "UPDATE users SET role='admin' WHERE email='your@email.com';"
```

Then log in again — the new JWT will carry `"role":"admin"`.

---

*Module: `github.com/Allmight-456/ticketflow` — built as a portfolio/learning project.*
*See `API_GUIDE.md` for step-by-step curl walkthrough and troubleshooting.*
