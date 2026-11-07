-- GetUnreadCounts by numbers (after 072), for one viewer of the big tenant over its 201 channels.
-- Viewers 1-100 hear every reply of the company channel (level all); the others the main stream.
\set i random(1, 1000)
WITH followed AS (
   SELECT tf.root_message_id AS id, tf.last_read_at
   FROM thread_follows tf WHERE tf.user_id = ('u1_' || :i) AND tf.following
   UNION ALL
   SELECT m.id, m.created_at FROM messages m
   WHERE m.parent_message_id IS NULL AND m.reply_count > 0
     AND (m.sender_id = ('u1_' || :i) OR m.mentions @> ARRAY[('u1_' || :i)]::text[])
     AND NOT EXISTS (SELECT 1 FROM thread_follows tf WHERE tf.user_id = ('u1_' || :i) AND tf.root_message_id = m.id)
 ), thread_unread AS (
   SELECT root.channel_id, COUNT(r.id)::int AS n
   FROM followed f
   JOIN messages root ON root.id = f.id
   JOIN messages r ON r.parent_message_id = f.id AND r.sender_id <> ('u1_' || :i) AND r.deleted_at IS NULL
     AND r.created_at > f.last_read_at
   GROUP BY root.channel_id
 ), mentioning AS MATERIALIZED (
   -- The live main messages that mention the user, found through the
   -- mentions index alone: a person is mentioned far less often than
   -- their channels hold messages, and letting the planner combine this
   -- with the channel index read every message of every channel.
   SELECT m.id, m.channel_id, m.seq FROM messages m
   WHERE m.mentions @> ARRAY[('u1_' || :i)]::text[] AND m.parent_message_id IS NULL
     AND m.deleted_at IS NULL AND m.sender_id <> ('u1_' || :i)
 ), mention_unread AS (
   -- Of those, the ones unread for them: numbered after the channel read
   -- number (as the unread count is), or mentioning them by an edit made
   -- after the read time.
   SELECT m.channel_id, COUNT(*)::int AS n
   FROM mentioning m
   LEFT JOIN read_receipts rr3 ON rr3.user_id = ('u1_' || :i) AND rr3.channel_id = m.channel_id
   LEFT JOIN message_mention_times mt ON mt.message_id = m.id AND mt.user_id = ('u1_' || :i)
   WHERE m.channel_id = ANY(ARRAY(SELECT 'c1_' || g FROM generate_series(0, 200) g))
     AND (m.seq > COALESCE(rr3.last_read_seq, 0)
          OR mt.mentioned_at > COALESCE(rr3.last_read_at, '-infinity'::timestamptz))
   GROUP BY m.channel_id
 ), levels AS (
   SELECT channel_id, level FROM channel_notification_levels
   WHERE user_id = ('u1_' || :i) AND channel_id = ANY(ARRAY(SELECT 'c1_' || g FROM generate_series(0, 200) g))
 ), all_thread_unread AS (
   -- "all": every reply by someone else counts — after the thread's own
   -- read position when the user follows it, else after the channel's.
   SELECT r.channel_id, COUNT(r.id)::int AS n
   FROM messages r
   JOIN levels l ON l.channel_id = r.channel_id AND l.level = 'all'
   LEFT JOIN read_receipts rr2 ON rr2.user_id = ('u1_' || :i) AND rr2.channel_id = r.channel_id
   LEFT JOIN followed f ON f.id = r.parent_message_id
   WHERE r.parent_message_id IS NOT NULL AND r.sender_id <> ('u1_' || :i) AND r.deleted_at IS NULL
     AND r.created_at > COALESCE(f.last_read_at, rr2.last_read_at, '-infinity'::timestamptz)
   GROUP BY r.channel_id
 )
 SELECT cm.channel_id,
        -- Main messages after the read number, less the deleted ones among them:
        -- a subtraction, not a count of the unread rows (deleting is rare, and
        -- the deleted rows have an index of their own).
        CASE WHEN l.level = 'off' THEN 0 ELSE GREATEST(
          COALESCE(c.last_seq, 0) - COALESCE(rr.last_read_seq, 0)
          - (SELECT COUNT(*) FROM messages d
             WHERE d.channel_id = cm.channel_id AND d.parent_message_id IS NULL
               AND d.deleted_at IS NOT NULL AND d.seq > COALESCE(rr.last_read_seq, 0)),
          0) END::int AS unread_count,
        COALESCE(rr.last_message_id, '') AS last_read_msg,
        CASE WHEN l.level = 'off' THEN 0
             WHEN l.level = 'all' THEN COALESCE(atu.n, 0)
             ELSE COALESCE(tu.n, 0) END AS thread_unread,
        COALESCE(l.level, 'main') AS level,
        COALESCE(mu.n, 0) AS mention_count,
        COALESCE(c.last_seq, 0) AS last_seq,
        COALESCE(rr.last_read_seq, 0) AS last_read_seq
 FROM (SELECT DISTINCT unnest(ARRAY(SELECT 'c1_' || g FROM generate_series(0, 200) g)) AS channel_id) cm
 LEFT JOIN channels c ON c.id = cm.channel_id
 LEFT JOIN read_receipts rr ON rr.user_id = ('u1_' || :i) AND rr.channel_id = cm.channel_id
 LEFT JOIN thread_unread tu ON tu.channel_id = cm.channel_id
 LEFT JOIN all_thread_unread atu ON atu.channel_id = cm.channel_id
 LEFT JOIN mention_unread mu ON mu.channel_id = cm.channel_id
 LEFT JOIN levels l ON l.channel_id = cm.channel_id
;
