-- +goose Up
CREATE TABLE wallet.accounts (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES iam.users(id),
    currency        VARCHAR(8) NOT NULL DEFAULT 'CNY',
    balance_cents   BIGINT NOT NULL DEFAULT 0,
    frozen_cents    BIGINT NOT NULL DEFAULT 0,
    total_recharged_cents  BIGINT NOT NULL DEFAULT 0,
    total_spent_cents      BIGINT NOT NULL DEFAULT 0,
    version         INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT ck_balance_nonneg CHECK (balance_cents >= 0),
    CONSTRAINT ck_frozen_range   CHECK (frozen_cents >= 0 AND frozen_cents <= balance_cents)
);
CREATE UNIQUE INDEX uq_wallet_user ON wallet.accounts(user_id, currency);

CREATE TABLE wallet.transactions (
    id              BIGSERIAL PRIMARY KEY,
    account_id      BIGINT NOT NULL REFERENCES wallet.accounts(id),
    type            VARCHAR(32) NOT NULL,
    amount_cents    BIGINT NOT NULL,
    balance_after   BIGINT NOT NULL,
    related_id      VARCHAR(64),
    related_type    VARCHAR(32),
    memo            TEXT,
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_txn_account_created ON wallet.transactions(account_id, created_at DESC);
CREATE INDEX idx_txn_related         ON wallet.transactions(related_type, related_id);

CREATE TABLE wallet.recharges (
    id              BIGSERIAL PRIMARY KEY,
    order_no        VARCHAR(64) UNIQUE NOT NULL,
    user_id         BIGINT NOT NULL REFERENCES iam.users(id),
    amount_cents    BIGINT NOT NULL,
    channel         VARCHAR(32) NOT NULL,
    channel_order_id VARCHAR(128),
    status          VARCHAR(16) NOT NULL DEFAULT 'pending',
    paid_at         TIMESTAMPTZ,
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE wallet.invoices (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES iam.users(id),
    invoice_no      VARCHAR(64) UNIQUE,
    amount_cents    BIGINT NOT NULL,
    title           VARCHAR(256),
    tax_no          VARCHAR(64),
    email           VARCHAR(255),
    status          VARCHAR(16) NOT NULL DEFAULT 'pending',
    issued_at       TIMESTAMPTZ,
    file_url        TEXT,
    metadata        JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE IF EXISTS wallet.invoices;
DROP TABLE IF EXISTS wallet.recharges;
DROP TABLE IF EXISTS wallet.transactions;
DROP TABLE IF EXISTS wallet.accounts;
