-- 020_personal_ua_unique.sql
--
-- A person's personal user attribute (properties type = personal_ua) is how an
-- item is shared with exactly one person. There must be at most one per user,
-- or two concurrent first shares to the same person would create two, and the
-- lookup that reuses it would be ambiguous.

CREATE UNIQUE INDEX IF NOT EXISTS uniq_ngac_personal_ua_per_user
    ON ngac_nodes ((properties->>'user_node_id'))
    WHERE properties->>'type' = 'personal_ua';
