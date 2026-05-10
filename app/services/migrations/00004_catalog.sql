-- +goose Up
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

CREATE TABLE catalog.voices (
    id              VARCHAR(64) PRIMARY KEY,
    display_name    VARCHAR(128) NOT NULL,
    language        VARCHAR(16) NOT NULL,
    gender          VARCHAR(8),
    style_tags      TEXT[] NOT NULL DEFAULT '{}',
    sample_url      TEXT,
    is_premium      BOOLEAN NOT NULL DEFAULT FALSE,
    status          VARCHAR(16) NOT NULL DEFAULT 'active',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE catalog.voice_mappings (
    id              BIGSERIAL PRIMARY KEY,
    voice_id        VARCHAR(64) REFERENCES catalog.voices(id),
    provider_id     VARCHAR(32) REFERENCES catalog.providers(id),
    upstream_voice  VARCHAR(128) NOT NULL,
    priority        SMALLINT NOT NULL DEFAULT 10,
    notes           TEXT
);

CREATE TABLE catalog.glossaries (
    id              VARCHAR(64) PRIMARY KEY,
    user_id         BIGINT REFERENCES iam.users(id),
    name            VARCHAR(128),
    source_lang     VARCHAR(16) NOT NULL,
    target_lang     VARCHAR(16) NOT NULL,
    entries         JSONB NOT NULL DEFAULT '[]'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS catalog.glossaries;
DROP TABLE IF EXISTS catalog.voice_mappings;
DROP TABLE IF EXISTS catalog.voices;
DROP TABLE IF EXISTS catalog.pricing;
DROP TABLE IF EXISTS catalog.model_mappings;
DROP TABLE IF EXISTS catalog.models;
DROP TABLE IF EXISTS catalog.providers;
DROP TABLE IF EXISTS catalog.capabilities;
