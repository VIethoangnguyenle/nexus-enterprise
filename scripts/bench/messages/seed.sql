-- Seeds a messaging data set shaped like a platform with many tenants, for
-- benchmarking the message queries. Run only against the throwaway ngac_bench
-- database (scripts/bench/messages/run.sh does).
--
-- psql variables:
--   tenants   number of workspaces (default 2000)
--   messages  total messages (default 50000000)
--
-- Shape:
--   w1 is the big tenant: 5,000 people, one company-wide channel c1_0 that
--   everyone is in and that holds 1/50 of all messages, and 200 more channels
--   c1_1..c1_200 that the first 1,000 people (the "viewers") are in.
--   Every other tenant has 50 people and 10 channels everyone is in.
--   5% of messages are thread replies, 1% are deleted, 2% mention someone.
\set ON_ERROR_STOP on
\if :{?tenants} \else \set tenants 2000 \endif
\if :{?messages} \else \set messages 50000000 \endif

SET synchronous_commit = off;
SET maintenance_work_mem = '1GB';

-- People ---------------------------------------------------------------------
INSERT INTO users (id, username)
SELECT 'u1_' || i, 'u1_' || i FROM generate_series(1, 5000) i;
INSERT INTO users (id, username)
SELECT 'u' || t || '_' || i, 'u' || t || '_' || i
FROM generate_series(2, :tenants) t, generate_series(1, 50) i;

-- Workspaces and channels (one workspace-wide channel each; the rest are groups) --
INSERT INTO workspaces (id, name, owner_id, type)
SELECT 'w' || t, 'Workspace ' || t, 'u' || t || '_1', 'organization'
FROM generate_series(1, :tenants) t;

INSERT INTO channels (id, name, channel_type, workspace_id)
SELECT 'c1_' || j, 'Channel ' || j, CASE WHEN j = 0 THEN 'workspace' ELSE 'private' END, 'w1'
FROM generate_series(0, 200) j;
INSERT INTO channels (id, name, channel_type, workspace_id)
SELECT 'c' || t || '_' || j, 'Channel ' || j, CASE WHEN j = 0 THEN 'workspace' ELSE 'private' END, 'w' || t
FROM generate_series(2, :tenants) t, generate_series(0, 9) j;

-- Memberships (the cache; keyed by the person's node, here their user id) ----
INSERT INTO channel_members (channel_id, ngac_node_id)
SELECT 'c1_0', 'u1_' || i FROM generate_series(1, 5000) i;
INSERT INTO channel_members (channel_id, ngac_node_id)
SELECT 'c1_' || j, 'u1_' || i FROM generate_series(1, 200) j, generate_series(1, 1000) i;
INSERT INTO channel_members (channel_id, ngac_node_id)
SELECT 'c' || t || '_' || j, 'u' || t || '_' || i
FROM generate_series(2, :tenants) t, generate_series(0, 9) j, generate_series(1, 50) i;

-- Messages -------------------------------------------------------------------
-- Big channel: ids mb<n>. Every 20th message replies to the one before it,
-- which is never itself a reply.
INSERT INTO messages (id, channel_id, sender_id, content, content_text, created_at,
                      parent_message_id, mentions, deleted_at)
SELECT 'mb' || n, 'c1_0', 'u1_' || (n % 5000 + 1), 'message ' || n, 'message ' || n,
       TIMESTAMPTZ '2025-01-01' + n * (INTERVAL '365 days' / (:messages / 50)),
       CASE WHEN n % 20 = 0 THEN 'mb' || (n - 1) END,
       CASE WHEN n % 50 = 3 THEN ARRAY['u1_' || ((n / 50) % 1000 + 1)] ELSE '{}' END,
       CASE WHEN n % 100 = 7 THEN TIMESTAMPTZ '2026-01-01' END
FROM generate_series(1, :messages / 50) n;

-- Every other channel: ids mg<n>, channel n mod C. A reply answers the message
-- C before it, in the same channel, when that one is not a reply itself.
-- psql does no arithmetic: these are SQL expressions pasted in, so they carry their parentheses.
\set channels_other '((' :tenants ' - 1) * 10 + 200)'
\set general '(' :messages ' - ' :messages ' / 50)'
INSERT INTO messages (id, channel_id, sender_id, content, content_text, created_at,
                      parent_message_id, mentions, deleted_at)
SELECT 'mg' || n,
       CASE WHEN k < 200 THEN 'c1_' || (k + 1) ELSE 'c' || (2 + (k - 200) / 10) || '_' || ((k - 200) % 10) END,
       CASE WHEN k < 200 THEN 'u1_' || (n % 1000 + 1) ELSE 'u' || (2 + (k - 200) / 10) || '_' || (n % 50 + 1) END,
       'message ' || n, 'message ' || n,
       TIMESTAMPTZ '2025-01-01' + n * (INTERVAL '365 days' / :general),
       CASE WHEN n % 20 = 0 AND n > :channels_other AND (n - :channels_other) % 20 <> 0
            THEN 'mg' || (n - :channels_other) END,
       CASE WHEN n % 50 = 3 THEN
         ARRAY[CASE WHEN k < 200 THEN 'u1_' || ((n / 50) % 1000 + 1) ELSE 'u' || (2 + (k - 200) / 10) || '_1' END]
       ELSE '{}' END,
       CASE WHEN n % 100 = 7 THEN TIMESTAMPTZ '2026-01-01' END
FROM (SELECT n, n % :channels_other AS k FROM generate_series(1, :general) n) s;

-- Thread summaries on the roots.
UPDATE messages r SET reply_count = c.n, last_reply_at = c.last
FROM (SELECT parent_message_id AS id, count(*) AS n, max(created_at) AS last
      FROM messages WHERE parent_message_id IS NOT NULL GROUP BY 1) c
WHERE r.id = c.id;

-- Read positions: viewers have read their channels up to 0..29 days ago;
-- the other people of the big tenant have read the company channel.
INSERT INTO read_receipts (user_id, channel_id, last_read_at)
SELECT 'u1_' || i, 'c1_' || j, TIMESTAMPTZ '2026-01-01' - ((i * 7 + j) % 30) * INTERVAL '1 day'
FROM generate_series(1, 1000) i, generate_series(0, 200) j;
INSERT INTO read_receipts (user_id, channel_id, last_read_at)
SELECT 'u1_' || i, 'c1_0', TIMESTAMPTZ '2026-01-01' - (i % 30) * INTERVAL '1 day'
FROM generate_series(1001, 5000) i;

-- Each viewer follows 20 threads of the company channel (roots are mb<20k+19>).
INSERT INTO thread_follows (user_id, root_message_id, following, last_read_at)
SELECT 'u1_' || i, 'mb' || (20 * (i + f) + 19), TRUE, TIMESTAMPTZ '2025-12-01'
FROM generate_series(1, 1000) i, generate_series(0, 19) f
WHERE 20 * (i + f) + 19 <= :messages / 50;

-- A tenth of the viewers hear every reply of the company channel.
INSERT INTO channel_notification_levels (user_id, channel_id, level)
SELECT 'u1_' || i, 'c1_0', 'all' FROM generate_series(1, 100) i;

ANALYZE;
