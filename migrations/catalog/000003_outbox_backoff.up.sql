-- Retry scheduling for the outbox relay. Identical to
-- migrations/order/000004_outbox_backoff.up.sql — the outbox table has
-- the same shape in every producing service, because pkg/outbox's relay
-- runs against all of them with one set of queries.
--
-- See that file for the full rationale.
ALTER TABLE outbox
    ADD COLUMN next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now();

DROP INDEX IF EXISTS outbox_unsent_idx;
CREATE INDEX outbox_pending_idx ON outbox (next_attempt_at, occurred_at) WHERE sent_at IS NULL;
