# ADR-0008 — Cancellation is allowed from every non-terminal state

**Status:** Accepted · 2026-09-02

## Context
The first draft of the order machine allowed cancellation only from `placed` and `accepted`,
on the theory that food already cooked has been paid for in ingredients and should not vanish
from the record. But parties leave, guests refuse dishes, and kitchens make the wrong thing.

## Decision
Both the order and the item machine accept `cancelled` from every non-terminal state,
including `ready`. A reason is required once cooking has started, and cancelling after that
point is a `manager`+ capability (TZ §11.3).

## Consequences
- The day report stays honest. A machine that forbids late cancellation pushes staff into
  marking food *served* that never was, which corrupts revenue and cook-time data far worse
  than an honest late cancellation does.
- Waste tracking gets its data from cancellations with reasons — a Release 2 report, free.
- `deriveOrderStatus` must treat "every line cancelled" as order-cancelled, and must never
  pull an order back off `ready` when a line is reopened.
