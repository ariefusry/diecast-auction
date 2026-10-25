-- How far each side has read a conversation, for unread badges.
CREATE TABLE chat_reads (
    auction_id           BIGINT NOT NULL REFERENCES auctions (id),
    user_id              BIGINT NOT NULL REFERENCES users (id),
    last_read_message_id BIGINT NOT NULL DEFAULT 0,
    PRIMARY KEY (auction_id, user_id)
);
