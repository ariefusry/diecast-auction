-- A message without a sender is written by the system: the notice that opens
-- a chat when an auction is sold, so both sides see an unread badge.
ALTER TABLE messages ALTER COLUMN sender_id DROP NOT NULL;
