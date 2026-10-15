#!/usr/bin/env bash
# Deploys one commit's images on a Compose host and rolls back automatically
# when the new version is not healthy (PORO-010).
#
#   deploy/deploy.sh <40-character commit sha>
#
# Runs ON the target host, from the repository checkout of that commit. The
# migration jobs run first (Compose dependencies); then every service must be
# running, and healthy when its image has a HEALTHCHECK, within the timeout.
# Otherwise the previous version is started again. Migrations must therefore
# be backward compatible (expand, then contract in a later release): the
# previous version runs on the migrated schema.
#
# Environment:
#   PORO_IMAGE_REGISTRY      e.g. ghcr.io/bigjames3 (required)
#   DEPLOY_COMPOSE_FILES     colon-separated compose files
#                            (default docker-compose.yml:deploy/staging/compose.staging.yml)
#   DEPLOY_ENV_FILE          secrets for interpolation (default .env.staging)
#   DEPLOY_STATE_DIR         where the deployed versions are recorded (default ./.deploy)
#   DEPLOY_WAIT_TIMEOUT      seconds to become healthy (default 300)
#   DEPLOY_SKIP_PULL=1       use images already on the host (tests)
set -euo pipefail

die() { echo "deploy: $*" >&2; exit 1; }
log() { echo "deploy: $*"; }

sha="${1:-}"
[[ "$sha" =~ ^[0-9a-f]{40}$ ]] || die "usage: $0 <40-character commit sha>"
[ -n "${PORO_IMAGE_REGISTRY:-}" ] || die "PORO_IMAGE_REGISTRY is required"

env_file="${DEPLOY_ENV_FILE:-.env.staging}"
[ -f "$env_file" ] || die "$env_file not found: secrets come from the host, never from the repository"
state_dir="${DEPLOY_STATE_DIR:-./.deploy}"
timeout="${DEPLOY_WAIT_TIMEOUT:-300}"
mkdir -p "$state_dir"

compose_args=(--env-file "$env_file")
IFS=: read -r -a files <<<"${DEPLOY_COMPOSE_FILES:-docker-compose.yml:deploy/staging/compose.staging.yml}"
for f in "${files[@]}"; do
  [ -f "$f" ] || die "compose file $f not found"
  compose_args+=(-f "$f")
done

compose() { PORO_IMAGE_TAG="sha-$1" docker compose "${compose_args[@]}" "${@:2}"; }

# One deployment at a time on this host.
exec 9>"$state_dir/lock"
flock -n 9 || die "another deployment is running"

start_version() { # sha
  if [ "${DEPLOY_SKIP_PULL:-}" != 1 ]; then
    compose "$1" pull --quiet || return 1
  fi
  compose "$1" up -d --no-build --remove-orphans --wait --wait-timeout "$timeout"
}

previous=""
[ -f "$state_dir/current" ] && previous=$(cat "$state_dir/current")

log "deploying $sha (previous: ${previous:-none})"
if start_version "$sha"; then
  [ -n "$previous" ] && [ "$previous" != "$sha" ] && echo "$previous" >"$state_dir/previous"
  echo "$sha" >"$state_dir/current"
  echo "$(date -u +%FT%TZ) $sha ok" >>"$state_dir/history"
  log "$sha is live and healthy"
  exit 0
fi

echo "$(date -u +%FT%TZ) $sha failed" >>"$state_dir/history"
compose "$sha" ps --all >&2 || true
if [ -z "$previous" ] || [ "$previous" = "$sha" ]; then
  die "$sha is not healthy and there is no previous version to roll back to"
fi
log "$sha is not healthy: rolling back to $previous"
if start_version "$previous"; then
  echo "$(date -u +%FT%TZ) $previous rollback ok" >>"$state_dir/history"
  die "rolled back to $previous; $sha needs a fix"
fi
echo "$(date -u +%FT%TZ) $previous rollback failed" >>"$state_dir/history"
die "rollback to $previous failed too: manual intervention needed (docs/runbooks/deploy.md)"
