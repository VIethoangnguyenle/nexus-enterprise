-- Migration 009: User Profile & Workspace Type for Nexus Hub
-- Adds profile enrichment fields to users and type classification to workspaces.

-- ============================================
-- User Profile Fields
-- ============================================
ALTER TABLE users ADD COLUMN IF NOT EXISTS email      TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS title      TEXT DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS department TEXT DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS location   TEXT DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url TEXT DEFAULT '';
ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name TEXT DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_users_department ON users(department) WHERE department != '';
CREATE INDEX IF NOT EXISTS idx_users_location   ON users(location)   WHERE location != '';

-- ============================================
-- Workspace Type
-- ============================================
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS type TEXT NOT NULL DEFAULT 'personal'
  CHECK (type IN ('personal', 'organization'));

CREATE INDEX IF NOT EXISTS idx_workspaces_type ON workspaces(type);
