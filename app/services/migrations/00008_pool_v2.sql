-- +goose Up
-- v0.2 五层资源模型重构（pool 域）
-- 旧表 pool.{accounts, api_keys, account_capability_quotas, account_events,
-- risk_isolation_groups} 全部 drop，按 vendor_account → credential →
-- credential_service 三层重建。pool.risk_isolation_groups 同步重建以
-- 切断对 catalog.providers 的外键（v0.2 中 vendor 是代码常量，DB 无表）。
--
-- 见 docs/03-核心数据结构设计-v2.md §pool。

-- +goose StatementBegin
DROP TABLE IF EXISTS pool.account_events           CASCADE;
DROP TABLE IF EXISTS pool.account_capability_quotas CASCADE;
DROP TABLE IF EXISTS pool.api_keys                 CASCADE;
DROP TABLE IF EXISTS pool.accounts                 CASCADE;
DROP TABLE IF EXISTS pool.risk_isolation_groups    CASCADE;
-- +goose StatementEnd

-- 风控隔离组：把可能被上游识别为同源的凭据归一组，封号时整组冷却。
CREATE TABLE pool.risk_isolation_groups (
    id              BIGSERIAL    PRIMARY KEY,
    code            VARCHAR(32)  UNIQUE NOT NULL,           -- 'volc-shared-001' / 'aliyun-team-A'
    name            VARCHAR(64),
    vendor_id       VARCHAR(32)  NOT NULL,                   -- 引用代码常量 catalog.Vendors
    tier_default    VARCHAR(8),                              -- 默认 tier
    origin          VARCHAR(32),                             -- enterprise / reseller / partner / trial
    notes           TEXT,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

-- 主账号：厂商的"行政/结算单元"。承担余额查询、账单对账，不调业务 API。
CREATE TABLE pool.vendor_accounts (
    id                    BIGSERIAL    PRIMARY KEY,
    vendor_id             VARCHAR(32)  NOT NULL,             -- 引用代码常量 catalog.Vendors
    name                  VARCHAR(64)  NOT NULL,             -- 运营备注："公司A 火山主账号"
    entity                VARCHAR(128),                       -- 主体/公司
    console_url           TEXT,                               -- 厂商控制台 URL（对账跳转）
    master_auth_ref       TEXT         NOT NULL,             -- vault://pool/vendor_accounts/{id}/master
    last_balance_cents    BIGINT,                             -- 上次查询余额（折算成分；外币按汇率换算）
    last_balance_currency VARCHAR(8),                         -- 'CNY' / 'USD' / ...
    last_balance_at       TIMESTAMPTZ,
    last_balance_error    TEXT,                               -- 上次查询失败的错误（NULL=成功）
    status                VARCHAR(16)  NOT NULL DEFAULT 'active', -- active / frozen / archived
    metadata              JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_vendor_account_name UNIQUE (vendor_id, name)
);
CREATE INDEX idx_vendor_accounts_vendor ON pool.vendor_accounts(vendor_id, status);

-- 凭据：某主账号下、某业务板块的"应用"。
CREATE TABLE pool.credentials (
    id                    BIGSERIAL    PRIMARY KEY,
    vendor_id             VARCHAR(32)  NOT NULL,             -- 反规范化字段（应用层校验 = account.vendor_id = product.vendor_id）
    account_id            BIGINT       NOT NULL REFERENCES pool.vendor_accounts(id) ON DELETE RESTRICT,
    product_id            VARCHAR(64)  NOT NULL,             -- 引用代码常量 catalog.Products
    name                  VARCHAR(64)  NOT NULL,             -- 运营备注："方舟应用-生产"
    env                   VARCHAR(16)  NOT NULL DEFAULT 'production',  -- production / staging / trial
    auth_payload_ref      TEXT         NOT NULL,             -- vault://pool/credentials/{id}
    auth_payload_digest   VARCHAR(64),                        -- 凭据规范化哈希，去重 + 漂移检测
    status                VARCHAR(16)  NOT NULL DEFAULT 'active', -- active / cooldown / banned / archived
    health_score          SMALLINT     NOT NULL DEFAULT 100,  -- 0-100，凭据级聚合健康度
    cooldown_until        TIMESTAMPTZ,                        -- cooldown 状态下的恢复时间
    isolation_group_id    BIGINT       REFERENCES pool.risk_isolation_groups(id),
    last_used_at          TIMESTAMPTZ,
    last_error_at         TIMESTAMPTZ,
    consecutive_failures  INT          NOT NULL DEFAULT 0,
    metadata              JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_credential_name UNIQUE (account_id, product_id, name)
);
CREATE INDEX idx_credentials_routing
    ON pool.credentials(product_id, status, health_score DESC)
    WHERE status = 'active';
CREATE INDEX idx_credentials_vendor    ON pool.credentials(vendor_id, status);
CREATE INDEX idx_credentials_account   ON pool.credentials(account_id);
CREATE INDEX idx_credentials_isolation ON pool.credentials(isolation_group_id);

-- 服务绑定：凭据 × 上游能力。调度的最小单元。
CREATE TABLE pool.credential_services (
    id                    BIGSERIAL      PRIMARY KEY,
    credential_id         BIGINT         NOT NULL REFERENCES pool.credentials(id) ON DELETE CASCADE,
    capability            VARCHAR(32)    NOT NULL,           -- 引用代码常量 catalog.Capabilities
    tier                  VARCHAR(8)     NOT NULL DEFAULT 'pro',  -- free / pro / enterprise
    qps_limit             INT,                                -- 单 binding QPS 上限（NULL=无限）
    daily_limit_cents     BIGINT,                             -- 日消费上限（分）
    quota_total_cents     BIGINT,                             -- 总额度（分）
    daily_used_cents      BIGINT         NOT NULL DEFAULT 0,  -- 当日已用，按 UTC 跨天重置
    quota_used_cents      BIGINT         NOT NULL DEFAULT 0,
    quota_reset_at        TIMESTAMPTZ,
    cost_basis_cents      DECIMAL(12,4)  NOT NULL DEFAULT 0,  -- 上游对此凭据的真实结算单价
    health_score          SMALLINT       NOT NULL DEFAULT 100,
    consecutive_failures  INT            NOT NULL DEFAULT 0,
    last_success_at       TIMESTAMPTZ,
    last_error_at         TIMESTAMPTZ,
    last_error_code       VARCHAR(64),
    status                VARCHAR(16)    NOT NULL DEFAULT 'active', -- active / cooldown / rate_limited / banned / archived
    cooldown_until        TIMESTAMPTZ,
    metadata              JSONB          NOT NULL DEFAULT '{}'::jsonb,
    created_at            TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    CONSTRAINT uq_credential_capability UNIQUE (credential_id, capability)
);
CREATE INDEX idx_bindings_routing
    ON pool.credential_services(capability, status, health_score DESC, cost_basis_cents ASC)
    WHERE status = 'active';
CREATE INDEX idx_bindings_credential ON pool.credential_services(credential_id);
CREATE INDEX idx_bindings_cooldown
    ON pool.credential_services(cooldown_until) WHERE status = 'cooldown';

-- 凭据/绑定状态事件（追加型）
CREATE TABLE pool.credential_events (
    id              BIGSERIAL    PRIMARY KEY,
    credential_id   BIGINT       REFERENCES pool.credentials(id),
    binding_id      BIGINT       REFERENCES pool.credential_services(id),
    event_type      VARCHAR(32)  NOT NULL,
                                 -- state_change / health_update / cooldown_start / cooldown_end
                                 -- quota_sync / probe_failed / probe_succeeded
                                 -- upstream_5xx / upstream_429 / upstream_auth_error
    from_state      VARCHAR(16),
    to_state        VARCHAR(16),
    capability      VARCHAR(32),                  -- binding 级事件填，凭据级 NULL
    reason          TEXT,
    metadata        JSONB        NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    CHECK (credential_id IS NOT NULL OR binding_id IS NOT NULL)
);
CREATE INDEX idx_credential_events_cred    ON pool.credential_events(credential_id, created_at DESC);
CREATE INDEX idx_credential_events_binding ON pool.credential_events(binding_id, created_at DESC);

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS pool.credential_events     CASCADE;
DROP TABLE IF EXISTS pool.credential_services   CASCADE;
DROP TABLE IF EXISTS pool.credentials           CASCADE;
DROP TABLE IF EXISTS pool.vendor_accounts       CASCADE;
DROP TABLE IF EXISTS pool.risk_isolation_groups CASCADE;
-- +goose StatementEnd

-- 回滚到 v0.1 schema（与 00005_pool.sql 一致）
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
CREATE INDEX idx_pool_selector  ON pool.accounts(provider_id, status, tier, health_score DESC) WHERE status = 'active';
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
