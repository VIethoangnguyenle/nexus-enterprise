-- Migration 005: Multi-Tenant Auth
-- Adds multi-tenant identity model: email/union_id on users, domain on workspaces,
-- tenant_users membership table with tenant-scoped open_id.

-- ============================================
-- 1. Extend users table
-- ============================================

ALTER TABLE users ADD COLUMN IF NOT EXISTS email TEXT UNIQUE;
ALTER TABLE users ADD COLUMN IF NOT EXISTS union_id TEXT UNIQUE DEFAULT gen_random_uuid()::TEXT;
ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name TEXT DEFAULT '';

-- ============================================
-- 2. Extend workspaces table with domain
-- ============================================

ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS domain TEXT;
CREATE INDEX IF NOT EXISTS idx_workspaces_domain ON workspaces(domain) WHERE domain IS NOT NULL;

-- ============================================
-- 3. Tenant membership table
-- ============================================

CREATE TABLE IF NOT EXISTS tenant_users (
    tenant_id    TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id      TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role         TEXT NOT NULL DEFAULT 'member' CHECK (role IN ('owner','admin','member')),
    status       TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','invited','disabled')),
    open_id      TEXT NOT NULL DEFAULT gen_random_uuid()::TEXT,
    ngac_node_id TEXT REFERENCES ngac_nodes(id),
    joined_at    TIMESTAMPTZ DEFAULT NOW(),
    PRIMARY KEY (tenant_id, user_id)
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_tenant_users_open_id ON tenant_users(open_id);
CREATE INDEX IF NOT EXISTS idx_tenant_users_user ON tenant_users(user_id);

-- ============================================
-- 4. Backfill: create tenant_users for existing workspace owners
-- ============================================

INSERT INTO tenant_users (tenant_id, user_id, role, status, ngac_node_id)
SELECT w.id, w.owner_id, 'owner', 'active', u.ngac_node
FROM workspaces w JOIN users u ON w.owner_id = u.id
ON CONFLICT DO NOTHING;
