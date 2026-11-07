-- SearchAllMessages for a word every message holds ("message"): the worst case, where the
-- full-text index matches every tenant's messages before the channel filter. pgbench script.
SELECT m.id, m.channel_id, m.sender_id, COALESCE(u.username,''), m.content, m.created_at,
COALESCE(m.message_type,'user'), COALESCE(m.parent_message_id,''),
COALESCE(m.linked_entity_type,''), COALESCE(m.linked_entity_id,''), m.reply_count,
COALESCE(m.content_format,'markdown'), COALESCE(m.mentions, '{}'), m.last_reply_at,
m.edited_at, m.deleted_at IS NOT NULL, COALESCE(m.quoted_message_id, '')
FROM messages m LEFT JOIN users u ON m.sender_id = u.id
WHERE m.channel_id = ANY(ARRAY(SELECT 'c1_' || g FROM generate_series(0, 200) g)) AND m.deleted_at IS NULL
  AND m.search_vector @@ (plainto_tsquery('simple', '') && to_tsquery('simple', '''messa'':*'))
ORDER BY m.created_at DESC, m.id DESC LIMIT 26;
