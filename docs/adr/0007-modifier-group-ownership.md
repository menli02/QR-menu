# ADR-0007 — A modifier group belongs to exactly one menu item

**Status:** Accepted · 2026-09-01

## Context
TZ §6's ER diagram models `MENU_ITEM }o--o{ MODIFIER_GROUP` — a many-to-many relation where
one "Milk" group could be reused by every coffee. But every RPC in `catalog.proto`
(`CreateModifierGroup`, `UpdateModifierGroup`, …) addresses a group through a single owning
`item_id`, and there is no group-reuse endpoint. The diagram and the contract disagreed, and
the schema had to pick one.

## Decision
Follow the proto, which is the binding contract: `modifier_groups.item_id` references
`menu_items(id) ON DELETE CASCADE`. One item owns N groups; no cross-item sharing.

## Consequences
- A venue with the same options on twenty drinks maintains twenty copies. Real cost, accepted
  for R1 because the alternative is an API surface nobody has asked for yet.
- Becomes a join table in Release 2 if reuse is requested — an additive migration plus new
  RPCs, with existing rows mapping one-to-one into it.
- TZ §6's diagram is superseded on this point by §9.1.
