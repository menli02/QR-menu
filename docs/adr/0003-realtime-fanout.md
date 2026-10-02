# ADR-0003 — Kafka for events, per-pod consumer groups for WebSocket fan-out

**Status:** Accepted · 2026-08-31

## Context
A ticket must appear on the KDS within ~2 s of submit. Domain events must also be durable for
audit, analytics and future consumers. Every gateway pod holds a different subset of sockets,
so every pod needs every event — the opposite of Kafka's usual partitioned-consumption model,
where one group member gets each message.

## Decision
Kafka carries durable domain events. The gateway consumes with a **per-instance consumer
group** (`gateway-<pod-name>`) so each pod receives every event and pushes to the sockets it
owns. These consumers are read-only and idempotent, which is what makes per-pod groups safe.
Redis Pub/Sub is the documented fallback.

## Consequences
- Consumer-group count grows with pod count; groups belonging to dead pods must be reaped by a
  cleanup job. An accepted operational cost.
- Realtime is an optimisation, never a source of truth: clients re-fetch over REST on
  reconnect (TZ §7.3), so a missed push degrades latency, not correctness.
- Switching to Redis Pub/Sub would touch only the gateway.
