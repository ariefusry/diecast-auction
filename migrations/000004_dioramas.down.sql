-- Dioramas become plain items named after their artist so the old rules hold again.
UPDATE items SET brand = coalesce(artist, 'Diorama'), condition = 'loose_mint' WHERE category = 'diorama';
ALTER TABLE items
    DROP CONSTRAINT items_category_fields,
    DROP COLUMN ready_weeks,
    DROP COLUMN size,
    DROP COLUMN artist,
    DROP COLUMN category,
    ALTER COLUMN brand SET NOT NULL,
    ADD CONSTRAINT items_condition_check CHECK (condition IN
        ('mint_on_card', 'sealed_box', 'acrylic_case', 'loose_mint', 'damaged_packaging'));
