-- =============================================
-- Migration 000040: Platform User Sessions
-- =============================================
-- One session per login of a platform user. Each refresh rotates its refresh
-- token; a token row's id is the token's jti.

CREATE TABLE platform_user_sessions (
    id VARCHAR(255) PRIMARY KEY, -- KSUID prefix: psess_
    platform_user_id VARCHAR(255) NOT NULL REFERENCES platform_users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_platform_user_sessions_platform_user_id ON platform_user_sessions(platform_user_id);

CREATE TRIGGER update_platform_user_sessions_updated_at BEFORE UPDATE ON platform_user_sessions FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TABLE platform_user_refresh_tokens (
    id VARCHAR(255) PRIMARY KEY, -- KSUID prefix: prtok_
    session_id VARCHAR(255) NOT NULL REFERENCES platform_user_sessions(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    rotated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_platform_user_refresh_tokens_session_id ON platform_user_refresh_tokens(session_id);

CREATE TRIGGER update_platform_user_refresh_tokens_updated_at BEFORE UPDATE ON platform_user_refresh_tokens FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
