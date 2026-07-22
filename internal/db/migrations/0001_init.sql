-- Kova schema. Workspaces are the top-level tenant; users belong to workspaces
-- through memberships (single workspace per user for now).

CREATE TABLE IF NOT EXISTS users (
    id            TEXT PRIMARY KEY,
    email         TEXT UNIQUE NOT NULL,
    name          TEXT NOT NULL DEFAULT '',
    password_hash BYTEA,
    github_id     TEXT UNIQUE,
    avatar_url    TEXT NOT NULL DEFAULT '',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS sessions (
    token      TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS password_resets (
    token      TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    used       BOOLEAN NOT NULL DEFAULT false
);

CREATE TABLE IF NOT EXISTS workspaces (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    org_name   TEXT NOT NULL DEFAULT '',
    use_case   TEXT NOT NULL DEFAULT 'fintech', -- 'fintech' | 'individual'
    plan       TEXT NOT NULL DEFAULT 'pilot',
    owner_id   TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS workspace_members (
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role         TEXT NOT NULL DEFAULT 'owner',
    PRIMARY KEY (workspace_id, user_id)
);

CREATE TABLE IF NOT EXISTS api_keys (
    id              TEXT PRIMARY KEY,
    workspace_id    TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    publishable     TEXT UNIQUE NOT NULL,
    secret          TEXT UNIQUE NOT NULL,
    allowed_domains TEXT[] NOT NULL DEFAULT '{}',
    allowed_ips     TEXT[] NOT NULL DEFAULT '{}',
    calls           BIGINT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at      TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS idx_api_keys_workspace ON api_keys(workspace_id);

CREATE TABLE IF NOT EXISTS requests (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT REFERENCES workspaces(id) ON DELETE CASCADE,
    note         TEXT NOT NULL DEFAULT '',
    status       TEXT NOT NULL DEFAULT 'pending', -- 'pending' | 'completed'
    report       JSONB,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_requests_workspace ON requests(workspace_id);

CREATE TABLE IF NOT EXISTS usage_events (
    id           BIGSERIAL PRIMARY KEY,
    workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    key_id       TEXT,
    kind         TEXT NOT NULL, -- 'score' | 'verify' | 'disburse'
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_usage_workspace_time ON usage_events(workspace_id, created_at);
