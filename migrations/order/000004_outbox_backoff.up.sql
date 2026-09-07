-- Retry scheduling for the outbox relay (docs/TZ.md §8.3: "5 attempts
-- with exponential backoff (1s, 2s, 4s, 8s, 16s), then the message goes
-- to the topic's DLQ").
--
-- 000002 gave the outbox an `attempts` counter but nothing to say *when*
-- the next attempt is due, so a broker outage would have the relay spin
-- on the same rows as fast as it could poll. next_attempt_at turns the
-- counter into an actual schedule: the relay pushes this forward on each
-- failure and skips rows that aren't due yet.
--
-- DEFAULT now() means every existing row is immediately eligible, which
-- is the correct reading of a row written before this column existed.
ALTER TABLE outbox
    ADD COLUMN next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- Replaces outbox_unsent_idx: the relay's claim query now filters on both
-- columns, and an index that doesn't cover next_attempt_at would have it
-- reading (and discarding) every backed-off row on every poll.
DROP INDEX IF EXISTS outbox_unsent_idx;
CREATE INDEX outbox_pending_idx ON outbox (next_attempt_at, occurred_at) WHERE sent_at IS NULL;
