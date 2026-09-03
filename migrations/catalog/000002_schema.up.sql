-- catalog_db schema — Venue, Hall, Table, Category, MenuItem, ModifierGroup,
-- ModifierOption (docs/TZ.md §6, §7.1). Column choices are traced to
-- proto/catalog/v1/catalog.proto and docs/TZ.md §5.1/§5.2 (FR-C*, FR-T*)
-- inline below. No cross-service foreign keys (docs/TZ.md §6): table_id,
-- staff_id etc. from other services are stored as plain UUIDs.

-- Applied once, at CREATE-time, to every table with an updated_at column.
CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ---------------------------------------------------------------------
-- Venue and its QR signing keys
-- ---------------------------------------------------------------------

-- No CreateVenue RPC exists in R1 (docs/TZ.md: venue provisioning is a
-- platform-operator/runbook action, out of product scope — §4.1). `slug`
-- is not in the VenueSettings proto message either, but ResolveTableRequest
-- takes venue_slug (catalog.proto) to resolve a QR link to a venue, so it
-- has to live somewhere stable — assumed to be assigned at provisioning
-- time and immutable thereafter.
CREATE TABLE venues (
    id                          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug                        TEXT NOT NULL,
    name                        TEXT NOT NULL,
    logo_url                    TEXT,
    currency                    CHAR(3) NOT NULL,               -- ISO-4217 (A1: single currency per venue)
    locales                     TEXT[] NOT NULL DEFAULT '{}',
    default_locale              TEXT NOT NULL DEFAULT 'en',
    timezone                    TEXT NOT NULL DEFAULT 'UTC',    -- IANA name; drives business-day cutoff (FR-A2)
    service_charge_bps          INT NOT NULL DEFAULT 0 CHECK (service_charge_bps BETWEEN 0 AND 10000),
    order_item_comment_max_len  INT NOT NULL DEFAULT 200 CHECK (order_item_comment_max_len > 0), -- FR-O5 ceiling is 200; see migration note in order_db
    order_total_limit_minor     BIGINT CHECK (order_total_limit_minor IS NULL OR order_total_limit_minor > 0), -- NULL = unlimited
    cancel_window_seconds       INT NOT NULL DEFAULT 60 CHECK (cancel_window_seconds >= 0),      -- FR-O11 default
    kds_amber_threshold_seconds INT NOT NULL DEFAULT 480 CHECK (kds_amber_threshold_seconds > 0), -- FR-K4 default (8 min)
    kds_red_threshold_seconds   INT NOT NULL DEFAULT 900 CHECK (kds_red_threshold_seconds > kds_amber_threshold_seconds), -- FR-K4 default (15 min)
    -- Cache-busting version for the guest menu (FR-C6). Bumped by catalog's
    -- write-path logic in the same transaction as any category/item/
    -- modifier change — not a DB trigger, to keep the "what changed"
    -- decision (e.g. skip on a no-op update) in application code.
    menu_version                BIGINT NOT NULL DEFAULT 1,
    created_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at                  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (slug)
);

CREATE TRIGGER venues_set_updated_at
    BEFORE UPDATE ON venues
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- HMAC keys used to sign/verify table QR codes (FR-T2, FR-T4). The secret
-- is application-level (used only server-side to verify a signature, never
-- handed to a client), which is why it can live in this table rather than
-- a K8s Secret — contrast with identity_db's JWT signing keys, whose
-- private key material is deliberately kept OUT of Postgres (see that
-- migration's header comment). Still sensitive: never log `secret`.
CREATE TABLE venue_qr_keys (
    venue_id    UUID NOT NULL REFERENCES venues(id) ON DELETE CASCADE,
    key_version INT NOT NULL,
    secret      BYTEA NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- NULL while this is the current signing key. Set to created_at-of-the-
    -- next-key + the venue's configured grace period when rotated out
    -- (FR-T4, default 30 days) — application logic's job, not a default
    -- here, since the grace period is a RotateVenueQRKey request param.
    expires_at  TIMESTAMPTZ,
    PRIMARY KEY (venue_id, key_version)
);

-- Fast "give me the current key for this venue" lookup (ResolveTable is on
-- the guest hot path).
CREATE UNIQUE INDEX venue_qr_keys_current_idx ON venue_qr_keys (venue_id) WHERE expires_at IS NULL;

-- ---------------------------------------------------------------------
-- Halls and tables (FR-T1..T5)
-- ---------------------------------------------------------------------

CREATE TABLE halls (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    venue_id   UUID NOT NULL REFERENCES venues(id) ON DELETE CASCADE,
    name       TEXT NOT NULL,
    sort_order INT NOT NULL DEFAULT 0,
    is_active  BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX halls_venue_idx ON halls (venue_id, sort_order);

CREATE TRIGGER halls_set_updated_at
    BEFORE UPDATE ON halls
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE tables (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    venue_id     UUID NOT NULL REFERENCES venues(id) ON DELETE CASCADE,
    hall_id      UUID NOT NULL REFERENCES halls(id),
    label        TEXT NOT NULL,
    seats        INT NOT NULL DEFAULT 1 CHECK (seats > 0),
    is_active    BOOLEAN NOT NULL DEFAULT true,
    -- Immutable, random, URL-safe (FR-T2). Uniqueness is scoped to the
    -- venue, not global: ResolveTable is always called with a venue_slug
    -- alongside table_code.
    table_code   TEXT NOT NULL CHECK (char_length(table_code) >= 10),
    key_version  INT NOT NULL DEFAULT 1,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (venue_id, table_code),
    -- A table's printed QR always points at a key_version that actually
    -- exists for that venue (see venue_qr_keys above).
    FOREIGN KEY (venue_id, key_version) REFERENCES venue_qr_keys (venue_id, key_version)
);

CREATE INDEX tables_venue_hall_idx ON tables (venue_id, hall_id);

CREATE TRIGGER tables_set_updated_at
    BEFORE UPDATE ON tables
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------
-- Categories and menu items (FR-C1..C8)
-- ---------------------------------------------------------------------

-- `name`/`description` mirror proto LocalizedText: {"en": "...", "ru": "..."}.
CREATE TABLE categories (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    venue_id   UUID NOT NULL REFERENCES venues(id) ON DELETE CASCADE,
    name       JSONB NOT NULL DEFAULT '{}',
    sort_order INT NOT NULL DEFAULT 0,
    is_visible BOOLEAN NOT NULL DEFAULT true,
    image_url  TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX categories_venue_idx ON categories (venue_id, sort_order);

CREATE TRIGGER categories_set_updated_at
    BEFORE UPDATE ON categories
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- No `currency` column here: price is always in the owning venue's
-- currency (A1). catalog logic fills Money.currency from venues.currency
-- when building a response; storing it per-item would let it silently
-- diverge from the venue's currency.
CREATE TABLE menu_items (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    venue_id         UUID NOT NULL REFERENCES venues(id) ON DELETE CASCADE,
    category_id      UUID NOT NULL REFERENCES categories(id),
    name             JSONB NOT NULL DEFAULT '{}',
    description      JSONB NOT NULL DEFAULT '{}',
    base_price_minor BIGINT NOT NULL CHECK (base_price_minor >= 0),
    image_url        TEXT,
    allergens        TEXT[] NOT NULL DEFAULT '{}',
    is_active        BOOLEAN NOT NULL DEFAULT true,   -- published/soft-delete flag
    is_available     BOOLEAN NOT NULL DEFAULT true,   -- stop-list state (FR-C4)
    sort_order       INT NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Guest menu read (FR-C5) filters on exactly these columns for one venue.
CREATE INDEX menu_items_guest_menu_idx ON menu_items (venue_id, is_active, is_available);
CREATE INDEX menu_items_category_idx ON menu_items (venue_id, category_id, sort_order);

CREATE TRIGGER menu_items_set_updated_at
    BEFORE UPDATE ON menu_items
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- FR-C7 (S-priority) availability windows. No `id` in the proto message —
-- it is never addressed individually over the API, only replaced wholesale
-- with the parent item — but the table still needs its own PK.
CREATE TABLE availability_windows (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id             UUID NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
    start_minute_of_day INT NOT NULL CHECK (start_minute_of_day BETWEEN 0 AND 1439),
    end_minute_of_day   INT NOT NULL CHECK (end_minute_of_day BETWEEN 0 AND 1439)
);

CREATE INDEX availability_windows_item_idx ON availability_windows (item_id);

-- ---------------------------------------------------------------------
-- Modifier groups and options (FR-C3)
-- ---------------------------------------------------------------------

-- NOTE: docs/TZ.md §6's ER diagram models MENU_ITEM }o--o{ MODIFIER_GROUP
-- (many-to-many — a group reusable across items), but every RPC in
-- catalog.proto (CreateModifierGroup, UpdateModifierGroup, ...) addresses
-- a group through a single owning item_id, with no group-reuse endpoint.
-- Implemented per the proto (the binding contract): one item owns N
-- groups, no cross-item sharing. Revisit as a join table if Release 2
-- wants reusable groups (e.g. one "Milk" group shared by every coffee).
CREATE TABLE modifier_groups (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    item_id     UUID NOT NULL REFERENCES menu_items(id) ON DELETE CASCADE,
    name        JSONB NOT NULL DEFAULT '{}',
    min_select  INT NOT NULL DEFAULT 0 CHECK (min_select >= 0),
    max_select  INT NOT NULL CHECK (max_select >= min_select),
    required    BOOLEAN NOT NULL DEFAULT false,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX modifier_groups_item_idx ON modifier_groups (item_id);

CREATE TRIGGER modifier_groups_set_updated_at
    BEFORE UPDATE ON modifier_groups
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE modifier_options (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id         UUID NOT NULL REFERENCES modifier_groups(id) ON DELETE CASCADE,
    name             JSONB NOT NULL DEFAULT '{}',
    price_delta_minor BIGINT NOT NULL DEFAULT 0, -- may be negative (e.g. a discount option)
    sort_order       INT NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX modifier_options_group_idx ON modifier_options (group_id, sort_order);

CREATE TRIGGER modifier_options_set_updated_at
    BEFORE UPDATE ON modifier_options
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------
-- Transactional outbox (docs/TZ.md §8.3) — produces qrmenu.catalog.v1
-- ---------------------------------------------------------------------

-- The domain write and this insert happen in one transaction; a relay
-- goroutine (not yet implemented) polls unsent rows, publishes to Kafka,
-- and stamps sent_at. Column names mirror the §8.3 JSON envelope 1:1 so
-- the relay can serialize a row with no field mapping.
CREATE TABLE outbox (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(), -- = envelope event_id
    event_type     TEXT NOT NULL,                              -- e.g. "catalog.item_availability_changed"
    schema_version INT NOT NULL DEFAULT 1,
    venue_id       UUID NOT NULL,
    topic          TEXT NOT NULL,                              -- "qrmenu.catalog.v1"
    partition_key  TEXT NOT NULL,                               -- item_id, per §8.3's Key column
    payload        JSONB NOT NULL,
    trace_id       TEXT,
    occurred_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    sent_at        TIMESTAMPTZ,                                 -- NULL = still pending relay pickup
    attempts       INT NOT NULL DEFAULT 0,
    last_error     TEXT
);

-- The relay's only query: unsent rows, oldest first.
CREATE INDEX outbox_unsent_idx ON outbox (occurred_at) WHERE sent_at IS NULL;
