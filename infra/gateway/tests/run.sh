#!/usr/bin/env bash
# Runs the gateway contract test: Traefik with the real configuration, one
# whoami stub per service under the service's compose name.
# Usage: infra/gateway/tests/run.sh   (from anywhere; needs Docker and python3)
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$here/../../.." && pwd)"
traefik_image="traefik:v3.7.14"
whoami_image="traefik/whoami:v1.12.0"
network="poro-gateway-test-$$"
port="${GATEWAY_TEST_PORT:-18000}"
services="auth:8081 user:8082 video:8083 feed:8084 social:8085 notification:8086 moderation:8087 search:8088 shop:8090 order:8091"

containers=()
cleanup() {
  if [ "${#containers[@]}" -gt 0 ]; then
    docker rm -f "${containers[@]}" >/dev/null 2>&1 || true
  fi
  docker network rm "$network" >/dev/null 2>&1 || true
}
trap cleanup EXIT

docker network create "$network" >/dev/null
for entry in $services; do
  name="${entry%%:*}"
  svc_port="${entry##*:}"
  id=$(docker run -d --network "$network" --network-alias "$name" --hostname "$name" \
    "$whoami_image" --port "$svc_port")
  containers+=("$id")
done

id=$(docker run -d --network "$network" -p "127.0.0.1:${port}:8000" \
  -e GATEWAY_CORS_ORIGINS="https://app.poro.test, https://admin.poro.test" \
  -v "$root/infra/gateway/traefik.yml:/etc/traefik/traefik.yml:ro" \
  -v "$root/infra/gateway/routes.yml:/etc/traefik/routes.yml:ro" \
  --user 65534:65534 --read-only \
  "$traefik_image" --configFile=/etc/traefik/traefik.yml)
containers+=("$id")

# Wait for the gateway, then for the first backend health checks (every 10s).
ready=false
for _ in $(seq 1 60); do
  if curl -fsS "http://127.0.0.1:${port}/health" >/dev/null 2>&1; then
    ready=true
    break
  fi
  sleep 1
done
if [ "$ready" != true ]; then
  echo "gateway did not start"
  docker logs "$id" 2>&1 | tail -n 40
  exit 1
fi
sleep 12

if ! python3 "$here/check_gateway.py" "http://127.0.0.1:${port}" "$root/docs/api"; then
  docker logs "$id" 2>&1 | tail -n 40
  exit 1
fi
