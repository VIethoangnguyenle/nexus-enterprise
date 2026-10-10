-- Retire the database-backed decision cache (L2) and the graph version counter
-- that only it read.
--
-- A runtime access check never touches the database: decisions are cached in
-- Redis and invalidated by the EPP path. The L2 table also stored only
-- ALLOW/DENY, dropping the explanation, and its workspace_id column was never
-- created by any applied migration, so every L2 read and write already failed.
-- Nothing reads ngac_graph_version once L2 is gone.

DROP TABLE IF EXISTS ngac_materialized_access;
DROP TABLE IF EXISTS ngac_graph_version;
