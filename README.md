# TicketFlow — Event Management & Ticket Booking REST API

A production-grade REST API backend for managing events and booking tickets, built in Go with Clean Architecture.

> JWT-based RBAC · Concurrency-safe booking · Optimistic locking · Immutable audit trail · Redis rate limiting · Batch operations

---

## Features

- **Event Management** — Create, update, soft-delete, and list events (admin-only writes). Supports batch creation of up to 50 events in a single atomic round-trip via pgx's Batch API.
- **Ticket Booking** — Users book tickets through a 10-step conflict-safe flow combining a Redis distributed lock, a PostgreSQL `SELECT FOR UPDATE`, and an optimistic version check. Cancellation automatically restores ticket inventory.
- **Authentication & RBAC** — JWT HS256 tokens issued on register/login. Two roles: `user` (read + book) and `admin` (full event write access + audit trail). Passwords hashed with bcrypt.
- **Audit Trail** — Every create/update/delete on events and bookings is recorded in an immutable JSONB log with field-level diffs, actor ID, and client IP. Admin-only via `GET /audit/{type}/{id}`.
- **Rate Limiting** — Redis sliding-window Lua script enforces 100 requests per minute per IP across all endpoints.
- **Structured Logging** — zerolog JSON output; pretty-printed in `development`, compact in `production`.

---

## Tech Stack

| Concern | Library | Why |
|---------|---------|-----|
| Router | `go-chi/chi/v5` | Lightweight, stdlib-compatible `http.Handler`; no custom context wrapper |
| DB driver | `jackc/pgx/v5` | Native UUID/JSONB/ENUM support, `pgxpool`, Batch API |
| Cache + locks | `redis/go-redis/v9` | Atomic Lua scripts for lock release and rate limiting |
| Auth | `golang-jwt/jwt/v5` + `bcrypt` | Industry-standard JWT; constant-time password comparison |
| Migrations | `golang-migrate/migrate/v4` | Versioned SQL files auto-applied at startup |
| Logging | `rs/zerolog` | Zero-alloc structured JSON |
| Config | `joho/godotenv` + env vars | Twelve-factor; fails fast on missing required vars |
| Container | Docker multi-stage | ~15 MB final image (`golang:1.23-alpine` → `alpine:3.20`) |

---

## Quick Start

> **Dev workflow:** PostgreSQL + Redis run as Docker containers. The Go app runs natively on your machine. Containerise the app only for deployment.

### 1 — Start infrastructure

```bash
cd docker
docker compose up postgres redis -d
```

Wait for both to be healthy (~5 s):
```bash
docker compose ps
```
```
NAME                IMAGE              STATUS
docker-postgres-1   postgres:16-alpine Up (healthy)   0.0.0.0:5433->5432/tcp
docker-redis-1      redis:7-alpine     Up (healthy)   0.0.0.0:6380->6379/tcp
```

Ports `5433` (postgres) and `6380` (redis) are intentionally offset to avoid collisions with any locally installed instances on their default ports.

### 2 — Configure environment

```bash
cp .env.example .env
```

Minimum required values:

```env
DATABASE_URL=postgres://ticketflow:secret@localhost:5433/ticketflow?sslmode=disable
REDIS_URL=redis://localhost:6380
JWT_SECRET=<run: openssl rand -hex 32>
```

### 3 — Run the API

```bash
# First run: fetch all dependencies
go mod tidy

# Start (database migrations apply automatically)
go run ./cmd/api/...
```

Expected startup:
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

### 4 — Fully containerised (deployment)

```bash
cd docker && docker compose up --build
```

Starts `api` + `postgres` + `redis` on Docker's internal network.

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
│   └── router/router.go         ← Chi router: public / authenticated / admin route groups
│
├── migrations/                  ← Plain SQL, versioned pairs (up only — no down migrations created)
│   ├── 001_create_users.up.sql
│   ├── 002_create_events.up.sql
│   ├── 003_create_bookings.up.sql
│   └── 004_create_audit_logs.up.sql
│
├── docker/
│   ├── Dockerfile               ← Multi-stage build
│   ├── docker-compose.yml       ← postgres (5433) + redis (6380) + optional pgAdmin (5050)
│   └── PGADMIN_GUIDE.md         ← Optional: browser-based DB GUI setup instructions
│
├── .env / .env.example
├── go.mod / go.sum
└── API_GUIDE.md                 ← Complete endpoint reference + curl walkthrough + troubleshooting
```

---

## API Reference

See **[API_GUIDE.md](./API_GUIDE.md)** for the complete endpoint reference, authentication setup, curl examples for every route, conflict-detection testing, and troubleshooting guide.

### Endpoint summary

| Endpoint | Method | Auth |
|----------|--------|------|
| `/health` | GET | Public |
| `/auth/register` | POST | Public |
| `/auth/login` | POST | Public |
| `/events` | GET | User token |
| `/events/{id}` | GET | User token |
| `/bookings` | POST | User token |
| `/bookings/{id}` | DELETE | User token (owner only) |
| `/bookings/batch` | POST | User token |
| `/events` | POST | **Admin token** |
| `/events/{id}` | PUT | **Admin token** |
| `/events/{id}` | DELETE | **Admin token** |
| `/events/batch` | POST | **Admin token** |
| `/audit/{type}/{id}` | GET | **Admin token** |

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
| Soft-delete on events (`deleted_at`) | Historical bookings keep their FK reference intact |
| `version INT` on events + bookings | Optimistic locking — concurrent updates detected without table-level locks |
| Partial unique index on bookings | One active booking per user per event; re-booking allowed after cancellation |
| `JSONB` audit columns + GIN index | Fast containment queries: `diff @> '{"price_cents": {}}'` |

### Connecting directly

```bash
# From the docker/ directory:
docker compose exec postgres psql -U ticketflow -d ticketflow

# Useful queries
\dt
SELECT email, role FROM users;
SELECT name, remaining_tickets, version FROM events WHERE deleted_at IS NULL;
SELECT * FROM audit_logs ORDER BY created_at DESC LIMIT 10;
```

> **Optional — pgAdmin GUI:** If you prefer a browser-based database explorer, see [docker/PGADMIN_GUIDE.md](./docker/PGADMIN_GUIDE.md). pgAdmin is **not required** to run the API — it is purely a visual inspection tool. The `pgadmin` service in `docker-compose.yml` is commented out by default; follow the guide to enable it.

---

## Booking Conflict Detection — The 10-Step Flow

When multiple users race to book the last ticket simultaneously:

```
N concurrent POST /bookings requests
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

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `DATABASE_URL` | ✅ | — | Full PostgreSQL connection string |
| `JWT_SECRET` | ✅ | — | ≥ 32 chars — generate with `openssl rand -hex 32` |
| `REDIS_URL` | — | `redis://localhost:6379` | Use `redis://localhost:6380` for local dev |
| `PORT` | — | `8080` | HTTP server listen port |
| `JWT_EXPIRATION` | — | `24h` | Go duration string: `1h`, `7d`, etc. |
| `APP_ENV` | — | `development` | Set to `production` for compact JSON logs |
| `MIGRATIONS_PATH` | — | `migrations` | Override migrations directory path |

---

## Promoting a User to Admin

All newly registered users have `role=user`. There is no API endpoint for promotion — it requires direct database access (privilege escalation cannot happen through the API).

```bash
# 1. Promote in the database
docker compose exec postgres psql -U ticketflow -d ticketflow \
  -c "UPDATE users SET role='admin' WHERE email='your@email.com';"

# 2. Login again — the old token still carries role=user
# The new JWT issued by POST /auth/login will carry role=admin
```

---

*Module: `github.com/Allmight-456/ticketflow`*
*See [API_GUIDE.md](./API_GUIDE.md) for the full endpoint walkthrough and troubleshooting guide.*
