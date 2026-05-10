-- +goose Up
CREATE TABLE pool.risk_isolation_groups (
    id              BIGSERIAL PRIMARY KEY,
    code            VARCHAR(32) UNIQUE NOT NULL,
    name            VARCHAR(64),
    provider_id     VARCHAR(32) NOT NULL REFERENCES catalog.providers(id),
    tier            VARCHAR(8) NOT NULL,
    origin          VARCHAR(32),
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE pool.accounts (
    id              BIGSERIAL PRIMARY KEY,
    provider_id     VARCHAR(32) NOT NULL REFERENCES catalog.providers(id),
    account_ref     VARCHAR(128),
    tier            VARCHAR(8) NOT NULL,
    origin          VARCHAR(32) NOT NULL,
    isolation_group_id BIGINT REFERENCES pool.risk_isolation_groups(id),
    status          VARCHAR(16) NOT NULL DEFAULT 'warmup',
    health_score    SMALLINT NOT NULL DEFAULT 60,
    supported_capabilities TEXT[] NOT NULL DEFAULT '{}',
    quota_total_cents   BIGINT,
    quota_used_cents    BIGINT NOT NULL DEFAULT 0,
    quota_reset_at      TIMESTAMPTZ,
    qps_limit       INT NOT NULL DEFAULT 20,
    daily_limit_cents   BIGINT,
    cost_basis_cents    BIGINT NOT NULL DEFAULT 0,
    tags            TEXT[] NOT NULL DEFAULT '{}',
    registered_at   TIMESTAMPTZ,
    warmup_ends_at  TIMESTAMPTZ,
    last_used_at    TIMESTAMPTZ,
    last_error_at   TIMESTAMPTZ,
    consecutive_failures INT NOT NULL DEFAULT 0,
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_pool_selector ON pool.accounts(provider_id, status, tier, health_score DESC)
  WHERE status = 'active';
CREATE INDEX idx_pool_isolation ON pool.accounts(isolation_group_id);

CREATE TABLE pool.api_keys (
    id              BIGSERIAL PRIMARY KEY,
    account_id      BIGINT NOT NULL REFERENCES pool.accounts(id),
    vault_ref       TEXT NOT NULL,
    scope           VARCHAR(32) NOT NULL,
    upstream_model_filter TEXT[] NOT NULL DEFAULT '{}',
    status          VARCHAR(16) NOT NULL DEFAULT 'active',
    rotated_at      TIMESTAMPTZ,
    last_used_at    TIMESTAMPTZ,
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_pool_keys_account ON pool.api_keys(account_id, status);

CREATE TABLE pool.account_capability_quotas (
    id              BIGSERIAL PRIMARY KEY,
    account_id      BIGINT NOT NULL REFERENCES pool.accounts(id),
    capability_id   VARCHAR(32) NOT NULL,
    quota_total_cents   BIGINT,
    quota_used_cents    BIGINT NOT NULL DEFAULT 0,
    qps_limit       INT,
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX uq_acc_cap ON pool.account_capability_quotas(account_id, capability_id);

CREATE TABLE pool.account_events (
    id              BIGSERIAL PRIMARY KEY,
    account_id      BIGINT NOT NULL REFERENCES pool.accounts(id),
    event_type      VARCHAR(32) NOT NULL,
    from_state      VARCHAR(16),
    to_state        VARCHAR(16),
    reason          TEXT,
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_events_acc_time ON pool.account_events(account_id, created_at DESC);

-- +goose Down
DROP TABLE IF EXISTS pool.account_events;
DROP TABLE IF EXISTS pool.account_capability_quotas;
DROP TABLE IF EXISTS pool.api_keys;
DROP TABLE IF EXISTS pool.accounts;
DROP TABLE IF EXISTS pool.risk_isolation_groups;
