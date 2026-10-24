-- Money is BIGINT whole rupiah everywhere. CHECK constraints are the last line
-- of defense: a service bug must never be able to persist invalid balances.

CREATE TABLE users (
    id            BIGSERIAL PRIMARY KEY,
    email         TEXT NOT NULL,
    username      TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_lower_key ON users (lower(email));

-- One wallet per user, keyed by user_id so the lock order rule
-- (wallets by ascending user_id) maps directly onto the primary key.
CREATE TABLE wallets (
    user_id           BIGINT PRIMARY KEY REFERENCES users (id),
    available_balance BIGINT NOT NULL DEFAULT 0 CHECK (available_balance >= 0),
    locked_balance    BIGINT NOT NULL DEFAULT 0 CHECK (locked_balance >= 0)
);

CREATE TABLE items (
    id          BIGSERIAL PRIMARY KEY,
    seller_id   BIGINT NOT NULL REFERENCES users (id),
    brand       TEXT NOT NULL,
    model_name  TEXT NOT NULL,
    series      TEXT,
    year        INT,
    color       TEXT,
    tags        TEXT[] NOT NULL DEFAULT '{}',
    condition   TEXT NOT NULL CHECK (condition IN
                  ('mint_on_card', 'sealed_box', 'acrylic_case', 'loose_mint', 'damaged_packaging')),
    image_url   TEXT,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX items_seller_id_idx ON items (seller_id);

CREATE TABLE auctions (
    id                  BIGSERIAL PRIMARY KEY,
    item_id             BIGINT NOT NULL REFERENCES items (id),
    start_price         BIGINT NOT NULL CHECK (start_price > 0),
    min_increment       BIGINT NOT NULL CHECK (min_increment > 0),
    buy_now_price       BIGINT CHECK (buy_now_price > start_price),
    current_highest_bid BIGINT,
    highest_bidder_id   BIGINT REFERENCES users (id),
    status              TEXT NOT NULL DEFAULT 'draft'
                          CHECK (status IN ('draft', 'active', 'completed', 'cancelled')),
    duration            INTERVAL NOT NULL CHECK (duration > interval '0'),
    start_time          TIMESTAMPTZ,
    end_time            TIMESTAMPTZ,
    winner_id           BIGINT REFERENCES users (id),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- A highest bid and its bidder always come as a pair. highest_bidder_id IS NULL
    -- is the single "no bids yet" signal used by cancel and buy-now.
    CHECK ((current_highest_bid IS NULL) = (highest_bidder_id IS NULL))
);
CREATE INDEX auctions_status_end_time_idx ON auctions (status, end_time);

CREATE TABLE bids (
    id         BIGSERIAL PRIMARY KEY,
    auction_id BIGINT NOT NULL REFERENCES auctions (id),
    bidder_id  BIGINT NOT NULL REFERENCES users (id),
    amount     BIGINT NOT NULL CHECK (amount > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX bids_auction_id_amount_idx ON bids (auction_id, amount DESC);

-- Append-only ledger: amount is always positive, type gives the direction.
CREATE TABLE wallet_transactions (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES wallets (user_id),
    type       TEXT NOT NULL CHECK (type IN ('deposit', 'lock', 'release', 'settle', 'buy_now')),
    amount     BIGINT NOT NULL CHECK (amount > 0),
    auction_id BIGINT REFERENCES auctions (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX wallet_transactions_user_id_idx ON wallet_transactions (user_id);

CREATE FUNCTION reject_ledger_change() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'wallet_transactions is append-only';
END;
$$;
CREATE TRIGGER wallet_transactions_append_only
    BEFORE UPDATE OR DELETE ON wallet_transactions
    FOR EACH ROW EXECUTE FUNCTION reject_ledger_change();
