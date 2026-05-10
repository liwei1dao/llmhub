-- +goose Up
-- v0.2 五层资源模型重构（catalog 域）
-- 旧的 catalog.{providers, models, model_mappings, pricing,
-- capabilities, voice_mappings} 全部 drop。
--   - providers / capabilities：v0.2 改为代码常量（catalog.Vendors、
--     catalog.Capabilities）
--   - models / model_mappings：合并为单表 catalog.platform_services
--   - pricing：拆为 catalog.platform_pricing（零售）+
--     pool.credential_services.cost_basis_cents（成本）
--   - voice_mappings：依赖 catalog.providers 外键，留作 TTS 路由重写
--     时再恢复
-- catalog.voices / glossaries 保持不变。
--
-- 见 docs/03-核心数据结构设计-v2.md §catalog。

-- +goose StatementBegin
DROP TABLE IF EXISTS catalog.voice_mappings CASCADE;
DROP TABLE IF EXISTS catalog.pricing        CASCADE;
DROP TABLE IF EXISTS catalog.model_mappings CASCADE;
DROP TABLE IF EXISTS catalog.models         CASCADE;
DROP TABLE IF EXISTS catalog.providers      CASCADE;
DROP TABLE IF EXISTS catalog.capabilities   CASCADE;
-- +goose StatementEnd

-- 平台对外 SKU：用户在"模型市场"看到的服务条目。
-- 每条 SKU 绑定唯一一组 (vendor_product, capability, [model])。
CREATE TABLE catalog.platform_services (
    id                  VARCHAR(64)  PRIMARY KEY,            -- "doubao-pro" / "doubao-asr-realtime" / "claude-haiku-4"
    category_id         VARCHAR(16)  NOT NULL,               -- 引用代码常量 catalog.Categories
    display_name        VARCHAR(128) NOT NULL,               -- 用户看到的显示名："豆包 Pro"
    description         TEXT,
    -- ── 上游绑定 ──
    vendor_product_id   VARCHAR(64)  NOT NULL,               -- 引用代码常量 catalog.Products（"volc.ark"）
    capability          VARCHAR(32)  NOT NULL,               -- 引用代码常量 catalog.Capabilities
                                                              -- 必须 ∈ Products[vendor_product_id].AllowedCapabilities（应用层校验）
    upstream_model      VARCHAR(128),                         -- llm 类填具体上游模型名；其它 NULL
    -- ── 计费规格 ──
    billing_unit        VARCHAR(16)  NOT NULL,               -- 1k_tokens / minute / 1k_chars / image / page
    context_window      INT,                                  -- llm 适用
    max_output_tokens   INT,                                  -- llm 适用
    -- ── 展示 ──
    is_public           BOOLEAN      NOT NULL DEFAULT TRUE,  -- 模型市场可见
    sort_order          INT          NOT NULL DEFAULT 100,
    tags                TEXT[]       NOT NULL DEFAULT '{}',  -- tool_use / json_mode / streaming / vision …
    metadata            JSONB        NOT NULL DEFAULT '{}'::jsonb,
    -- ── 状态 ──
    status              VARCHAR(16)  NOT NULL DEFAULT 'active', -- active / hidden / deprecated
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_platform_services_category
    ON catalog.platform_services(category_id, sort_order)
    WHERE status = 'active' AND is_public = TRUE;
CREATE INDEX idx_platform_services_routing
    ON catalog.platform_services(vendor_product_id, capability)
    WHERE status = 'active';

-- 平台零售价（带历史）：每个 SKU 的 retail 价。
-- 修改保留历史，按 effective_from 取生效价。
CREATE TABLE catalog.platform_pricing (
    id                     BIGSERIAL      PRIMARY KEY,
    platform_service_id    VARCHAR(64)    NOT NULL REFERENCES catalog.platform_services(id),
    -- 不同 billing_unit 下含义不同，每条只填相关列。
    input_per_unit_cents   DECIMAL(12,4),                    -- llm: 输入；asr/tts/mt: 主单价
    output_per_unit_cents  DECIMAL(12,4),                    -- llm: 输出
    image_per_unit_cents   DECIMAL(12,4),                    -- vision / image_gen
    -- 生效时间窗
    effective_from         TIMESTAMPTZ    NOT NULL DEFAULT NOW(),
    effective_until        TIMESTAMPTZ,                       -- NULL = 仍生效
    -- 元信息
    notes                  TEXT,                              -- 调价原因
    created_by             BIGINT,                            -- iam.users.id（运营）
    created_at             TIMESTAMPTZ    NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_platform_pricing_lookup
    ON catalog.platform_pricing(platform_service_id, effective_from DESC);

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS catalog.platform_pricing  CASCADE;
DROP TABLE IF EXISTS catalog.platform_services CASCADE;
-- +goose StatementEnd

-- 回滚到 v0.1 schema（与 00004_catalog.sql 的相关部分一致）
CREATE TABLE catalog.capabilities (
    id              VARCHAR(32) PRIMARY KEY,
    category        VARCHAR(16) NOT NULL,
    display_name    VARCHAR(64) NOT NULL,
    billing_unit    VARCHAR(16) NOT NULL,
    sub_modes       TEXT[] NOT NULL DEFAULT '{}',
    transports      TEXT[] NOT NULL DEFAULT '{http}',
    description     TEXT
);
CREATE TABLE catalog.providers (
    id              VARCHAR(32) PRIMARY KEY,
    display_name    VARCHAR(64) NOT NULL,
    base_url        TEXT NOT NULL,
    auth_mode       VARCHAR(32) NOT NULL,
    protocol_family VARCHAR(32) NOT NULL,
    status          VARCHAR(16) NOT NULL DEFAULT 'active',
    supported_capabilities TEXT[] NOT NULL DEFAULT '{chat}',
    config          JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE catalog.models (
    id              VARCHAR(64) PRIMARY KEY,
    display_name    VARCHAR(128) NOT NULL,
    capability_id   VARCHAR(32) REFERENCES catalog.capabilities(id),
    category        VARCHAR(32) NOT NULL,
    capabilities    TEXT[] NOT NULL DEFAULT '{}',
    context_window  INT,
    max_output      INT,
    is_public       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE TABLE catalog.model_mappings (
    id              BIGSERIAL PRIMARY KEY,
    model_id        VARCHAR(64) NOT NULL REFERENCES catalog.models(id),
    provider_id     VARCHAR(32) NOT NULL REFERENCES catalog.providers(id),
    upstream_model  VARCHAR(128) NOT NULL,
    priority        SMALLINT NOT NULL DEFAULT 10,
    status          VARCHAR(16) NOT NULL DEFAULT 'active',
    notes           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE UNIQUE INDEX uq_model_mapping ON catalog.model_mappings(model_id, provider_id, upstream_model);
CREATE TABLE catalog.pricing (
    id              BIGSERIAL PRIMARY KEY,
    model_id        VARCHAR(64) NOT NULL REFERENCES catalog.models(id),
    provider_id     VARCHAR(32) REFERENCES catalog.providers(id),
    capability_id   VARCHAR(32),
    kind            VARCHAR(16) NOT NULL,
    unit            VARCHAR(16) NOT NULL,
    input_per_1k_cents   DECIMAL(12, 4) NOT NULL DEFAULT 0,
    output_per_1k_cents  DECIMAL(12, 4) NOT NULL DEFAULT 0,
    unit_price_cents     DECIMAL(12, 4) NOT NULL DEFAULT 0,
    effective_from  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    effective_until TIMESTAMPTZ,
    notes           TEXT
);
CREATE INDEX idx_pricing_lookup ON catalog.pricing(model_id, provider_id, kind, effective_from DESC);
CREATE TABLE catalog.voice_mappings (
    id              BIGSERIAL PRIMARY KEY,
    voice_id        VARCHAR(64) REFERENCES catalog.voices(id),
    provider_id     VARCHAR(32) REFERENCES catalog.providers(id),
    upstream_voice  VARCHAR(128) NOT NULL,
    priority        SMALLINT NOT NULL DEFAULT 10,
    notes           TEXT
);
