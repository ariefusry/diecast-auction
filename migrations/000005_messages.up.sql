-- Buyer and seller arrange shipping after an auction is won. Only those two
-- may write; the service checks that, the table keeps the text sane.
CREATE TABLE messages (
    id         BIGSERIAL PRIMARY KEY,
    auction_id BIGINT NOT NULL REFERENCES auctions (id),
    sender_id  BIGINT NOT NULL REFERENCES users (id),
    body       TEXT NOT NULL CHECK (char_length(body) BETWEEN 1 AND 1000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX messages_auction_id_idx ON messages (auction_id, id);
