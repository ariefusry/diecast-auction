ALTER TABLE auctions
    DROP CONSTRAINT auctions_start_price_max,
    DROP CONSTRAINT auctions_min_increment_max,
    DROP CONSTRAINT auctions_buy_now_price_max,
    DROP CONSTRAINT auctions_current_highest_bid_max;
ALTER TABLE items DROP CONSTRAINT items_year_range;
