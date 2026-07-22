-- Button text colour for borrower link pages (pairs with brand_color).
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS brand_text_color TEXT NOT NULL DEFAULT '';
