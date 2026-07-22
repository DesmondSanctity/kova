-- Final settlement status from Monnify disbursement webhooks.
ALTER TABLE requests ADD COLUMN IF NOT EXISTS disbursement_status TEXT NOT NULL DEFAULT '';
