-- Lender-configurable auto-decline threshold. 0 means "use the platform default".
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS min_score INT NOT NULL DEFAULT 0;
