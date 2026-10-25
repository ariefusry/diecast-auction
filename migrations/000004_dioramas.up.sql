-- Dioramas have no diecast brand. They are sold by the artist who built them
-- (optionally as a PO, ready_weeks) or second-hand by an owner, so the artist
-- is optional and condition is simply new or used. The packaging conditions
-- (mint_on_card, ...) only make sense for diecast.
ALTER TABLE items
    ADD COLUMN category TEXT NOT NULL DEFAULT 'diecast' CHECK (category IN ('diecast', 'diorama')),
    ADD COLUMN artist TEXT,
    ADD COLUMN size TEXT,
    ADD COLUMN ready_weeks INT CHECK (ready_weeks BETWEEN 1 AND 52),
    ALTER COLUMN brand DROP NOT NULL,
    DROP CONSTRAINT items_condition_check;
ALTER TABLE items ADD CONSTRAINT items_category_fields CHECK (
    CASE category
        WHEN 'diecast' THEN brand IS NOT NULL
            AND condition IN ('mint_on_card', 'sealed_box', 'acrylic_case', 'loose_mint', 'damaged_packaging')
            AND artist IS NULL AND size IS NULL AND ready_weeks IS NULL
        ELSE brand IS NULL AND condition IN ('new', 'used')
    END);
