CREATE TYPE booking_status AS ENUM ('pending', 'confirmed', 'cancelled', 'refunded');

CREATE TABLE bookings (
    id                UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID           NOT NULL REFERENCES users(id),
    event_id          UUID           NOT NULL REFERENCES events(id),
    ticket_count      INT            NOT NULL,
    total_price_cents BIGINT         NOT NULL DEFAULT 0,
    status            booking_status NOT NULL DEFAULT 'pending',
    version           INT            NOT NULL DEFAULT 1,
    booked_at         TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    cancelled_at      TIMESTAMPTZ,
    CONSTRAINT bookings_ticket_count_positive CHECK (ticket_count > 0),
    CONSTRAINT bookings_price_nonnegative     CHECK (total_price_cents >= 0)
);

-- One active booking per user per event (Postgres partial unique index).
-- Using a partial index instead of a CHECK constraint lets us store historical
-- cancelled/refunded bookings without blocking future rebookings.
CREATE UNIQUE INDEX idx_bookings_one_active_per_user
    ON bookings (user_id, event_id)
    WHERE status IN ('pending', 'confirmed');

CREATE INDEX idx_bookings_user_id  ON bookings (user_id);
CREATE INDEX idx_bookings_event_id ON bookings (event_id);
