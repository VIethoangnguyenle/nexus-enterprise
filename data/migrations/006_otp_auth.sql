-- Migration 006: OTP Authentication Support
-- Adds phone column and relaxes password NOT NULL for OTP-only users.

-- Phone number column for OTP login
ALTER TABLE users ADD COLUMN IF NOT EXISTS phone VARCHAR(20);
CREATE UNIQUE INDEX IF NOT EXISTS idx_users_phone ON users (phone) WHERE phone IS NOT NULL AND phone != '';

-- Allow NULL password for OTP-only users (no password set)
ALTER TABLE users ALTER COLUMN password DROP NOT NULL;
ALTER TABLE users ALTER COLUMN password SET DEFAULT '';
