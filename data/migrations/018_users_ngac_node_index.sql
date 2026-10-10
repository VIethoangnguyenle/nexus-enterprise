-- users(ngac_node) had no index. The drive resolves owner display names with a
-- filter on it for every folder listing, and the foreign key to ngac_nodes
-- scans it on every node delete; both were sequential scans of users.

CREATE INDEX IF NOT EXISTS idx_users_ngac_node ON users (ngac_node);
