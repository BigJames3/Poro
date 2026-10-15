#!/usr/bin/env bash
# Runs the object-storage access-control test (ADR-0015): SeaweedFS with the
# image pinned in docker-compose.yml, configured by the real init.sh.
# The video service's storage-migrate command then moves the legacy sources
# with the video identity, as Compose does, and the video, user and shop
# storage code runs against it: SeaweedFS now checks every signature.
# Usage: infra/storage/tests/run.sh   (from anywhere; needs Docker, Go, the
# npm dependencies of services/user and services/shop, and python3 with
# boto3: pip install -r infra/storage/tests/requirements.txt)
set -euo pipefail

here="$(cd "$(dirname "$0")" && pwd)"
root="$(cd "$here/../../.." && pwd)"
image=$(awk '/^  seaweedfs:$/ { found = 1 } found && /image:/ { print $2; exit }' "$root/docker-compose.yml")
network="poro-storage-test-$$"
port="${STORAGE_TEST_PORT:-18333}"
endpoint="http://127.0.0.1:${port}"
python="${PYTHON:-python3}"

if [ -z "$image" ]; then
  echo "seaweedfs image not found in docker-compose.yml"
  exit 1
fi

# Test-only credentials, generated per run.
random() { "$python" -c 'import secrets; print(secrets.token_urlsafe(24))'; }
S3_USER_ACCESS_KEY_ID="user-$(random)"
S3_USER_SECRET_ACCESS_KEY="$(random)"
S3_VIDEO_ACCESS_KEY_ID="video-$(random)"
S3_VIDEO_SECRET_ACCESS_KEY="$(random)"
S3_SHOP_ACCESS_KEY_ID="shop-$(random)"
S3_SHOP_SECRET_ACCESS_KEY="$(random)"
export S3_USER_ACCESS_KEY_ID S3_USER_SECRET_ACCESS_KEY S3_VIDEO_ACCESS_KEY_ID \
  S3_VIDEO_SECRET_ACCESS_KEY S3_SHOP_ACCESS_KEY_ID S3_SHOP_SECRET_ACCESS_KEY

server=""
workdir=$(mktemp -d)
cleanup() {
  if [ -n "$server" ]; then
    docker rm -f "$server" >/dev/null 2>&1 || true
  fi
  docker network rm "$network" >/dev/null 2>&1 || true
  rm -rf "$workdir"
}
trap cleanup EXIT

docker network create "$network" >/dev/null
server=$(docker run -d --network "$network" --network-alias seaweedfs -p "127.0.0.1:${port}:8333" \
  "$image" server -s3 -dir=/data -master.volumeSizeLimitMB=64 -volume.max=50)

ready=false
for _ in $(seq 1 60); do
  if docker exec "$server" wget -q -O /dev/null http://127.0.0.1:9333/cluster/status 2>/dev/null &&
    curl -s -o /dev/null "$endpoint/"; then
    ready=true
    break
  fi
  sleep 1
done
if [ "$ready" != true ]; then
  echo "seaweedfs did not start"
  docker logs "$server" 2>&1 | tail -n 40
  exit 1
fi

init() {
  docker run --rm --network "$network" \
    -e S3_USER_ACCESS_KEY_ID -e S3_USER_SECRET_ACCESS_KEY \
    -e S3_VIDEO_ACCESS_KEY_ID -e S3_VIDEO_SECRET_ACCESS_KEY \
    -e S3_SHOP_ACCESS_KEY_ID -e S3_SHOP_SECRET_ACCESS_KEY \
    -v "$root/infra/storage/init.sh:/init.sh:ro" \
    --entrypoint /bin/sh "$image" /init.sh
}

check() {
  if ! "$python" "$here/check_storage.py" "$endpoint" "$1"; then
    docker logs "$server" 2>&1 | grep -iE "error|denied" | tail -n 20
    exit 1
  fi
}

# Wait until the volume server accepts writes, then store what an earlier
# version left behind.
for _ in $(seq 1 30); do
  if "$python" "$here/check_storage.py" "$endpoint" seed >/dev/null 2>&1; then
    break
  fi
  sleep 1
done
check seed

init
# The S3 gateway reloads its IAM configuration asynchronously.
sleep 3

# Build outside the repository and run from an empty directory: the command
# must not pick up a developer's .env.
(cd "$root/services/video" && go build -o "$workdir/storage-migrate" ./cmd/storage-migrate)
(
  cd "$workdir"
  APP_ENV=dev LOG_LEVEL=info \
    S3_ENDPOINT="$endpoint" S3_PUBLIC_ENDPOINT="$endpoint" S3_REGION=us-east-1 \
    S3_BUCKET=poro-videos S3_SOURCE_BUCKET=poro-video-sources S3_QUARANTINE_BUCKET=poro-quarantine \
    S3_ACCESS_KEY_ID="$S3_VIDEO_ACCESS_KEY_ID" S3_SECRET_ACCESS_KEY="$S3_VIDEO_SECRET_ACCESS_KEY" \
    S3_FORCE_PATH_STYLE=true ./storage-migrate
)
check matrix

export S3_INTEGRATION_ENDPOINT="$endpoint"
(cd "$root/services/video" && go test -count=1 -v -run '^TestS3AgainstSeaweedFS$' ./internal/storage/ | grep -E '^(--- |ok|FAIL)')
(cd "$root/services/user" && npx jest --ci test/seaweedfs.e2e.spec.ts)
(cd "$root/services/shop" && npx jest --ci test/seaweedfs.e2e.spec.ts)

# A second run is idempotent and applies a rotated secret.
export S3_VIDEO_OLD_SECRET_ACCESS_KEY="$S3_VIDEO_SECRET_ACCESS_KEY"
S3_VIDEO_SECRET_ACCESS_KEY="$(random)"
export S3_VIDEO_SECRET_ACCESS_KEY
init
sleep 3
check rotated

# init.sh refuses a shared access key and does not leak secrets to its logs.
if output=$(S3_SHOP_ACCESS_KEY_ID="$S3_USER_ACCESS_KEY_ID" init 2>&1); then
  echo "init.sh accepted two services sharing an access key"
  exit 1
fi
case "$output" in
  *"own S3 access key"*) ;;
  *) echo "unexpected init.sh error: $output"; exit 1 ;;
esac
log=$(init 2>&1)
for secret in "$S3_USER_SECRET_ACCESS_KEY" "$S3_VIDEO_SECRET_ACCESS_KEY" "$S3_SHOP_SECRET_ACCESS_KEY"; do
  case "$log" in
    *"$secret"*) echo "init.sh printed a secret"; exit 1 ;;
  esac
done
echo "storage: init.sh is idempotent, rejects shared keys and keeps secrets out of its logs"
