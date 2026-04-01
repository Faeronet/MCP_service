#!/bin/sh
# Run Postgres migrations against a host (локально или IP ВМ с db-zone).
# SQL лежат в зоне БД: db-zone/migrations/
# Usage: ./scripts/migrate.sh
# Env: POSTGRES_DSN (default: localhost assistant DB)
set -e
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
export POSTGRES_DSN="${POSTGRES_DSN:-postgres://postgres:postgres@localhost:5432/assistant?sslmode=disable}"
for f in "$ROOT/db-zone/migrations"/*.up.sql; do
  [ -f "$f" ] || continue
  name=$(basename "$f" .up.sql)
  echo "Applying $name..."
  psql "$POSTGRES_DSN" -f "$f" || true
done
