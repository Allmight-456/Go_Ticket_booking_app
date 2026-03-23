CREATE TYPE audit_action AS ENUM ('create', 'update', 'delete', 'login');

CREATE TABLE audit_logs (
    id            UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_id      UUID         REFERENCES users(id) ON DELETE SET NULL,
    resource_type VARCHAR(50)  NOT NULL,
    resource_id   UUID         NOT NULL,
    action        audit_action NOT NULL,
    old_state     JSONB,
    new_state     JSONB,
    diff          JSONB,
    ip_address    INET,
    user_agent    TEXT,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
    -- Intentionally immutable: no updated_at, no deleted_at.
    -- To "correct" an audit log, insert a new entry — never modify.
);

CREATE INDEX idx_audit_resource   ON audit_logs (resource_type, resource_id);
CREATE INDEX idx_audit_actor      ON audit_logs (actor_id);
CREATE INDEX idx_audit_created_at ON audit_logs (created_at DESC);
-- GIN index enables fast JSONB containment queries: diff @> '{"field": ...}'
CREATE INDEX idx_audit_diff_gin   ON audit_logs USING GIN (diff);
