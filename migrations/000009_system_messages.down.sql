DELETE FROM messages WHERE sender_id IS NULL;
ALTER TABLE messages ALTER COLUMN sender_id SET NOT NULL;
