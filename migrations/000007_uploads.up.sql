-- One row per stored photo, so uploads can be capped per user and in total.
CREATE TABLE uploads (
    name       TEXT PRIMARY KEY,
    user_id    BIGINT NOT NULL REFERENCES users (id),
    size       BIGINT NOT NULL CHECK (size > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX uploads_created_at_idx ON uploads (created_at);
