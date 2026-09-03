-- order_db schema — TableSession, GuestSession, Order, OrderItem,
-- ServiceRequest, Bill (docs/TZ.md §6, §7.1). Traced to
-- proto/order/v1/order.proto and docs/TZ.md §5.3/§5.4/§5.5 (FR-O*, FR-K*,
-- FR-S*) inline below. No cross-service foreign keys (docs/TZ.md §6):
-- venue_id, table_id, menu_item_id, staff_id are plain UUIDs — catalog and
-- identity own their own referential integrity.

CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ---------------------------------------------------------------------
-- Table sessions and guest sessions
-- ---------------------------------------------------------------------

-- No CreateTableSession RPC exists: FR-O9 opens one lazily, inside the
-- same transaction as the first CreateOrder (or CreateServiceRequest —
-- CreateServiceRequestRequest also takes a table_session_id, so a guest
-- calling the waiter before ordering must go through the same
-- open-or-join path). Only one *open* session per table at a time.
CREATE TABLE table_sessions (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    venue_id           UUID NOT NULL,
    table_id           UUID NOT NULL,
    status             TEXT NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed')),
    opened_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    closed_at          TIMESTAMPTZ,
    closed_by_staff_id UUID,
    payment_method     TEXT CHECK (payment_method IN ('cash', 'card_terminal', 'other')),
    currency           CHAR(3) NOT NULL, -- snapshot of the venue's currency at open time (A1)
    -- Denormalized mirror of GetBill's total, kept in sync by order-write
    -- logic in the same transaction as any order/order_item change. GetBill
    -- itself is computed fresh from `orders` (source of truth) — this
    -- column exists only so ListActiveTableSessions (the floor view) can
    -- render totals without a join per row.
    total_minor        BIGINT NOT NULL DEFAULT 0 CHECK (total_minor >= 0),
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX table_sessions_one_open_per_table_idx ON table_sessions (table_id) WHERE status = 'open';
CREATE INDEX table_sessions_venue_status_idx ON table_sessions (venue_id, status);

CREATE TRIGGER table_sessions_set_updated_at
    BEFORE UPDATE ON table_sessions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- `id` has NO default: it must equal the guest_session_id already minted
-- into the guest's JWT by identity.IssueGuestToken *before* this row is
-- created (gateway generates the id, calls identity, then the row is
-- persisted lazily on the guest's first order/service request — same
-- open-or-join transaction as table_sessions above). A DEFAULT here would
-- risk silently diverging from the token's claim if the caller forgets to
-- pass one explicitly.
CREATE TABLE guest_sessions (
    id               UUID PRIMARY KEY,
    venue_id         UUID NOT NULL,
    table_id         UUID NOT NULL,
    table_session_id UUID NOT NULL REFERENCES table_sessions(id),
    issued_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at       TIMESTAMPTZ NOT NULL,      -- FR-O1: 4h TTL, sliding — bookkeeping copy; the JWT `exp` claim is the actual stateless check
    last_seen_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at       TIMESTAMPTZ
);

CREATE INDEX guest_sessions_table_session_idx ON guest_sessions (table_session_id);

-- ---------------------------------------------------------------------
-- Per-venue, per-business-day human order numbers (FR-O8, e.g. "A-014")
-- ---------------------------------------------------------------------

-- Usage: INSERT INTO order_number_counters (venue_id, business_date, next_seq)
-- VALUES ($1, $2, 1) ON CONFLICT (venue_id, business_date)
-- DO UPDATE SET next_seq = order_number_counters.next_seq + 1
-- RETURNING next_seq;
-- Atomic under concurrent access — no SELECT ... FOR UPDATE needed.
-- business_date is the venue-local business day (FR-A2 cutoff), computed
-- by application logic, not derived here.
CREATE TABLE order_number_counters (
    venue_id      UUID NOT NULL,
    business_date DATE NOT NULL,
    next_seq      INT NOT NULL DEFAULT 1,
    PRIMARY KEY (venue_id, business_date)
);

-- ---------------------------------------------------------------------
-- Orders and order items
-- ---------------------------------------------------------------------

CREATE TABLE orders (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    venue_id          UUID NOT NULL,
    table_id          UUID NOT NULL,
    table_session_id  UUID NOT NULL REFERENCES table_sessions(id),
    guest_session_id  UUID NOT NULL REFERENCES guest_sessions(id),
    business_date     DATE NOT NULL,   -- see order_number_counters above
    number            TEXT NOT NULL,   -- e.g. "A-014", unique per (venue_id, business_date)
    status            TEXT NOT NULL DEFAULT 'placed'
                          CHECK (status IN ('placed', 'accepted', 'in_progress', 'ready', 'served', 'cancelled')),
    -- Denormalized snapshots (docs/TZ.md §6: "so a menu edit never rewrites
    -- history"). total_minor/currency/menu_version are fixed at placement.
    total_minor       BIGINT NOT NULL CHECK (total_minor >= 0),
    currency          CHAR(3) NOT NULL,
    menu_version      TEXT NOT NULL,   -- verbatim copy of whatever catalog.ResolveOrderItems returned; TEXT (not catalog_db's internal BIGINT) so this table never depends on catalog's representation
    cancelled_reason  TEXT,
    placed_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (venue_id, business_date, number)
    -- FR-O5's hard caps ("items per order <= 50", "total <= venue limit")
    -- are cross-row/cross-service checks (count of order_items; a limit
    -- that lives in catalog_db's venues table) — enforced in order-service
    -- validation logic at submit time, not as a DB constraint here.
);

-- FR-K1: ticket list per venue, sorted oldest-first, typically filtered to
-- the active subset (placed..ready).
CREATE INDEX orders_venue_status_placed_idx ON orders (venue_id, status, placed_at);
CREATE INDEX orders_table_session_idx ON orders (table_session_id);

CREATE TRIGGER orders_set_updated_at
    BEFORE UPDATE ON orders
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE order_items (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id          UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    menu_item_id      UUID NOT NULL,             -- catalog-owned, no FK; snapshot below is authoritative once placed
    name              TEXT NOT NULL,             -- snapshot
    unit_price_minor  BIGINT NOT NULL CHECK (unit_price_minor >= 0), -- snapshot
    qty               INT NOT NULL CHECK (qty BETWEEN 1 AND 99),     -- FR-O5
    comment           TEXT CHECK (comment IS NULL OR char_length(comment) <= 200), -- FR-O5 hard ceiling; a stricter per-venue setting (VenueSettings.order_item_comment_max_len) is enforced in application logic
    status            TEXT NOT NULL DEFAULT 'placed'
                          CHECK (status IN ('placed', 'cooking', 'ready', 'cancelled')),
    line_total_minor  BIGINT NOT NULL CHECK (line_total_minor >= 0),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX order_items_order_idx ON order_items (order_id);

CREATE TRIGGER order_items_set_updated_at
    BEFORE UPDATE ON order_items
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE order_item_modifiers (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_item_id     UUID NOT NULL REFERENCES order_items(id) ON DELETE CASCADE,
    option_id         UUID NOT NULL,   -- catalog-owned, no FK; snapshot below is authoritative
    name              TEXT NOT NULL,   -- snapshot
    price_delta_minor BIGINT NOT NULL DEFAULT 0 -- snapshot; may be negative
);

CREATE INDEX order_item_modifiers_order_item_idx ON order_item_modifiers (order_item_id);

-- ---------------------------------------------------------------------
-- Service requests (FR-S1..S7)
-- ---------------------------------------------------------------------

-- FR-S2: only one OPEN request per (table, type) — a repeat within the
-- window returns the existing one instead of creating a new row.
CREATE TABLE service_requests (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    venue_id         UUID NOT NULL,
    table_id         UUID NOT NULL,
    table_session_id UUID NOT NULL REFERENCES table_sessions(id),
    type             TEXT NOT NULL CHECK (type IN ('call_waiter', 'request_bill')),
    status           TEXT NOT NULL DEFAULT 'open'
                         CHECK (status IN ('open', 'acknowledged', 'resolved', 'expired')),
    note             TEXT, -- e.g. preferred payment method hint (FR-S7)
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    acknowledged_at  TIMESTAMPTZ,
    resolved_at      TIMESTAMPTZ,
    expires_at       TIMESTAMPTZ NOT NULL -- FR-S4: created_at + 15 min default, computed by application logic (venue-configurable)
);

CREATE UNIQUE INDEX service_requests_one_open_per_table_type_idx ON service_requests (table_id, type) WHERE status = 'open';
CREATE INDEX service_requests_venue_status_idx ON service_requests (venue_id, status, created_at);

-- ---------------------------------------------------------------------
-- Idempotency (docs/TZ.md §7.4)
-- ---------------------------------------------------------------------

-- One row per (venue_id, endpoint, key). A request handler:
--   1. INSERTs with in_progress=true before doing any work (a concurrent
--      second request finds the row and takes `409 REQUEST_IN_PROGRESS`
--      via `SELECT ... FOR UPDATE NOWAIT` failing to acquire the lock);
--   2. on completion, UPDATEs status_code/response_body and flips
--      in_progress to false.
-- A daily job purges rows older than 24h (created_at index below).
CREATE TABLE idempotency_keys (
    venue_id      UUID NOT NULL,
    endpoint      TEXT NOT NULL,
    key           TEXT NOT NULL, -- client-supplied UUIDv4
    fingerprint   TEXT NOT NULL, -- SHA-256 hex of the canonical request body
    in_progress   BOOLEAN NOT NULL DEFAULT true,
    status_code   INT,
    response_body JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (venue_id, endpoint, key)
);

CREATE INDEX idempotency_keys_created_at_idx ON idempotency_keys (created_at);

-- ---------------------------------------------------------------------
-- Transactional outbox (docs/TZ.md §8.3) — produces qrmenu.order.v1 and
-- qrmenu.service_request.v1
-- ---------------------------------------------------------------------

CREATE TABLE outbox (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- = envelope event_id
    event_type     TEXT NOT NULL,                              -- e.g. "order.placed"; see docs/TZ.md §8.3 for the full R1 list
    schema_version INT NOT NULL DEFAULT 1,
    venue_id       UUID NOT NULL,
    topic          TEXT NOT NULL,                              -- "qrmenu.order.v1" | "qrmenu.service_request.v1"
    partition_key  TEXT NOT NULL,                              -- order_id or table_id, per §8.3's Key column
    -- Payload commonly carries actor_staff_id/reason/before-after status
    -- for transition events — the audit trail behind FR-A4 lives in these
    -- events (consumed downstream), not in extra columns on the tables
    -- above (docs/TZ.md §7.3: "Kafka carries durable domain events —
    -- audit, analytics").
    payload        JSONB NOT NULL,
    trace_id       TEXT,
    occurred_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at        TIMESTAMPTZ, -- NULL = still pending relay pickup
    attempts       INT NOT NULL DEFAULT 0,
    last_error     TEXT
);

CREATE INDEX outbox_unsent_idx ON outbox (occurred_at) WHERE sent_at IS NULL;
