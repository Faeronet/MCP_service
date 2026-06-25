#!/usr/bin/env bash 
# Поднимает зоны по очереди. В каждой зоне свой docker-compose.yml и свой .env (рабочая директория — каталог зоны).
# GPU: MCP_WITH_VLLM=1 ./stack-up.sh
# Остановка: ./stack-up.sh down
# Привязка портов на VPS: внутренние — только localhost; публичные — сайт, админка, tg-bot.
export MCP_INTERNAL_BIND="${MCP_INTERNAL_BIND:-127.0.0.1}"
export MCP_PUBLIC_BIND="${MCP_PUBLIC_BIND:-0.0.0.0}"
export MCP_IMAGE_REGISTRY="${MCP_IMAGE_REGISTRY:-}"
export MCP_IMAGE_TAG="${MCP_IMAGE_TAG:-latest}"
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"

wait_tcp() {
  local host="$1" port="$2" msg="$3" i
  for i in $(seq 1 90); do
    if command -v nc >/dev/null 2>&1; then
      nc -z "$host" "$port" && echo "OK: $msg" && return 0
    elif bash -c "echo > /dev/tcp/${host}/${port}" 2>/dev/null; then
      echo "OK: $msg" && return 0
    fi
    sleep 1
  done
  echo "Timeout: $msg (${host}:${port})" >&2
  return 1
}

zone() {
  ( cd "$ROOT/$1" && docker compose "${@:2}" )
}

admin_zone_compose() {
  ( cd "$ROOT/admin-zone" && bash ./render-compose-bind.sh && docker compose -f docker-compose.yml -f docker-compose.bind.generated.yml "$@" )
}

if [[ "${1:-up}" == "down" ]]; then
  zone tg-bot down --remove-orphans
  admin_zone_compose down --remove-orphans
  zone angels-web down --remove-orphans
  zone notification down --remove-orphans
  zone workers down --remove-orphans
  zone chat-orchestrator down --remove-orphans
  zone mcp-files down --remove-orphans
  zone ai-zone --profile vllm down --remove-orphans 2>/dev/null || zone ai-zone down --remove-orphans
  zone log-zone down --remove-orphans
  zone file-orchestrator down --remove-orphans
  zone db-zone down --remove-orphans
  docker network rm mcp_stack 2>/dev/null || true
  echo "Все зоны остановлены."
  exit 0
fi

docker network create mcp_stack 2>/dev/null || true

zone db-zone up -d postgres redis qdrant
wait_tcp 127.0.0.1 5433 postgres
zone db-zone up migrate

zone file-orchestrator up -d
wait_tcp 127.0.0.1 9000 minio
wait_tcp 127.0.0.1 5672 rabbitmq

zone log-zone up -d
wait_tcp 127.0.0.1 3100 loki

if [[ "${MCP_WITH_VLLM:-}" == "1" ]]; then
  zone ai-zone --profile vllm up -d
else
  zone ai-zone up -d promtail
fi

zone mcp-files up -d
wait_tcp 127.0.0.1 8001 mcp-write

zone chat-orchestrator up -d
wait_tcp 127.0.0.1 8083 mcp-proxy

zone workers up -d

zone notification up -d
wait_tcp 127.0.0.1 8090 scheduler

zone angels-web up -d

admin_zone_compose up -d

zone tg-bot up -d

echo "Готово. Публично (${MCP_PUBLIC_BIND}): angels-web :3000."
echo "Admin UI :5173 — только на IP из ADMIN_BIND_HOSTS (admin-zone/.env). API — внутри Docker."
echo "tg-bot — long polling, :8081 только ${MCP_INTERNAL_BIND}. Остальное — ${MCP_INTERNAL_BIND} или без проброса."
