-- Transition timestamps on orders, needed by DayReport's
-- average_accept_seconds and average_cook_seconds
-- (proto/order/v1/order.proto: DayReport fields 8 and 9).
--
-- Why these are columns rather than derived: 000002 stored only placed_at
-- and updated_at, so the moment an order was accepted or became ready was
-- recoverable only from the outbox event stream. Reading Kafka to answer a
-- REST report would couple the reporting path to the relay's liveness and
-- retention window; three nullable timestamps on the row the transition
-- already updates cost nothing and keep GetDayReport a pure order_db read.
--
-- Additive and nullable, so this deploys ahead of the code that writes it
-- and leaves every existing row valid. Orders placed before this migration
-- simply have NULLs and are excluded from the averages (see report.go).
ALTER TABLE orders
    ADD COLUMN accepted_at TIMESTAMPTZ,
    ADD COLUMN ready_at    TIMESTAMPTZ,
    ADD COLUMN served_at   TIMESTAMPTZ;

-- GetDayReport scans one venue-day at a time; without this it is a
-- seq scan over the whole table once a venue has real history.
CREATE INDEX orders_venue_business_date_idx ON orders (venue_id, business_date);
