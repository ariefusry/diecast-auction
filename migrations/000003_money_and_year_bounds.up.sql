-- Upper bounds back up service.MaxMoney so bid + increment can never overflow
-- BIGINT, and keep item years sane. Rp1.000.000.000.000 = service.MaxMoney.
ALTER TABLE auctions
    ADD CONSTRAINT auctions_start_price_max CHECK (start_price <= 1000000000000),
    ADD CONSTRAINT auctions_min_increment_max CHECK (min_increment <= 1000000000000),
    ADD CONSTRAINT auctions_buy_now_price_max CHECK (buy_now_price <= 1000000000000),
    ADD CONSTRAINT auctions_current_highest_bid_max CHECK (current_highest_bid <= 1000000000000);
ALTER TABLE items ADD CONSTRAINT items_year_range CHECK (year BETWEEN 1950 AND 2100);
