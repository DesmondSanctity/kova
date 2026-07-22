-- Move password resets from single-use link tokens to short-lived OTP codes.
ALTER TABLE password_resets ADD COLUMN IF NOT EXISTS code       TEXT NOT NULL DEFAULT '';
ALTER TABLE password_resets ADD COLUMN IF NOT EXISTS created_at TIMESTAMPTZ NOT NULL DEFAULT now();
CREATE INDEX IF NOT EXISTS idx_password_resets_user ON password_resets(user_id);
