#!/bin/sh
# Run Postgres migrations. Usage: ./scripts/migrate.sh [up|down]
# Requires POSTGRES_DSN or set connection via env.
set -e
cd "$(dirname "$0")/.."
export POSTGRES_DSN="${POSTGRES_DSN:-postgres://postgres:postgres@localhost:5432/assistant?sslmode=disable}"
for f in migrations/*.up.sql; do
  [ -f "$f" ] || continue
  name=$(basename "$f" .up.sql)
  echo "Applying $name..."
  psql "$POSTGRES_DSN" -f "$f" || true
done
