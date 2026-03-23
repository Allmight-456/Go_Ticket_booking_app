CREATE TYPE event_status AS ENUM ('draft', 'published', 'cancelled', 'sold_out');

CREATE TABLE events (
    id                UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    name              VARCHAR(255) NOT NULL,
    description       TEXT,
    location          VARCHAR(255) NOT NULL,
    starts_at         TIMESTAMPTZ  NOT NULL,
    ends_at           TIMESTAMPTZ  NOT NULL,
    total_tickets     INT          NOT NULL,
    remaining_tickets INT          NOT NULL,
    price_cents       INT          NOT NULL DEFAULT 0,
    status            event_status NOT NULL DEFAULT 'draft',
    created_by        UUID         NOT NULL REFERENCES users(id),
    version           INT          NOT NULL DEFAULT 1,
    created_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    deleted_at        TIMESTAMPTZ,
    CONSTRAINT events_tickets_positive  CHECK (total_tickets > 0),
    CONSTRAINT events_remaining_valid   CHECK (remaining_tickets >= 0 AND remaining_tickets <= total_tickets),
    CONSTRAINT events_dates_valid       CHECK (ends_at > starts_at),
    CONSTRAINT events_price_nonnegative CHECK (price_cents >= 0)
);

-- Partial indexes exclude soft-deleted rows for free on every query.
CREATE INDEX idx_events_status    ON events (status)    WHERE deleted_at IS NULL;
CREATE INDEX idx_events_starts_at ON events (starts_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_events_created_by ON events (created_by) WHERE deleted_at IS NULL;
