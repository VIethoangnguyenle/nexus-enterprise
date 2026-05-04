-- Migration 008: Add scope_oa_id to drive_items for NGAC scope-based queries
-- Enables fast department-scoped listing without per-item CheckAccess calls.
-- Each item inherits its scope from parent folder's OA; root folders set it on creation.

ALTER TABLE drive_items ADD COLUMN IF NOT EXISTS scope_oa_id TEXT;

-- Partial index: only active items need scope queries
CREATE INDEX IF NOT EXISTS idx_drive_items_scope
    ON drive_items(scope_oa_id, status, created_at DESC) WHERE status = 'active';
