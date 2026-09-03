-- identity_db schema — Staff, Role, refresh tokens, JWT signing-key
-- metadata (docs/TZ.md §6, §7.1). Traced to proto/identity/v1/identity.proto
-- and docs/TZ.md §5.6 (FR-A1) inline below. No cross-service foreign keys:
-- venue_id is a plain UUID, catalog owns its own referential integrity.

-- Case-insensitive, accent-preserving comparison for email lookups
-- (login is `(venue_id, email, password)` — see identity.proto LoginRequest).
-- Avoids the class of bug where "Staff@x.com" and "staff@x.com" are
-- silently treated as different accounts.
CREATE EXTENSION IF NOT EXISTS citext;

CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
  NEW.updated_at = now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ---------------------------------------------------------------------
-- Staff
-- ---------------------------------------------------------------------

-- Login is scoped by venue_id (LoginRequest carries all three), so
-- uniqueness is per-venue, not global — two different venues may
-- legitimately have staff sharing an email address.
CREATE TABLE staff (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    venue_id      UUID NOT NULL,
    name          TEXT NOT NULL,
    email         CITEXT NOT NULL,
    password_hash TEXT NOT NULL, -- bcrypt/argon2id; never the plaintext password (FR-A1 password policy is enforced in application logic, not here)
    role          TEXT NOT NULL CHECK (role IN ('admin', 'manager', 'waiter', 'cook')),
    is_active     BOOLEAN NOT NULL DEFAULT true,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (venue_id, email)
);

CREATE INDEX staff_venue_idx ON staff (venue_id);

CREATE TRIGGER staff_set_updated_at
    BEFORE UPDATE ON staff
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ---------------------------------------------------------------------
-- Refresh tokens
-- ---------------------------------------------------------------------

-- Only the SHA-256 hash of the refresh token is stored — a DB read (or
-- leak) never yields a usable credential. `replaced_by` chains rotations:
-- Refresh should issue a new token and mark the old one replaced, so a
-- replayed *old* token after rotation is detectable as reuse (a session
-- one step behind is normal in flight; a token whose replacement itself
-- has been superseded further, or is already revoked, is reuse/theft).
CREATE TABLE refresh_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    staff_id    UUID NOT NULL REFERENCES staff(id) ON DELETE CASCADE,
    token_hash  TEXT NOT NULL,
    issued_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ,
    replaced_by UUID REFERENCES refresh_tokens(id),
    UNIQUE (token_hash)
);

CREATE INDEX refresh_tokens_staff_idx ON refresh_tokens (staff_id);
CREATE INDEX refresh_tokens_expires_at_idx ON refresh_tokens (expires_at); -- cleanup job

-- ---------------------------------------------------------------------
-- JWT signing keys (staff and guest tokens; served via ListJWKS)
-- ---------------------------------------------------------------------

-- DELIBERATELY NO PRIVATE KEY COLUMN. identity is called out in
-- catalog.proto/docs/TZ.md §7.1 as "security-sensitive; smallest possible
-- blast radius" — a private signing key sitting in Postgres (even
-- encrypted) means a DB dump can forge any staff or guest token in the
-- system, staff and guest alike. Private key material for each `kid`
-- instead lives in a mounted Kubernetes Secret (or equivalent) that only
-- the identity pod can read; this table holds exactly what ListJWKS needs
-- to publish plus rotation bookkeeping. Wiring the Secret mount is part of
-- implementing identity's logic, not this migration.
CREATE TABLE signing_keys (
    kid         TEXT PRIMARY KEY,
    kty         TEXT NOT NULL CHECK (kty IN ('RSA', 'EC', 'oct')),
    alg         TEXT NOT NULL,           -- e.g. "RS256"
    public_jwk  JSONB NOT NULL,          -- public parts only: n/e (RSA) or crv/x/y (EC) — never private material
    -- Exactly one row should be active at a time (the key identity signs
    -- new tokens with); older rows stay present and are still returned by
    -- ListJWKS so tokens they already signed keep verifying until expiry.
    is_active   BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    retired_at  TIMESTAMPTZ
);

CREATE UNIQUE INDEX signing_keys_one_active_idx ON signing_keys (is_active) WHERE is_active;
