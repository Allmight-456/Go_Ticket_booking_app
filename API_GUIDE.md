# TicketFlow — API Walkthrough & Troubleshooting Guide

Complete step-by-step guide to starting the stack, exercising every endpoint, and diagnosing failures.

---

## Quick Reference: Authentication Levels

| Endpoint | Method | Auth Required |
|----------|--------|---------------|
| `/health` | GET | None (public) |
| `/auth/register` | POST | None (public) |
| `/auth/login` | POST | None (public) |
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

> **There is no API route to promote a user to admin.** Promotion is done directly in the database. See [Admin Setup](#admin-setup-required-before-using-admin-routes) below.

---

## 1. Start Everything

### 1a. Start infrastructure (postgres + redis as containers)

```bash
cd docker
docker compose up postgres redis -d
```

Wait for both to be healthy (~5 seconds):
```bash
docker compose ps
```
```
NAME                STATUS           PORTS
docker-postgres-1   Up (healthy)     0.0.0.0:5433->5432/tcp
docker-redis-1      Up (healthy)     0.0.0.0:6380->6379/tcp
```

**If a port is already in use:**
```bash
ss -tlnp 'sport = :5433'   # check what's on 5433
ss -tlnp 'sport = :6380'   # check what's on 6380
```
The compose file maps postgres→5433 and redis→6380 specifically to avoid conflicts with a native postgres on 5432 or another Redis on 6379.

### 1b. Verify .env is correct

```bash
cat .env | grep -E 'DATABASE_URL|REDIS_URL|JWT_SECRET'
```

Should show:
```
DATABASE_URL=postgres://ticketflow:secret@localhost:5433/ticketflow?sslmode=disable
REDIS_URL=redis://localhost:6380
JWT_SECRET=<your 64-char hex string>
```

### 1c. Run the Go app

```bash
# From project root:
go run ./cmd/api/...
```

Expected startup logs (zerolog pretty format in dev):
```
INF postgres pool connected
INF database migrations applied
INF server starting addr=:8080
```

**Smoke test:**
```bash
curl -s localhost:8080/health
```
```json
{"status":"ok"}
```

If you don't see `{"status":"ok"}`, check [Troubleshooting](#8-troubleshooting) below.

---

## 2. Authentication Routes

### Register a new user

`POST /auth/register` is public — no token needed.

Required fields: `email`, `password`, `first_name`, `last_name`.
`first_name` and `last_name` are **only required at registration** — there is no profile-update endpoint, so these fields are collected once here.

```bash
curl -s -X POST localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "ishan@example.com",
    "password": "mypassword123",
    "first_name": "Ishan",
    "last_name": "Kumar"
  }'
```

**Success → 201 Created:**
```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "user": {
    "id": "523327355-9eb3-...",
    "email": "ishan@example.com",
    "first_name": "Ishan",
    "last_name": "Kumar",
    "role": "user",
    "is_active": true,
    "created_at": "2026-03-23T16:29:48Z",
    "updated_at": "2026-03-23T16:29:48Z"
  }
}
```

> **The token returned by `/auth/register` is a fully valid JWT — you can use it immediately for all user-level endpoints without calling `/auth/login` separately.**
> You only need to call `/auth/login` when: your token has expired (default 24h), or after being promoted to admin (to get a new token with `role: "admin"` embedded).

Save the token:
```bash
TOKEN="eyJhbGci..."
```

**Validation errors → 400:**
```json
{"error": "password must be at least 8 characters"}
{"error": "first and last name must be at least 2 characters"}
{"error": "invalid email address"}
```

**Duplicate email → 409:**
```json
{"error": "email already registered"}
```

---

### Login

`POST /auth/login` only needs `email` and `password`.

```bash
curl -s -X POST localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "ishan@example.com", "password": "mypassword123"}'
```

**Success → 200:**
```json
{"token": "eyJ...", "user": {...}}
```

**Wrong password → 401:**
```json
{"error": "invalid email or password"}
```

Save the token:
```bash
TOKEN="eyJhbGci..."
```

---

## 3. Admin Setup (Required Before Using Admin Routes)

> **There is no API endpoint to promote a user to admin.** This is intentional — privilege escalation cannot happen through the API. Promotion requires direct database access.

Follow these three steps in order:

### Step 1 — Register an account (if you haven't already)

```bash
curl -s -X POST localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "admin@example.com",
    "password": "adminpass123",
    "first_name": "Admin",
    "last_name": "User"
  }'
```

You will get a token back, but it has `role: "user"` — it cannot be used for admin routes yet.

### Step 2 — Promote to admin in the database

Connect to the running postgres container and run the SQL:

```bash
docker compose exec postgres psql -U ticketflow -d ticketflow \
  -c "UPDATE users SET role = 'admin' WHERE email = 'admin@example.com';"
```

Verify it worked:
```bash
docker compose exec postgres psql -U ticketflow -d ticketflow \
  -c "SELECT email, role FROM users WHERE email = 'admin@example.com';"
```

### Step 3 — Login again to get an admin JWT

The token from Step 1 still has `role: "user"` embedded in its claims — you **must** login again after promotion:

```bash
curl -s -X POST localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email": "admin@example.com", "password": "adminpass123"}'
```

Save this token separately from your regular user token:
```bash
ADMIN_TOKEN="eyJhbGci..."
```

You now have two tokens:
- `TOKEN` — for endpoints available to any logged-in user
- `ADMIN_TOKEN` — for admin-only endpoints (event write operations, audit trail)

---

## 4. Event Routes

**Read endpoints** require any valid user token (`TOKEN` or `ADMIN_TOKEN`).
**Write endpoints** (`POST`, `PUT`, `DELETE`) require an admin token (`ADMIN_TOKEN`).

### List events

```bash
curl -s localhost:8080/events \
  -H "Authorization: Bearer $TOKEN"
```

With filters:
```bash
curl -s "localhost:8080/events?status=published&limit=5&offset=0" \
  -H "Authorization: Bearer $TOKEN"
```

**Response → 200:**
```json
{
  "data": [
    {
      "id": "b6350aea-...",
      "name": "Go Conference 2026",
      "location": "Bangalore, India",
      "starts_at": "2026-12-01T09:00:00Z",
      "ends_at": "2026-12-01T18:00:00Z",
      "total_tickets": 3,
      "remaining_tickets": 1,
      "price_cents": 5000,
      "status": "published",
      "version": 3
    }
  ]
}
```

Valid `status` values: `draft`, `published`, `cancelled`, `sold_out`

---

### Get a single event

```bash
curl -s localhost:8080/events/<event-id> \
  -H "Authorization: Bearer $TOKEN"
```

**Not found → 404:**
```json
{"error": "event not found"}
```

---

### Create an event (admin only)

```bash
curl -s -X POST localhost:8080/events \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{
    "name": "Go Conference 2026",
    "description": "Annual Go developer summit",
    "location": "Bangalore, India",
    "starts_at": "2026-12-01T09:00:00Z",
    "ends_at":   "2026-12-01T18:00:00Z",
    "total_tickets": 100,
    "price_cents": 5000,
    "status": "published"
  }'
```

**Success → 201 Created:** returns the full event object.

**Non-admin user → 403:**
```json
{"error": "forbidden: admin access required"}
```

**Validation failures → 400:**
```json
{"error": "event name must be at least 3 characters"}
{"error": "ends_at must be after starts_at"}
{"error": "total_tickets must be positive"}
{"error": "price_cents cannot be negative"}
```

---

### Update an event (admin only)

You **must** send the current `version` field. This is the optimistic lock — if someone else edited the event between your GET and your PUT, the version will have changed and you'll get a 409 instead of silently overwriting.

The event ID can be provided **either** in the URL path (preferred) **or** as an `"id"` field in the JSON body. Both are equivalent — the URL param takes precedence if both are given.

**Option A — ID in the URL path (standard REST):**
```bash
curl -s -X PUT localhost:8080/events/<event-id> \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{
    "name": "Go Conference 2026 — Updated",
    "description": "Now with workshops!",
    "location": "Bangalore, India",
    "starts_at": "2026-12-01T09:00:00Z",
    "ends_at":   "2026-12-01T20:00:00Z",
    "total_tickets": 100,
    "price_cents": 6000,
    "status": "published",
    "version": 1
  }'
```

**Option B — ID in the request body (useful when you don't want to build the URL dynamically):**
```bash
curl -s -X PUT localhost:8080/events \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '{
    "id": "<event-id>",
    "name": "Go Conference 2026 — Updated",
    "description": "Now with workshops!",
    "location": "Bangalore, India",
    "starts_at": "2026-12-01T09:00:00Z",
    "ends_at":   "2026-12-01T20:00:00Z",
    "total_tickets": 100,
    "price_cents": 6000,
    "status": "published",
    "version": 1
  }'
```

> **Getting the event ID:** call `GET /events` first. The `"id"` field in each returned event object is the UUID to use here. The `"version"` field from that same GET response is what you send as `"version"`.

**Version mismatch → 409:**
```json
{"error": "version conflict — reload and retry"}
```

**Missing ID (neither URL nor body) → 400:**
```json
{"error": "event id is required: provide it in the URL path (/events/{id}) or as 'id' in the request body"}
```

---

### Delete an event (admin only, soft-delete)

```bash
curl -s -X DELETE localhost:8080/events/<event-id> \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

**Success → 204 No Content** (empty body).

The event row is not physically removed — `deleted_at` is set. Existing bookings still reference it. The event disappears from all `GET /events` responses automatically (partial index).

---

### Batch create events (admin only)

Create up to 50 events in a single round-trip using pgx's Batch API (all insert atomically):

```bash
curl -s -X POST localhost:8080/events/batch \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -d '[
    {
      "name": "Workshop A",
      "location": "Room 1",
      "starts_at": "2026-12-02T09:00:00Z",
      "ends_at":   "2026-12-02T12:00:00Z",
      "total_tickets": 30,
      "price_cents": 2000,
      "status": "draft"
    },
    {
      "name": "Workshop B",
      "location": "Room 2",
      "starts_at": "2026-12-02T13:00:00Z",
      "ends_at":   "2026-12-02T16:00:00Z",
      "total_tickets": 30,
      "price_cents": 2000,
      "status": "draft"
    }
  ]'
```

**Success → 201:**
```json
{"data": [...], "count": 2}
```

**Validation failure in one item stops the batch → 400:**
```json
{"error": "event[1]: ends_at must be after starts_at"}
```

---

## 5. Booking Routes

All booking routes require a valid user token. Cancellation additionally checks ownership — you can only cancel your own bookings.

### Book tickets

```bash
curl -s -X POST localhost:8080/bookings \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"event_id": "<event-id>", "ticket_count": 2}'
```

**Success → 201 Created:**
```json
{
  "id": "f7658234-...",
  "user_id": "523327...",
  "event_id": "b6350a...",
  "ticket_count": 2,
  "total_price_cents": 10000,
  "status": "confirmed",
  "version": 1,
  "booked_at": "2026-03-23T16:32:00Z",
  "updated_at": "2026-03-23T16:32:00Z"
}
```

**Error responses and what they mean:**

| HTTP | Error message | Cause |
|------|--------------|-------|
| 404 | `event not found` | Wrong event ID |
| 422 | `event is not available for booking` | Status is `draft`/`cancelled`/`sold_out` |
| 409 | `not enough tickets remaining` | Race lost — someone got the last ticket first |
| 409 | `active booking already exists for this event` | You already have a `pending`/`confirmed` booking for this event |
| 400 | `event is currently being processed, try again shortly` | Redis lock held — retry in ~1 second |

---

### Cancel a booking

```bash
curl -s -X DELETE localhost:8080/bookings/<booking-id> \
  -H "Authorization: Bearer $TOKEN"
```

**Success → 200:** returns the updated booking with `"status":"cancelled"`.

Cancelling automatically restores the ticket count on the event — no admin action needed.

**Error responses:**

| HTTP | Error | Cause |
|------|-------|-------|
| 404 | `booking not found` | Wrong booking ID |
| 403 | `only the booking owner can perform this action` | Trying to cancel someone else's booking |
| 422 | `booking cannot be cancelled in current status` | Already cancelled/refunded |

---

### Batch book (up to 10 bookings)

Each entry goes through the full conflict-safe flow independently:

```bash
curl -s -X POST localhost:8080/bookings/batch \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '[
    {"event_id": "<event-id-1>", "ticket_count": 1},
    {"event_id": "<event-id-2>", "ticket_count": 2}
  ]'
```

**Success → 201:**
```json
{"data": [...], "count": 2}
```

**Partial failure** — if booking[1] fails, booking[0] is already committed (not rolled back). The response contains completed bookings + the error for the failed one:
```json
{"error": "booking[1] (event abc...): not enough tickets remaining"}
```

---

## 6. Audit Trail Route (admin only)

```bash
curl -s "localhost:8080/audit/<resource_type>/<resource_id>" \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

`resource_type` is one of: `event`, `booking`, `user`

**Example — full history of an event:**
```bash
curl -s "localhost:8080/audit/event/<event-id>" \
  -H "Authorization: Bearer $ADMIN_TOKEN"
```

**Response → 200:**
```json
{
  "count": 2,
  "data": [
    {
      "id": "a1061f...",
      "actor_id": "2d3ea6...",
      "resource_type": "event",
      "resource_id": "b6350a...",
      "action": "update",
      "old_state": {"name": "Go Conference 2026", "price_cents": 5000},
      "new_state": {"name": "Go Conference 2026 — Updated", "price_cents": 6000},
      "diff": {
        "name":        {"old": "Go Conference 2026", "new": "Go Conference 2026 — Updated"},
        "price_cents": {"old": 5000,                 "new": 6000}
      },
      "ip_address": "127.0.0.1/32",
      "created_at": "2026-03-23T17:00:00Z"
    },
    {
      "action": "create",
      "diff": {},
      "created_at": "2026-03-23T16:24:40Z"
    }
  ]
}
```

The `diff` field contains only changed keys — makes it easy to answer "who changed the price and when?"

---

## 7. Testing Conflict Detection

To see the concurrency protection work, fire 4 simultaneous bookings for an event that only has 3 tickets:

```bash
EVENT_ID="<your-event-id>"

# Register 4 users first, save their tokens
for name in Alice Bob Carol Dave; do
  eval "T_$name=$(curl -s -X POST localhost:8080/auth/register \
    -H 'Content-Type: application/json' \
    -d "{\"email\":\"${name}@race.dev\",\"password\":\"pass1234\",\"first_name\":\"$name\",\"last_name\":\"Test\"}" \
    | python3 -c "import sys,json; print(json.load(sys.stdin)['token'])")"
done

# Fire all 4 simultaneously
for name in Alice Bob Carol Dave; do
  eval "TOK=\$T_$name"
  (curl -s -X POST localhost:8080/bookings \
    -H 'Content-Type: application/json' \
    -H "Authorization: Bearer $TOK" \
    -d "{\"event_id\":\"$EVENT_ID\",\"ticket_count\":1}" \
    | python3 -c "import sys,json; d=json.load(sys.stdin); print('$name:', d.get('status', d.get('error')))") &
done
wait

# Verify remaining tickets
docker compose exec postgres psql -U ticketflow -d ticketflow \
  -c "SELECT remaining_tickets, version FROM events WHERE id='$EVENT_ID';"
```

**Expected output:**
```
Alice: confirmed
Bob:   event is currently being processed, try again shortly
Carol: confirmed
Dave:  event is currently being processed, try again shortly

 remaining_tickets | version
-------------------+---------
                 1 |       3
```

> Only 1 booking can hold the Redis lock at a time. Others get an immediate rejection and should retry — there's no automatic retry in the current implementation.

---

## 8. Troubleshooting

### Server won't start

**`missing required environment variables: DATABASE_URL, JWT_SECRET`**
```bash
# .env file missing or env vars not set
cp .env.example .env
# Edit .env and set DATABASE_URL, REDIS_URL, JWT_SECRET
```

**`database connection failed: ping db: ...`**
```bash
# Postgres container not running or wrong port
docker compose ps          # check health status
docker compose logs postgres  # see postgres startup errors
# Check .env: DATABASE_URL should use port 5433, not 5432
```

**`redis connection failed: redis ping: ...`**
```bash
docker compose ps          # check redis health
# Check .env: REDIS_URL should use port 6380, not 6379
```

**`listen tcp :8080: bind: address already in use`**
```bash
fuser -k 8080/tcp          # kill whatever is on 8080
# or change PORT= in .env
```

---

### Auth errors

**`{"error":"missing or invalid authorization header"}`**
```bash
# Header must be exactly: Authorization: Bearer <token>
# Not: Bearer<token>, not: Token <token>, not just the token alone
curl -H "Authorization: Bearer $TOKEN" ...
```

**`{"error":"invalid or expired token"}`**
- Token has expired (default 24h) → login again
- JWT_SECRET changed since the token was issued → login again
- Token was copy-pasted with a newline or extra space → trim it

**Admin route returns `{"error":"forbidden: admin access required"}`**
- Your token was issued before you were promoted → login again after the `UPDATE users SET role='admin'` SQL
- You are using `$TOKEN` instead of `$ADMIN_TOKEN`

---

### Booking errors

**`{"error":"event is currently being processed, try again shortly"}`**
- The Redis lock for this event is held by another request
- This is expected under high concurrency — add retry logic in your client or just re-run the curl

**`{"error":"not enough tickets remaining"}`**
- Remaining tickets < requested count
- Check: `SELECT remaining_tickets FROM events WHERE id='<id>';`

**`{"error":"active booking already exists for this event"}`**
- The same user already has a `pending` or `confirmed` booking for this event
- Cancel the existing booking first, or use a different user

**`{"error":"event is not available for booking"}`**
- Event status is not `published`
- Admin must set `"status":"published"` first via `PUT /events/:id`

---

### Database inspection

```bash
# Connect
docker compose exec postgres psql -U ticketflow -d ticketflow

# Useful queries
SELECT email, role FROM users;
SELECT name, status, remaining_tickets, version FROM events WHERE deleted_at IS NULL;
SELECT u.email, b.status, b.ticket_count FROM bookings b JOIN users u ON u.id=b.user_id;
SELECT resource_type, action, ip_address, created_at FROM audit_logs ORDER BY created_at DESC LIMIT 20;

# Check migration history
SELECT * FROM schema_migrations;

# Manually fix an event's ticket count if something went wrong in testing
UPDATE events SET remaining_tickets = total_tickets, version = version + 1 WHERE id = '<id>';
```

---

### Resetting test data

```bash
# Wipe everything and start fresh (containers stay up, data is cleared)
docker compose exec postgres psql -U ticketflow -d ticketflow \
  -c "TRUNCATE audit_logs, bookings, events, users CASCADE;"

# Or nuke the postgres volume entirely and recreate
docker compose down -v
docker compose up postgres redis -d
# Migrations re-apply automatically on next `go run ./cmd/api/...`
```

---

### Checking Redis

```bash
# Connect to the redis container
docker compose exec redis redis-cli

# See all active keys
KEYS *

# Check a specific rate-limit window
ZRANGE "rl:127.0.0.1:bookings" 0 -1 WITHSCORES

# Check if an event lock is held
GET "lock:event:<event-id>"

# Flush all Redis data (clears all locks and caches)
FLUSHALL
```

---

## 9. Quick Reference Card

Set your shell variables first:
```bash
BASE_URL="localhost:8080"
TOKEN=""        # set after register or login
ADMIN_TOKEN=""  # set after admin login (see Section 3)
EVENT_ID=""     # set after creating an event
BOOKING_ID=""   # set after creating a booking
```

> **Postman users:** create an Environment with variables `BASE_URL`, `TOKEN`, `ADMIN_TOKEN`, `EVENT_ID`, `BOOKING_ID` and replace the shell `$VAR` syntax with `{{VAR}}` in each request.

```bash
# ─── Start ───────────────────────────────────────────────────────────
cd docker && docker compose up postgres redis -d   # infra containers
go run ./cmd/api/...                               # app (from project root)
curl $BASE_URL/health                              # smoke test

# ─── Auth (public — no token needed) ─────────────────────────────────
curl -X POST $BASE_URL/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"email":"x@x.com","password":"pass1234","first_name":"Xx","last_name":"Xx"}'

curl -X POST $BASE_URL/auth/login \
  -H 'Content-Type: application/json' \
  -d '{"email":"x@x.com","password":"pass1234"}'

# ─── Admin setup (no API route — DB only) ────────────────────────────
docker compose exec postgres psql -U ticketflow -d ticketflow \
  -c "UPDATE users SET role='admin' WHERE email='x@x.com';"
# Then login again to get ADMIN_TOKEN (same login curl as above)

# ─── Events — read (user token) ──────────────────────────────────────
curl $BASE_URL/events -H "Authorization: Bearer $TOKEN"
curl $BASE_URL/events/$EVENT_ID -H "Authorization: Bearer $TOKEN"

# ─── Events — write (admin token required) ───────────────────────────
curl -X POST $BASE_URL/events \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"name":"My Event","location":"City","starts_at":"2026-12-01T09:00:00Z","ends_at":"2026-12-01T18:00:00Z","total_tickets":100,"price_cents":5000,"status":"published"}'

curl -X PUT $BASE_URL/events/$EVENT_ID \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{...,"version":1}'
# OR — embed the id in the body (no EVENT_ID needed in the URL):
curl -X PUT $BASE_URL/events \
  -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"id":"'$EVENT_ID'",...,"version":1}'

curl -X DELETE $BASE_URL/events/$EVENT_ID \
  -H "Authorization: Bearer $ADMIN_TOKEN"

# ─── Bookings (user token) ────────────────────────────────────────────
curl -X POST $BASE_URL/bookings \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"event_id":"'$EVENT_ID'","ticket_count":1}'

curl -X DELETE $BASE_URL/bookings/$BOOKING_ID \
  -H "Authorization: Bearer $TOKEN"

# ─── Audit (admin token required) ────────────────────────────────────
curl "$BASE_URL/audit/event/$EVENT_ID" -H "Authorization: Bearer $ADMIN_TOKEN"
curl "$BASE_URL/audit/booking/$BOOKING_ID" -H "Authorization: Bearer $ADMIN_TOKEN"
curl "$BASE_URL/audit/user/<user-id>" -H "Authorization: Bearer $ADMIN_TOKEN"
```
