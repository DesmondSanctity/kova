-- Repayment tracking (single bullet: principal + interest due at tenor end).
ALTER TABLE requests ADD COLUMN IF NOT EXISTS repayment_total  BIGINT NOT NULL DEFAULT 0;
ALTER TABLE requests ADD COLUMN IF NOT EXISTS repayment_due_at TIMESTAMPTZ;
ALTER TABLE requests ADD COLUMN IF NOT EXISTS repaid           BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE requests ADD COLUMN IF NOT EXISTS repaid_at        TIMESTAMPTZ;
