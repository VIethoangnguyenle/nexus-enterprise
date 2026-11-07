-- ListMessages by number (after 072), on the company-wide channel: a page of 50 below a random number.
\set s random(1000, 1000000)
SELECT m.id, m.channel_id, m.sender_id, COALESCE(u.username,''), m.content, m.created_at,
COALESCE(m.message_type,'user'), COALESCE(m.parent_message_id,''),
COALESCE(m.linked_entity_type,''), COALESCE(m.linked_entity_id,''), m.reply_count,
COALESCE(m.content_format,'markdown'), COALESCE(m.mentions, '{}'), m.last_reply_at,
m.edited_at, m.deleted_at IS NOT NULL, COALESCE(m.quoted_message_id, ''),
COALESCE(m.seq, 0), COALESCE(m.reply_seq, 0)
FROM messages m LEFT JOIN users u ON m.sender_id = u.id
WHERE m.channel_id = 'c1_0' AND m.parent_message_id IS NULL
  AND (m.deleted_at IS NULL OR m.reply_count > 0)
  AND m.seq < :s
ORDER BY m.seq DESC LIMIT 51;
