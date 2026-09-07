DROP INDEX IF EXISTS orders_venue_business_date_idx;

ALTER TABLE orders
    DROP COLUMN IF EXISTS served_at,
    DROP COLUMN IF EXISTS ready_at,
    DROP COLUMN IF EXISTS accepted_at;
