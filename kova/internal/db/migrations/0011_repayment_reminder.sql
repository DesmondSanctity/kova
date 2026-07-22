-- Track when a repayment reminder email was last sent (scheduler + manual trigger).
ALTER TABLE requests ADD COLUMN IF NOT EXISTS repayment_reminded_at TIMESTAMPTZ;
