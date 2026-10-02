-- =============================================
-- Migration 000040: Platform User Sessions
-- =============================================
-- A session is one login of a platform user. Each refresh rotates its refresh
-- token: the presented token is marked rotated_at and a new one is issued in
-- the same session. A rotated token presented again after the reuse grace
-- revokes the whole session. Logout sets revoked_at. Deleting the platform
-- user cascades to its sessions and their tokens.
--
-- expires_at on a session is the expiry of its newest refresh token, so a
-- session whose expires_at has passed holds no usable token and can be pruned.
--
-- The refresh token itself is never stored, only its SHA-256 hash.
--
-- No CHECK constraints and no business triggers.

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
    id VARCHAR(255) PRIMARY KEY, -- KSUID prefix: prtok_, carried as the token's jti
    session_id VARCHAR(255) NOT NULL REFERENCES platform_user_sessions(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    rotated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_platform_user_refresh_tokens_session_id ON platform_user_refresh_tokens(session_id);
