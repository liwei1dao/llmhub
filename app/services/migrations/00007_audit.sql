-- +goose Up
CREATE TABLE audit.logs (
    id              BIGSERIAL PRIMARY KEY,
    actor_type      VARCHAR(16) NOT NULL,
    actor_id        BIGINT,
    action          VARCHAR(64) NOT NULL,
    target_type     VARCHAR(32),
    target_id       VARCHAR(64),
    ip              INET,
    user_agent      TEXT,
    result          VARCHAR(16),
    payload         JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_audit_actor  ON audit.logs(actor_type, actor_id, created_at DESC);
CREATE INDEX idx_audit_action ON audit.logs(action, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS audit.logs;
