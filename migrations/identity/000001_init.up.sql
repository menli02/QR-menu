-- Baseline migration. Enables the extension domain-model migrations will
-- rely on for UUID primary keys (gen_random_uuid()). Real schema (per
-- docs/TZ.md §6) lands in numbered migrations that follow this one.
CREATE EXTENSION IF NOT EXISTS pgcrypto;
