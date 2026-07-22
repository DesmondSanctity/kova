-- Per-workspace branding applied to borrower/lender link pages.
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS brand_name    TEXT NOT NULL DEFAULT '';
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS brand_color   TEXT NOT NULL DEFAULT '';
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS support_email TEXT NOT NULL DEFAULT '';
