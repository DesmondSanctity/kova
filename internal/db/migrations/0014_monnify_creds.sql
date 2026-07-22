-- Per-lender Monnify credentials. Each workspace disburses/collects on its own
-- Monnify account. API key and secret are stored encrypted (AES-256-GCM); the
-- contract code and wallet account are non-secret identifiers.
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS monnify_base_url        TEXT NOT NULL DEFAULT '';
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS monnify_api_key_enc     TEXT NOT NULL DEFAULT '';
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS monnify_secret_key_enc  TEXT NOT NULL DEFAULT '';
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS monnify_contract_code   TEXT NOT NULL DEFAULT '';
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS monnify_wallet_account  TEXT NOT NULL DEFAULT '';
