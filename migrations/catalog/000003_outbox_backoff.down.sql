DROP INDEX IF EXISTS outbox_pending_idx;
CREATE INDEX outbox_unsent_idx ON outbox (occurred_at) WHERE sent_at IS NULL;

ALTER TABLE outbox DROP COLUMN IF EXISTS next_attempt_at;
