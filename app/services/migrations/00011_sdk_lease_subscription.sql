-- +goose Up
-- v0.2 SDK 凭据下发 + 用户订阅 / 配额扣减
-- 这是 LLMHub「聚合 SDK 平台」模式的两张核心新表：
--   pool.leases             — SDK 来 issue 时签发一次性 lease，记录哪个 binding 的真凭据被分发给了哪个用户的哪次会话
--   iam.subscriptions       — 用户订阅了哪些 SKU、剩余配额、套餐周期
-- 这两张表是中间站 → 聚合 SDK 平台转型的数据基础。

-- +goose StatementBegin
-- ─────────────────────────────────────────────────────────────
-- pool.leases — SDK 凭据下发记录
-- ─────────────────────────────────────────────────────────────
-- 每次 SDK 调 POST /sdk/credentials/issue 都新增一行。
-- lease 是一次性的、有 TTL 的「凭据租约」，平台后端通过 lease_id 关联：
--   ① 哪个 user 在用
--   ② 走的哪个 SKU（vendor_product + capability + upstream_model）
--   ③ pick 到的哪个 binding（→ credential → vault auth_payload_ref）
-- SDK 异步上报 usage 时带 lease_id，后端就能扣对应用户的配额、调对应 binding 的健康分。
CREATE TABLE pool.leases (
    id                BIGSERIAL    PRIMARY KEY,
    -- 公开 lease 标识符（SDK 持有），不暴露内部 id
    lease_id          UUID         NOT NULL DEFAULT gen_random_uuid() UNIQUE,
    user_id           BIGINT       NOT NULL REFERENCES iam.users(id),
    api_key_id        BIGINT       NOT NULL REFERENCES iam.api_keys(id), -- SDK 用的哪对 (id,key) 来 issue 的
    sku_id            VARCHAR(64)  NOT NULL REFERENCES catalog.platform_services(id),
    binding_id        BIGINT       NOT NULL REFERENCES pool.credential_services(id),
    credential_id     BIGINT       NOT NULL REFERENCES pool.credentials(id),
    -- 客户端环境 fingerprint（用于风控；SDK 可上报机器 / 进程指纹）
    client_fingerprint VARCHAR(128),
    client_ip         INET,
    -- 状态
    status            VARCHAR(16)  NOT NULL DEFAULT 'active', -- active / expired / revoked
    issued_at         TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    expires_at        TIMESTAMPTZ  NOT NULL, -- 默认 issued_at + 15min
    revoked_at        TIMESTAMPTZ,
    revoke_reason     VARCHAR(64),
    -- 用量汇总（方便 lease 维度统计）
    last_used_at      TIMESTAMPTZ,
    use_count         INT          NOT NULL DEFAULT 0,
    total_input_units BIGINT       NOT NULL DEFAULT 0,
    total_output_units BIGINT      NOT NULL DEFAULT 0
);

CREATE INDEX idx_leases_user_active
    ON pool.leases(user_id, expires_at DESC)
    WHERE status = 'active';
CREATE INDEX idx_leases_binding
    ON pool.leases(binding_id, issued_at DESC);
CREATE INDEX idx_leases_expiring
    ON pool.leases(expires_at)
    WHERE status = 'active';
-- +goose StatementEnd

-- +goose StatementBegin
-- ─────────────────────────────────────────────────────────────
-- iam.subscriptions — 用户订阅 SKU + 配额追踪
-- ─────────────────────────────────────────────────────────────
-- 不再走 wallet freeze/settle 那套；改成「订阅 + 配额」：
--   一个用户对一个 SKU 一行，记录套餐周期、剩余配额、单价（套餐内 / 套餐外）。
-- /sdk/credentials/issue 入口校验 quota_remaining > 0（粗粒度），
-- /sdk/usage/report 入口扣减 quota_remaining。
CREATE TABLE iam.subscriptions (
    id                  BIGSERIAL    PRIMARY KEY,
    user_id             BIGINT       NOT NULL REFERENCES iam.users(id),
    sku_id              VARCHAR(64)  NOT NULL REFERENCES catalog.platform_services(id),
    -- 套餐：包月 / 按量预付费 / 试用
    plan_kind           VARCHAR(16)  NOT NULL,                 -- monthly / prepaid / trial
    plan_name           VARCHAR(64),                           -- "doubao-pro 入门版" 之类
    -- 配额：以 SKU.billing_unit 为单位
    --   1k_tokens → quota = 千 tokens 数；image → 张数；minute → 分钟数
    quota_total         BIGINT       NOT NULL,                 -- 周期内总配额
    quota_used          BIGINT       NOT NULL DEFAULT 0,
    -- 周期边界
    period_start        TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    period_end          TIMESTAMPTZ  NOT NULL,
    auto_renew          BOOLEAN      NOT NULL DEFAULT FALSE,
    -- 状态
    status              VARCHAR(16)  NOT NULL DEFAULT 'active', -- active / expired / suspended / cancelled
    -- 风控限速（避免某个 user 一秒打爆 binding）
    qps_limit           INT          NOT NULL DEFAULT 10,
    daily_quota_limit   BIGINT,                                -- NULL = 不限日配额
    daily_used          BIGINT       NOT NULL DEFAULT 0,
    daily_used_date     DATE         NOT NULL DEFAULT CURRENT_DATE,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uq_subscriptions_user_sku_active
    ON iam.subscriptions(user_id, sku_id)
    WHERE status = 'active';
CREATE INDEX idx_subscriptions_period
    ON iam.subscriptions(period_end)
    WHERE status = 'active';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS iam.subscriptions CASCADE;
DROP TABLE IF EXISTS pool.leases       CASCADE;
-- +goose StatementEnd
