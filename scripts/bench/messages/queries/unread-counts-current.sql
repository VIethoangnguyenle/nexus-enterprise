-- GetUnreadCounts as the store runs it today (store/reactions_pins_receipts.go), for one viewer
-- of the big tenant over its 201 channels. pgbench script: one random viewer per run.
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
 ), mention_unread AS (
   -- Main messages that mention the user and are unread for them: after
   -- the channel read position from when the message was sent, or from
   -- the edit that added the user to it.
   SELECT m.channel_id, COUNT(*)::int AS n
   FROM messages m
   LEFT JOIN read_receipts rr3 ON rr3.user_id = ('u1_' || :i) AND rr3.channel_id = m.channel_id
   LEFT JOIN message_mention_times mt ON mt.message_id = m.id AND mt.user_id = ('u1_' || :i)
   WHERE m.channel_id = ANY(ARRAY(SELECT 'c1_' || g FROM generate_series(0, 200) g)) AND m.parent_message_id IS NULL AND m.deleted_at IS NULL
     AND m.mentions @> ARRAY[('u1_' || :i)]::text[] AND m.sender_id <> ('u1_' || :i)
     AND GREATEST(m.created_at, mt.mentioned_at) > COALESCE(rr3.last_read_at, '-infinity'::timestamptz)
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
        CASE WHEN l.level = 'off' THEN 0 ELSE COUNT(m.id) END AS unread_count,
        COALESCE(rr.last_message_id, '') AS last_read_msg,
        CASE WHEN l.level = 'off' THEN 0
             WHEN l.level = 'all' THEN COALESCE(MAX(atu.n), 0)
             ELSE COALESCE(MAX(tu.n), 0) END AS thread_unread,
        COALESCE(l.level, 'main') AS level,
        COALESCE(MAX(mu.n), 0) AS mention_count
 FROM (SELECT DISTINCT unnest(ARRAY(SELECT 'c1_' || g FROM generate_series(0, 200) g)) AS channel_id) cm
 LEFT JOIN read_receipts rr ON rr.user_id = ('u1_' || :i) AND rr.channel_id = cm.channel_id
 LEFT JOIN messages m ON m.channel_id = cm.channel_id
   AND m.parent_message_id IS NULL AND m.deleted_at IS NULL
   AND (rr.last_read_at IS NULL OR m.created_at > rr.last_read_at)
 LEFT JOIN thread_unread tu ON tu.channel_id = cm.channel_id
 LEFT JOIN all_thread_unread atu ON atu.channel_id = cm.channel_id
 LEFT JOIN mention_unread mu ON mu.channel_id = cm.channel_id
 LEFT JOIN levels l ON l.channel_id = cm.channel_id
 GROUP BY cm.channel_id, rr.last_message_id, l.level
;
