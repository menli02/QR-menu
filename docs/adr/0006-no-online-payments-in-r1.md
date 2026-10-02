# ADR-0006 — No online payments in Release 1

**Status:** Accepted · 2026-08-31

## Context
Online payment adds a provider integration, webhook idempotency, refunds and partial refunds,
reconciliation, a PCI-relevant perimeter and legal/fiscal questions. None of it is needed to
test the core hypothesis: guests order from the table and the kitchen works from a screen.

## Decision
Release 1 settles payment off-system — cash or the venue's own card terminal. Staff mark the
bill paid with a method (`cash`, `card_terminal`, `other`), which closes the table session.
No amounts received, no change, no refunds.

## Consequences
- The bill stays a read model over orders. Adding payments later means a new `payment` service
  and a reference on the table session — additive, not a rewrite.
- The guest flow never touches card data, keeping Release 1 entirely out of PCI scope.
- "Request bill" is a service request to a human, not a checkout, and the UX must say so.
