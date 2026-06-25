#!/usr/bin/env bash
# Генерирует docker-compose.bind.generated.yml — привязка admin-web-ui :5173 к IP интерфейсов хоста.
# ADMIN_BIND_HOSTS в .env: через запятую, например 127.0.0.1,10.8.0.1,192.168.1.50
set -euo pipefail
cd "$(dirname "$0")"
if [ -f .env ]; then set -a; # shellcheck disable=SC1091
  source .env; set +a; fi
HOSTS="${ADMIN_BIND_HOSTS:-127.0.0.1}"
OUT="docker-compose.bind.generated.yml"
{
  echo "# Сгенерировано render-compose-bind.sh — не редактировать вручную"
  echo "services:"
  echo "  admin-web-ui:"
  echo "    ports:"
  IFS=',' read -ra arr <<< "$HOSTS"
  for ip in "${arr[@]}"; do
    ip="${ip// /}"
    [ -n "$ip" ] || continue
    echo "      - \"${ip}:5173:80\""
  done
} > "$OUT"
echo "admin-zone: ports -> $OUT ($(grep -c ':5173:80' "$OUT" || true) bind(s), hosts=$HOSTS)"
