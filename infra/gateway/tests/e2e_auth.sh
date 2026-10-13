#!/usr/bin/env bash
# End-to-end check of the gateway with a real service: the auth binary, built
# from source, runs behind Traefik with Postgres and Redis. It signs up and
# reads the account through the gateway, checks that internal
# routes stay closed, and that auth records the client IP (not the gateway's)
# because it trusts the gateway's X-Forwarded-For.
# Usage: infra/gateway/tests/e2e_auth.sh   (needs Docker, Go, openssl, python3)
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$here/../../.." && pwd)"
work="$(mktemp -d)"
network="poro-gateway-e2e-$$"
port="${GATEWAY_E2E_PORT:-18001}"
containers=()
cleanup() {
  if [ "${#containers[@]}" -gt 0 ]; then
    docker rm -f "${containers[@]}" >/dev/null 2>&1 || true
  fi
  docker network rm "$network" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT

(cd "$root/services/auth" && CGO_ENABLED=0 go build -o "$work/auth" ./cmd/server)
mkdir -p "$work/keys"
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out "$work/keys/private.pem" 2>/dev/null
openssl rsa -in "$work/keys/private.pem" -pubout -out "$work/keys/public.pem" 2>/dev/null
chmod 0644 "$work/keys/"*.pem

docker network create "$network" >/dev/null
containers+=("$(docker run -d --network "$network" --network-alias postgres \
  -e POSTGRES_USER=poro -e POSTGRES_PASSWORD=e2e -e POSTGRES_DB=poro_auth postgres:16-alpine)")
containers+=("$(docker run -d --network "$network" --network-alias redis redis:7-alpine)")
for _ in $(seq 1 30); do
  docker exec "${containers[0]}" pg_isready -U poro -d poro_auth >/dev/null 2>&1 && break
  sleep 1
done
sleep 2

auth=$(docker run -d --network "$network" --network-alias auth \
  -v "$work/auth:/app/auth:ro" -v "$work/keys:/app/keys:ro" \
  -v "$root/services/auth/migrations:/app/migrations:ro" \
  -e APP_ENV=dev -e APP_PORT=8081 -e LOG_LEVEL=info \
  -e POSTGRES_HOST=postgres -e POSTGRES_PORT=5432 -e POSTGRES_USER=poro -e POSTGRES_PASSWORD=e2e \
  -e POSTGRES_DB=poro_auth -e POSTGRES_SSLMODE=disable -e REDIS_HOST=redis -e REDIS_PORT=6379 \
  -e JWT_PRIVATE_KEY_PATH=/app/keys/private.pem -e JWT_PUBLIC_KEY_PATH=/app/keys/public.pem \
  -e MIGRATIONS_PATH=/app/migrations -e KAFKA_BROKERS=redpanda:9092 \
  -e TRUSTED_PROXIES=172.16.0.0/12,192.168.0.0/16 \
  alpine:3.22 /app/auth)
containers+=("$auth")

gateway=$(docker run -d --network "$network" -p "127.0.0.1:${port}:8000" \
  -v "$root/infra/gateway/traefik.yml:/etc/traefik/traefik.yml:ro" \
  -v "$root/infra/gateway/routes.yml:/etc/traefik/routes.yml:ro" \
  --user 65534:65534 --read-only traefik:v3.7.14 --configFile=/etc/traefik/traefik.yml)
containers+=("$gateway")

base="http://127.0.0.1:${port}"
for _ in $(seq 1 60); do
  [ "$(curl -s -o /dev/null -w '%{http_code}' "$base/api/v1/auth/me")" = "401" ] && break
  sleep 1
done

if ! python3 - "$base" "$(docker network inspect "$network" -f '{{(index .IPAM.Config 0).Gateway}}')" \
  "$(docker inspect "$gateway" -f "{{(index .NetworkSettings.Networks \"$network\").IPAddress}}")" <<'PY'
import json, sys, urllib.error, urllib.request

base, client_ip, gateway_ip = sys.argv[1:4]

def call(method, path, body=None, token=None):
    req = urllib.request.Request(base + path, method=method,
                                 data=json.dumps(body).encode() if body is not None else None)
    req.add_header("Content-Type", "application/json")
    req.add_header("X-Forwarded-For", "6.6.6.6")
    if token:
        req.add_header("Authorization", "Bearer " + token)
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            return resp.status, json.loads(resp.read() or b"null")
    except urllib.error.HTTPError as err:
        return err.code, None

status, body = call("POST", "/api/v1/auth/email/register",
                    {"email": "e2e@poro.test", "password": "mot-de-passe-e2e", "full_name": "Awa E2E",
                     "device_name": "Gateway E2E"})
assert status == 201, status
token = body["data"]["access_token"]
status, body = call("GET", "/api/v1/auth/me", token=token)
assert status == 200 and body["data"]["email"] == "e2e@poro.test", (status, body)
status, body = call("GET", "/api/v1/auth/sessions", token=token)
assert status == 200, status
session = body["data"][0]
assert session["device"] == "Gateway E2E", session
assert session["ip_address"] == client_ip, (session["ip_address"], client_ip, gateway_ip)
for path in ("/health/ready", "/metrics", "/.well-known/jwks.json"):
    assert call("GET", path)[0] == 404, path
print(f"gateway e2e: sign-up, /me and /sessions through the gateway; auth saw {client_ip}, "
      f"not the gateway ({gateway_ip}) nor the forged 6.6.6.6; internal routes closed")
PY
then
  docker logs "$auth" 2>&1 | tail -n 20
  docker logs "$gateway" 2>&1 | tail -n 20
  exit 1
fi
