-- +goose Up
-- v0.3 后台管理员独立用户体系。
-- 后台管理员 (运营 / 客服 / 财务) 与终端用户 (iam.users) 是两套不同的身份：
--   - 终端用户：注册自营销站，有钱包/订阅/API key/调用记录
--   - 后台管理员：内部员工，账号+密码登录，scope = 平台运维操作
-- 不混在 iam.users 里，避免「is_admin 标志」越界（admin 不该有钱包/订阅/SDK）。

CREATE SCHEMA IF NOT EXISTS adminauth;

-- +goose StatementBegin
CREATE TABLE adminauth.admins (
    id              BIGSERIAL PRIMARY KEY,
    account         VARCHAR(128) NOT NULL UNIQUE,        -- 登录账号 (邮箱 / 用户名)
    password_hash   VARCHAR(256) NOT NULL,                -- argon2id 编码
    display_name    VARCHAR(64),
    status          VARCHAR(16) NOT NULL DEFAULT 'active',-- active / disabled
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login_at   TIMESTAMPTZ
);
CREATE INDEX idx_adminauth_admins_status ON adminauth.admins(status) WHERE status <> 'active';
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE adminauth.sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    admin_id        BIGINT NOT NULL REFERENCES adminauth.admins(id) ON DELETE CASCADE,
    token_hash      CHAR(64) NOT NULL UNIQUE,             -- sha256 hex
    user_agent      TEXT,
    client_ip       INET,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL,
    revoked_at      TIMESTAMPTZ
);
CREATE INDEX idx_adminauth_sessions_admin ON adminauth.sessions(admin_id, expires_at DESC);
CREATE INDEX idx_adminauth_sessions_live  ON adminauth.sessions(expires_at) WHERE revoked_at IS NULL;
-- +goose StatementEnd

-- +goose Down
DROP TABLE IF EXISTS adminauth.sessions;
DROP TABLE IF EXISTS adminauth.admins;
DROP SCHEMA IF EXISTS adminauth;
