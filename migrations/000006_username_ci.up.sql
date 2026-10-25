-- Usernames are public storefront names: Budi_64 and budi_64 must not both exist.
CREATE UNIQUE INDEX users_username_lower_key ON users (lower(username));
