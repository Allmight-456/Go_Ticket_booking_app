# TicketFlow — pgAdmin & PostgreSQL Visualisation Guide

Step-by-step instructions for spinning up pgAdmin, connecting it to the TicketFlow postgres container, and inspecting every table.

---

## 1. Enable the pgAdmin Container

The `docker-compose.yml` ships with pgAdmin commented out.
Open `docker/docker-compose.yml` and **uncomment** the `pgadmin` service block and the `pgadmin_data` volume line.

The relevant sections look like this after uncommenting:

```yaml
  pgadmin:
    image: dpage/pgadmin4:latest
    environment:
      PGADMIN_DEFAULT_EMAIL: admin@ticketflow.local
      PGADMIN_DEFAULT_PASSWORD: pgadminpass
    ports:
      - "5050:80"
    depends_on:
      postgres:
        condition: service_healthy
    volumes:
      - pgadmin_data:/var/lib/pgadmin
    restart: unless-stopped

volumes:
  postgres_data:
  pgadmin_data:
```

---

## 2. Start (or Restart) the Stack

```bash
cd docker

# If postgres is already running, just add pgadmin:
docker compose up pgadmin -d

# If starting from scratch:
docker compose up postgres redis pgadmin -d
```

Verify all three are healthy:
```bash
docker compose ps
```
```
NAME                  STATUS           PORTS
docker-postgres-1     Up (healthy)     0.0.0.0:5433->5432/tcp
docker-redis-1        Up (healthy)     0.0.0.0:6380->6379/tcp
docker-pgadmin-1      Up               0.0.0.0:5050->80/tcp
```

---

## 3. Open pgAdmin in Your Browser

```
http://localhost:5050
```

Login with:
| Field    | Value                      |
|----------|----------------------------|
| Email    | `admin@ticketflow.local`   |
| Password | `pgadminpass`              |

---

## 4. Register the TicketFlow PostgreSQL Server

Once logged in:

1. In the left panel, right-click **Servers → Register → Server…**
2. Fill in the **General** tab:
   - **Name**: `TicketFlow (local)`
3. Switch to the **Connection** tab and fill in:

| Field               | Value        | Note |
|---------------------|--------------|------|
| Host name / address | `postgres`   | Docker service name — not `localhost` |
| Port                | `5432`       | Internal container port, not the host-mapped 5433 |
| Maintenance database | `ticketflow` | |
| Username            | `ticketflow` | |
| Password            | `secret`     | |
| Save password?      | ✓ Yes        | |

4. Click **Save**.

> **Why `postgres` and not `localhost:5433`?**
> Both pgAdmin and postgres containers are on the same Docker bridge network.
> Within that network, containers reach each other by **service name** on the container's
> native port (`5432`). The `5433` mapping only applies to traffic coming from your host machine.

---

## 5. Explore the Database

Navigate the left panel:

```
Servers
  └─ TicketFlow (local)
       └─ Databases
            └─ ticketflow
                 └─ Schemas
                      └─ public
                           └─ Tables
                                ├─ users
                                ├─ events
                                ├─ bookings
                                ├─ audit_logs
                                └─ schema_migrations
```

Right-click any table → **View/Edit Data → All Rows** to inspect contents interactively.

---

## 6. Useful Queries (run in pgAdmin Query Tool)

Open the **Query Tool** via `Tools → Query Tool` or press `Alt+Shift+Q`.

### See all users and their roles
```sql
SELECT id, email, role, is_active, created_at
FROM users
ORDER BY created_at DESC;
```

### See published events with ticket availability
```sql
SELECT name, status, remaining_tickets, total_tickets, version, starts_at
FROM events
WHERE deleted_at IS NULL
ORDER BY starts_at;
```

### See all bookings with user emails
```sql
SELECT u.email, b.status, b.ticket_count, b.total_price_cents, b.booked_at
FROM bookings b
JOIN users u ON u.id = b.user_id
ORDER BY b.booked_at DESC;
```

### Full audit trail (most recent first)
```sql
SELECT actor.email, al.resource_type, al.resource_id,
       al.action, al.diff, al.ip_address, al.created_at
FROM audit_logs al
LEFT JOIN users actor ON actor.id = al.actor_id
ORDER BY al.created_at DESC
LIMIT 50;
```

### Check migration history
```sql
SELECT * FROM schema_migrations ORDER BY version;
```

### Manually promote a user to admin
```sql
UPDATE users
SET role = 'admin'
WHERE email = 'your@email.com';
```
> After running this SQL, the user must **login again** via `POST /auth/login` to get a new JWT with `role: "admin"` embedded.

### Reset remaining tickets on an event (useful after testing)
```sql
UPDATE events
SET remaining_tickets = total_tickets,
    version = version + 1
WHERE id = '<paste-event-uuid-here>';
```

### Wipe all test data (keeps schema intact)
```sql
TRUNCATE audit_logs, bookings, events, users CASCADE;
```

---

## 7. Stopping pgAdmin

```bash
cd docker
docker compose stop pgadmin
```

The `pgadmin_data` volume preserves your server connection so you don't have to re-enter credentials next time.

To remove it completely (including saved connections):
```bash
docker compose down
docker volume rm docker_pgadmin_data
```

---

## 8. Troubleshooting

### "could not connect to server"
- Make sure you used `postgres` (service name) as the host, **not** `localhost`.
- Confirm postgres is healthy: `docker compose ps`
- Try pinging from inside pgAdmin's container:
  ```bash
  docker compose exec pgadmin ping postgres
  ```

### pgAdmin shows a blank page or won't load
- Give it 10–15 seconds on first boot — the image initialises its internal database on startup.
- Check logs: `docker compose logs pgadmin`

### Forgot the pgAdmin login password
```bash
# Reset by recreating the container (volume keeps server connections)
docker compose stop pgadmin
docker compose rm -f pgadmin
docker compose up pgadmin -d
```
Login with the defaults again: `admin@ticketflow.local` / `pgadminpass`.
