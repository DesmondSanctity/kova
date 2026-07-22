-- Immutable audit trail of key lender/borrower actions.
CREATE TABLE IF NOT EXISTS audit_events (
    id           TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    actor        TEXT NOT NULL DEFAULT '',
    action       TEXT NOT NULL,
    target       TEXT NOT NULL DEFAULT '',
    detail       TEXT NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_audit_ws ON audit_events(workspace_id, created_at DESC);
