-- +goose Up
-- v0.2 重构对 metering.call_logs 的影响：调度命中的链路从原来的
-- (provider, pool_account) 升级为五层 (vendor, vendor_account,
-- product, credential, binding) + 平台侧 platform_service_id。
-- 旧字段（model_id / provider_id / pool_account_id）保留作为兼容期，
-- 由后续 release 在确认无回滚需求后清理。
--
-- 见 docs/03-核心数据结构设计-v2.md §metering.call_logs。

ALTER TABLE metering.call_logs
    ADD COLUMN platform_service_id VARCHAR(64),
    ADD COLUMN vendor_id           VARCHAR(32),
    ADD COLUMN vendor_account_id   BIGINT,
    ADD COLUMN product_id          VARCHAR(64),
    ADD COLUMN credential_id       BIGINT,
    ADD COLUMN binding_id          BIGINT;

-- 显式标注 deprecated（注释，无运行时影响）
COMMENT ON COLUMN metering.call_logs.model_id        IS 'DEPRECATED: 用 platform_service_id 替代';
COMMENT ON COLUMN metering.call_logs.provider_id     IS 'DEPRECATED: 用 vendor_id + product_id 替代';
COMMENT ON COLUMN metering.call_logs.pool_account_id IS 'DEPRECATED: 用 binding_id 替代';

-- 新调度键索引（旧索引 idx_calls_provider_ts / idx_calls_pool_ts 保留至下个 release）
CREATE INDEX idx_calls_sku_ts  ON metering.call_logs(platform_service_id, ts DESC);
CREATE INDEX idx_calls_cred_ts ON metering.call_logs(credential_id, ts DESC);
CREATE INDEX idx_calls_acct_ts ON metering.call_logs(vendor_account_id, ts DESC);

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS metering.idx_calls_acct_ts;
DROP INDEX IF EXISTS metering.idx_calls_cred_ts;
DROP INDEX IF EXISTS metering.idx_calls_sku_ts;
-- +goose StatementEnd

ALTER TABLE metering.call_logs
    DROP COLUMN IF EXISTS binding_id,
    DROP COLUMN IF EXISTS credential_id,
    DROP COLUMN IF EXISTS product_id,
    DROP COLUMN IF EXISTS vendor_account_id,
    DROP COLUMN IF EXISTS vendor_id,
    DROP COLUMN IF EXISTS platform_service_id;

COMMENT ON COLUMN metering.call_logs.model_id        IS NULL;
COMMENT ON COLUMN metering.call_logs.provider_id     IS NULL;
COMMENT ON COLUMN metering.call_logs.pool_account_id IS NULL;
