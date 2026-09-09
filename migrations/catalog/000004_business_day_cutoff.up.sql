-- Business-day cutoff (docs/TZ.md FR-A2).
--
-- FR-A2 lists a "business-day cutoff" among venue settings, but 000002
-- carried only the timezone, and its comment read "IANA name; drives
-- business-day cutoff". That was the whole cutoff: midnight venue-local.
-- For a venue that serves past midnight it puts the night's late orders on
-- the next business day, splitting one service across two day reports and
-- restarting the FR-O8 ticket numbers mid-shift.
--
-- Minutes rather than an hour, because some venues really do close at
-- 03:30. The CHECK bounds it to a day; the order service treats anything
-- before the cutoff as belonging to the previous business day.
--
-- DEFAULT 0 is exactly the behaviour every existing row already had, so
-- this deploys ahead of the code that reads it and changes nothing until a
-- venue sets it.
ALTER TABLE venues
    ADD COLUMN business_day_cutoff_minute INT NOT NULL DEFAULT 0
        CHECK (business_day_cutoff_minute >= 0 AND business_day_cutoff_minute < 1440);

COMMENT ON COLUMN venues.business_day_cutoff_minute IS
    'Minutes past venue-local midnight at which the business day rolls over (FR-A2). 0 = midnight, 240 = 04:00.';
