#!/bin/sh
# Aborts the multipart uploads left unfinished for more than a day, in every
# bucket, then sleeps an hour (PORO-008). A safety net under the video
# service's own sweep: it runs even when no service does. Runs in the
# chrislusf/seaweedfs image (busybox sh) as the seaweedfs-janitor service.
#
# Environment:
#   WEED_MASTER         master address (default seaweedfs:9333)
#   JANITOR_MAX_AGE     age of an abandoned multipart upload (default 24h)
#   JANITOR_INTERVAL    seconds between two passes (default 3600)
#   JANITOR_ONCE        set to 1 to run a single pass and exit with its status
set -eu

master="${WEED_MASTER:-seaweedfs:9333}"
max_age="${JANITOR_MAX_AGE:-24h}"
interval="${JANITOR_INTERVAL:-3600}"

case "$max_age" in
  *[!0-9hm.]* | "") echo "storage-janitor: JANITOR_MAX_AGE must look like 24h or 90m" >&2; exit 1 ;;
esac
case "$interval" in
  *[!0-9]* | "") echo "storage-janitor: JANITOR_INTERVAL must be a number of seconds" >&2; exit 1 ;;
esac

while :; do
  # A failed pass is retried at the next one; the loop never exits on it.
  if out=$(echo "s3.clean.uploads -timeAgo=$max_age" | weed shell -master="$master" 2>&1); then
    purged=$(printf '%s\n' "$out" | grep -c '^purge ' || true)
    echo "storage-janitor: $purged abandoned multipart upload(s) aborted"
  else
    echo "storage-janitor: pass failed, retrying in ${interval}s" >&2
    [ "${JANITOR_ONCE:-}" != 1 ] || exit 1
  fi
  [ "${JANITOR_ONCE:-}" != 1 ] || exit 0
  sleep "$interval"
done
