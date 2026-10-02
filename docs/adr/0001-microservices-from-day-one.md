# ADR-0001 — Microservices from day one

**Status:** Accepted · 2026-08-31

## Context
Release 1 is small enough that a modular monolith would ship faster. But the product runs
multi-tenant in production, the organization standard is microservices with gRPC, and the four
candidate components have genuinely different profiles: the gateway scales with open
WebSocket connections, `catalog` is read-dominated and cacheable, `order` is the transactional
core, and `identity` is the security-sensitive piece we want to keep small and auditable.

## Decision
Four services: `gateway` (go-zero api), `catalog`, `order`, `identity` (go-zero rpc).
Database per service. No cross-service foreign keys or joins; ids from other services are
stored as plain UUIDs.

## Consequences
- Costs paid up front: proto contracts, codegen, four deployments, a heavier local setup,
  eventual consistency between catalog and order.
- Mitigated by holding the count at four. Realtime fan-out, reporting and media processing
  deliberately stay inside existing services until an explicit trigger (TZ §15).
- `order` depends synchronously on `catalog` for pricing. The breaker and the degradation
  matrix (TZ §11.2) define what happens when that dependency is down.
