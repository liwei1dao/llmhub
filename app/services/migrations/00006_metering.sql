-- +goose Up
-- +goose StatementBegin
CREATE TABLE metering.call_logs (
    id              UUID NOT NULL,
    ts              TIMESTAMPTZ NOT NULL,
    user_id         BIGINT NOT NULL,
    api_key_id      BIGINT NOT NULL,
    request_id      VARCHAR(64) NOT NULL,
    capability_id   VARCHAR(32) NOT NULL,
    model_id        VARCHAR(64) NOT NULL,
    provider_id     VARCHAR(32) NOT NULL,
    pool_account_id BIGINT NOT NULL,
    upstream_model  VARCHAR(128) NOT NULL,
    status          VARCHAR(16) NOT NULL,
    error_code      VARCHAR(64),
    duration_ms     INT NOT NULL,
    ttfb_ms         INT,
    tokens_in       INT NOT NULL DEFAULT 0,
    tokens_out      INT NOT NULL DEFAULT 0,
    audio_duration_seconds DECIMAL(10,3),
    characters      INT,
    images          SMALLINT,
    billing_unit    VARCHAR(16),
    billing_count   DECIMAL(16, 4),
    cost_wholesale_cents DECIMAL(16, 6) NOT NULL DEFAULT 0,
    cost_retail_cents    DECIMAL(16, 6) NOT NULL DEFAULT 0,
    retry_count     SMALLINT NOT NULL DEFAULT 0,
    fallback_chain  JSONB,
    source_lang     VARCHAR(16),
    target_lang     VARCHAR(16),
    voice_id        VARCHAR(64),
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    PRIMARY KEY (id, ts)
);
-- +goose StatementEnd

SELECT create_hypertable('metering.call_logs', 'ts', chunk_time_interval => INTERVAL '1 day');

CREATE INDEX idx_calls_user_ts     ON metering.call_logs(user_id, ts DESC);
CREATE INDEX idx_calls_provider_ts ON metering.call_logs(provider_id, ts DESC);
CREATE INDEX idx_calls_pool_ts     ON metering.call_logs(pool_account_id, ts DESC);
CREATE INDEX idx_calls_status      ON metering.call_logs(status, ts DESC) WHERE status <> 'success';
CREATE INDEX idx_calls_request     ON metering.call_logs(request_id);

CREATE TABLE metering.balance_holds (
    request_id      VARCHAR(64) PRIMARY KEY,
    user_id         BIGINT NOT NULL,
    account_id      BIGINT NOT NULL,
    amount_cents    BIGINT NOT NULL,
    status          VARCHAR(16) NOT NULL,
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    settled_at      TIMESTAMPTZ
);
CREATE INDEX idx_holds_expiry ON metering.balance_holds(status, expires_at) WHERE status = 'held';

CREATE TABLE metering.reconciliation (
    id              BIGSERIAL PRIMARY KEY,
    day             DATE NOT NULL,
    provider_id     VARCHAR(32) NOT NULL,
    pool_account_id BIGINT NOT NULL,
    platform_cost_cents  DECIMAL(16, 6) NOT NULL,
    upstream_bill_cents  DECIMAL(16, 6),
    diff_cents      DECIMAL(16, 6),
    diff_ratio      DECIMAL(8, 4),
    status          VARCHAR(16) NOT NULL,
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX uq_recon ON metering.reconciliation(day, pool_account_id);

-- +goose Down
DROP TABLE IF EXISTS metering.reconciliation;
DROP TABLE IF EXISTS metering.balance_holds;
DROP TABLE IF EXISTS metering.call_logs;
