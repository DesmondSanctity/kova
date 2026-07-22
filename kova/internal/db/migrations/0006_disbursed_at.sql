-- Timestamp when a loan was actually paid out (for accurate reporting).
ALTER TABLE requests ADD COLUMN IF NOT EXISTS disbursed_at TIMESTAMPTZ;
