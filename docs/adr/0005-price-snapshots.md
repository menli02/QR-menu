# ADR-0005 — Server-resolved prices with snapshots on order items

**Status:** Accepted · 2026-08-31

## Context
The guest client is untrusted. Menu prices also change while a guest is browsing, and orders
must render correctly years later even if the item was renamed, re-priced or deleted.

## Decision
The client sends only item ids, modifier option ids, quantity and comment. `order` resolves
current price and availability through `catalog.ResolveOrderItems` and stores a snapshot
(`name`, `unit_price_minor`, modifier names and deltas, `line_total_minor`) on the order rows.
A price that differs from what the client displayed returns `409 PRICE_CHANGED` for explicit
re-confirmation. `menu_item_id` and `option_id` are retained for analytics only and are never
re-resolved.

## Consequences
- A forged price is structurally impossible, not merely validated against.
- Order history is immutable and independent of later menu edits.
- Order submit takes a synchronous dependency on `catalog` (ADR-0001 consequences).
- `menu_version` is stored as `TEXT` in `order_db` so order history never depends on
  catalog's internal representation of it.
