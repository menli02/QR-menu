# ADR-0004 — Transactional outbox for event publication

**Status:** Accepted · 2026-08-31

## Context
Writing to Postgres and publishing to Kafka is not one atomic operation. Publishing before
commit can announce an order that never existed; publishing after commit can lose an order the
kitchen never sees. Neither failure is acceptable on the ordering path.

## Decision
The domain write and the `outbox` insert commit in the same transaction. A relay
(`pkg/outbox`) polls unsent rows, publishes, and stamps `sent_at`; failures increment
`attempts`, record `last_error` and push `next_attempt_at` out with exponential backoff.
Consumers are idempotent (dedupe by `event_id`), because a crashed relay can publish a row
twice. The outbox table has the same shape in `catalog_db` and `order_db` so one relay
implementation serves both.

## Consequences
- At-least-once delivery, never zero. The failure mode is a duplicate, which consumers absorb.
- Needs monitoring: `outbox_pending` is alerted on, so a stalled relay is visible in minutes.
- `next_attempt_at` keeps a broken broker from turning the relay into a hot loop against
  Postgres — the failure mode that would otherwise take the database down with Kafka.
- Slight write amplification on the hot order path; acceptable at pilot scale.
