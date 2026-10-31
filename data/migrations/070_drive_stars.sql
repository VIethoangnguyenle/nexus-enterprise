-- 070_drive_stars.sql
--
-- Tài liệu on the phone (docs/specs/drive-tree-navigation, drive-permission-engine):
--   drive_stars          one row per person per starred file or folder. A star is personal: only the person who
--                        set it ever reads it, and it grants nothing. Every list built from it re-checks read on
--                        each item through the policy service, so a star on an item the person can no longer read
--                        is kept but not shown.
--   drive_items_pending  partial index for the drive service's hourly cleanup of uploads left `pending` > 24h.
--
-- Re-runnable: every statement is IF NOT EXISTS, so `make db-migrate`, which re-applies every file, is harmless.
--
-- Reverse (additive; the release before this one never reads it):
--   DROP TABLE IF EXISTS drive_stars;
--   DROP INDEX IF EXISTS drive_items_pending_created;

BEGIN;

CREATE TABLE IF NOT EXISTS drive_stars (
    user_id    TEXT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    item_id    TEXT        NOT NULL REFERENCES drive_items(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, item_id)
);

-- A person's stars, newest first (Gắn sao, Truy cập nhanh).
CREATE INDEX IF NOT EXISTS drive_stars_user_created ON drive_stars (user_id, created_at DESC);
-- Deleting an item removes its stars; without this the cascade scans the table.
CREATE INDEX IF NOT EXISTS drive_stars_item ON drive_stars (item_id);

-- The stale-upload cleanup reads only pending rows, oldest first.
CREATE INDEX IF NOT EXISTS drive_items_pending_created ON drive_items (created_at) WHERE status = 'pending';

COMMIT;
