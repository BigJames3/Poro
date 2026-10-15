#!/usr/bin/env bash
# deploy.sh against a real Docker Compose project (PORO-010): a healthy
# version goes live, an unhealthy one or a failing migration rolls back to the
# previous version, and bad input is refused.
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
work=$(mktemp -d)
reg="poro-deploy-test-$$"
export COMPOSE_PROJECT_NAME="poro-deploy-test-$$"
good=$(printf 'a%.0s' {1..40})
bad=$(printf 'b%.0s' {1..40})
broken=$(printf 'c%.0s' {1..40})

cleanup() {
  (cd "$work" && docker compose -f compose.yml down --remove-orphans >/dev/null 2>&1) || true
  docker image rm -f "$reg/app:sha-$good" "$reg/app:sha-$bad" "$reg/app:sha-$broken" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "ok    $*"; }

# Any small image with a shell will do; the Redis image of the stack is
# always present where the stack runs.
if docker image inspect alpine:latest >/dev/null 2>&1; then
  base=alpine:latest
else
  base=$(awk '/^  redis:$/ { found = 1 } found && /image:/ { print $2; exit }' "$root/docker-compose.yml")
fi

build() { # sha, healthy (0/1), migration exit code
  mkdir -p "$work/img-$1"
  cat >"$work/img-$1/Dockerfile" <<EOF
FROM $base
RUN echo "$1" > /version
HEALTHCHECK --interval=1s --timeout=1s --start-period=1s --retries=2 CMD exit $2
CMD ["sh", "-c", "if [ \"\$ROLE\" = migrate ]; then exit $3; fi; exec sleep 3600"]
EOF
  docker build -q -t "$reg/app:sha-$1" "$work/img-$1" >/dev/null
}
build "$good" 0 0
build "$bad" 1 0
build "$broken" 0 1

cat >"$work/compose.yml" <<'EOF'
services:
  migrate:
    image: ${PORO_IMAGE_REGISTRY}/app:${PORO_IMAGE_TAG}
    environment: { ROLE: migrate }
    healthcheck: { disable: true }
    restart: "no"
  app:
    image: ${PORO_IMAGE_REGISTRY}/app:${PORO_IMAGE_TAG}
    depends_on:
      migrate: { condition: service_completed_successfully }
EOF
echo "UNUSED=1" >"$work/.env.staging"

deploy() {
  (cd "$work" && PORO_IMAGE_REGISTRY="$reg" DEPLOY_COMPOSE_FILES=compose.yml DEPLOY_SKIP_PULL=1 \
    DEPLOY_WAIT_TIMEOUT=30 "$root/deploy/deploy.sh" "$@")
}
running() { (cd "$work" && docker compose -f compose.yml exec -T app cat /version); }

deploy "$good" >/dev/null || fail "healthy first version did not deploy"
[ "$(cat "$work/.deploy/current")" = "$good" ] || fail "current version not recorded"
[ "$(running)" = "$good" ] || fail "app does not run the deployed version"
pass "healthy version deployed and recorded"

if deploy "$bad" >"$work/out" 2>&1; then fail "unhealthy version reported as deployed"; fi
grep -q "rolled back to $good" "$work/out" || fail "no rollback message: $(cat "$work/out")"
[ "$(cat "$work/.deploy/current")" = "$good" ] || fail "current version changed after a failed deploy"
[ "$(running)" = "$good" ] || fail "app was not rolled back"
pass "unhealthy version rolled back to the previous one"

if deploy "$broken" >"$work/out" 2>&1; then fail "failing migration reported as deployed"; fi
grep -q "rolled back to $good" "$work/out" || fail "no rollback after a failing migration"
[ "$(running)" = "$good" ] || fail "app does not run the previous version after a failing migration"
pass "failing migration rolled back to the previous one"

grep -q "$bad failed" "$work/.deploy/history" || fail "history does not record the failed deploy"
grep -q "$good rollback ok" "$work/.deploy/history" || fail "history does not record the rollback"
pass "history records deployments, failures and rollbacks"

if deploy "not-a-sha" >/dev/null 2>&1; then fail "accepted a malformed sha"; fi
rm "$work/.env.staging"
if deploy "$good" >/dev/null 2>&1; then fail "deployed without the host secrets file"; fi
pass "malformed sha and missing secrets file refused"

echo "deploy: all checks passed"
