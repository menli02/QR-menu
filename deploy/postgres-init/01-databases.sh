#!/usr/bin/env bash
# Creates one database per service (docs/TZ.md §6, §7.1: no cross-service
# foreign keys, no shared schema) and enables the extensions migrations
# rely on for UUID primary keys.
set -euo pipefail

for db in catalog_db order_db identity_db; do
  psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-SQL
	    SELECT 'CREATE DATABASE $db OWNER $POSTGRES_USER'
	    WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '$db')\gexec
	SQL

  psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$db" <<-SQL
	    CREATE EXTENSION IF NOT EXISTS pgcrypto;
	SQL
done
