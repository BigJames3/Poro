#!/usr/bin/env bash
# Runs the end-to-end journeys of the Android app against the whole stack,
# built from source by docker compose, through the gateway on 127.0.0.1:8000.
# The stack and its volumes are removed afterwards (E2E_KEEP=1 keeps them).
# Usage: tests/e2e/run.sh   (needs Docker, openssl and python3)
set -euo pipefail

root="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$root"
# Every compose command below reads the documented development values.
export COMPOSE_ENV_FILES="$root/.env.example"
services="auth:8081 user:8082 video:8083 feed:8084 social:8085 notification:8086 moderation:8087 search:8088 shop:8090 order:8091"

# Auth signs tokens with a key pair that is never committed.
keys="$root/services/auth/keys"
if [ ! -f "$keys/private.pem" ]; then
  openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$keys/private.pem" 2>/dev/null
  openssl rsa -in "$keys/private.pem" -pubout -out "$keys/public.pem" 2>/dev/null
  chmod 0644 "$keys/"*.pem
fi

cleanup() {
  if [ "${E2E_KEEP:-0}" != 1 ]; then
    docker compose down -v --remove-orphans >/dev/null 2>&1 || true
  fi
}
trap cleanup EXIT

dump() {
  docker compose ps -a
  for svc in auth user video media-worker feed social search notification moderation shop order gateway \
    outbox-relay-auth outbox-relay-user outbox-relay-video outbox-relay-social outbox-relay-shop outbox-relay-order; do
    echo "----- $svc"
    docker compose logs --no-log-prefix --tail 40 "$svc" 2>&1 || true
  done
}

docker compose up -d --build

# Each service answers ready once its database, Redis and Kafka are reachable.
for entry in $services gateway:8000; do
  name="${entry%%:*}"
  port="${entry##*:}"
  path=/health/ready
  [ "$name" = gateway ] && path=/health
  ready=false
  for _ in $(seq 1 120); do
    if curl -fsS -o /dev/null "http://127.0.0.1:${port}${path}"; then
      ready=true
      break
    fi
    sleep 2
  done
  if [ "$ready" != true ]; then
    echo "$name never became ready"
    dump
    exit 1
  fi
done

# Traefik checks each backend every 10 s: wait until it routes to every
# service (any answer but 502, 503 or 504 means the route works).
for path in /api/v1/auth/me /api/v1/users/me /api/v1/videos /api/v1/feed/public/trending \
  /api/v1/comments/x/replies /api/v1/notifications /api/v1/moderation/cases \
  /api/v1/search/suggest /api/v1/shops/me /api/v1/cart; do
  routed=false
  for _ in $(seq 1 60); do
    code=$(curl -s -o /dev/null -w '%{http_code}' "http://127.0.0.1:8000${path}" || true)
    case "$code" in
      502 | 503 | 504 | 000) sleep 1 ;;
      *)
        routed=true
        break
        ;;
    esac
  done
  if [ "$routed" != true ]; then
    echo "the gateway never routed ${path} (last status ${code})"
    dump
    exit 1
  fi
done

if ! python3 "$root/tests/e2e/e2e.py"; then
  dump
  exit 1
fi
