-- Lenders define reusable loan products (amount cap, rate, tenor) on the workspace.
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS loan_products JSONB NOT NULL DEFAULT '[]';
