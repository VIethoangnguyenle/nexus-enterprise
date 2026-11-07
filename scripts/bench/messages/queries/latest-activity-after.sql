-- (unchanged by 072) LatestActivity as the store runs it today (store/activity.go): newest main message of each
-- of a viewer's 201 channels. pgbench script.
SELECT m.id, m.channel_id, m.sender_id, COALESCE(u.username,''), m.content, m.created_at,
COALESCE(m.message_type,'user'), COALESCE(m.parent_message_id,''),
COALESCE(m.linked_entity_type,''), COALESCE(m.linked_entity_id,''), m.reply_count,
COALESCE(m.content_format,'markdown'), COALESCE(m.mentions, '{}'), m.last_reply_at,
m.edited_at, m.deleted_at IS NOT NULL, COALESCE(m.quoted_message_id, '')
FROM unnest(ARRAY(SELECT 'c1_' || g FROM generate_series(0, 200) g)) AS c(id)
CROSS JOIN LATERAL (
  SELECT x.* FROM messages x
  WHERE x.channel_id = c.id AND x.parent_message_id IS NULL AND x.deleted_at IS NULL
  ORDER BY x.created_at DESC, x.id DESC LIMIT 1
) m
LEFT JOIN users u ON m.sender_id = u.id;
