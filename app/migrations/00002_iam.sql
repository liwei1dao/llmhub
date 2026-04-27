-- +goose Up
CREATE TABLE iam.users (
    id              BIGSERIAL PRIMARY KEY,
    email           VARCHAR(255) UNIQUE,
    phone           VARCHAR(32) UNIQUE,
    password_hash   VARCHAR(128) NOT NULL,
    display_name    VARCHAR(64),
    status          VARCHAR(16) NOT NULL DEFAULT 'active',
    realname_level  SMALLINT NOT NULL DEFAULT 0,
    risk_score      SMALLINT NOT NULL DEFAULT 0,
    qps_limit       INT NOT NULL DEFAULT 10,
    daily_spend_limit_cents BIGINT NOT NULL DEFAULT 100000,
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login_at   TIMESTAMPTZ
);

CREATE INDEX idx_users_status ON iam.users(status) WHERE status <> 'active';
CREATE INDEX idx_users_risk   ON iam.users(risk_score) WHERE risk_score > 60;

CREATE TABLE iam.api_keys (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES iam.users(id),
    prefix          VARCHAR(32) NOT NULL,
    key_hash        VARCHAR(128) NOT NULL,
    name            VARCHAR(64),
    scopes          TEXT[] NOT NULL DEFAULT '{}',
    ip_allowlist    INET[] NOT NULL DEFAULT '{}',
    status          VARCHAR(16) NOT NULL DEFAULT 'active',
    usage_cap_cents BIGINT,
    used_cents      BIGINT NOT NULL DEFAULT 0,
    last_used_at    TIMESTAMPTZ,
    expires_at      TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX uq_api_keys_hash ON iam.api_keys(key_hash);
CREATE INDEX idx_api_keys_user   ON iam.api_keys(user_id, status);
CREATE INDEX idx_api_keys_prefix ON iam.api_keys(prefix) WHERE status = 'active';

CREATE TABLE iam.sessions (
    id              UUID PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES iam.users(id),
    token_hash      VARCHAR(128) NOT NULL,
    user_agent      TEXT,
    ip              INET,
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at      TIMESTAMPTZ
);

-- +goose Down
DROP TABLE IF EXISTS iam.sessions;
DROP TABLE IF EXISTS iam.api_keys;
DROP TABLE IF EXISTS iam.users;
