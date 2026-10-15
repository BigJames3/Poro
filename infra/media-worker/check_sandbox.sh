#!/usr/bin/env bash
# Checks that a running media-worker container is confined as ADR-0007
# requires (PORO-007): non-root, no capabilities, no privilege escalation,
# read-only root, bounded /tmp, memory, CPU and processes, no host mounts.
#
#   infra/media-worker/check_sandbox.sh [container]   (default: poro-media-worker)
#
# Read-only: it inspects the container and runs a few commands inside it.
set -euo pipefail

name="${1:-poro-media-worker}"
failures=0

ok() { printf 'ok    %s\n' "$1"; }
ko() { printf 'FAIL  %s\n' "$1"; failures=$((failures + 1)); }
expect() { # description, actual, expected
  if [ "$2" = "$3" ]; then ok "$1"; else ko "$1 (got '$2', want '$3')"; fi
}

inspect() { docker inspect --format "$1" "$name"; }

if ! docker inspect "$name" >/dev/null 2>&1; then
  echo "container $name not found: start it with docker compose up -d media-worker" >&2
  exit 2
fi

expect "runs as uid 10001" "$(docker exec "$name" id -u)" "10001"
expect "privileged off" "$(inspect '{{.HostConfig.Privileged}}')" "false"
expect "all capabilities dropped" "$(inspect '{{json .HostConfig.CapDrop}}')" '["ALL"]'
expect "no capability added" "$(inspect '{{json .HostConfig.CapAdd}}')" "null"
expect "no effective capability" "$(docker exec "$name" awk '/^CapEff/ {print $2}' /proc/self/status)" "0000000000000000"
expect "no-new-privileges" "$(docker exec "$name" awk '/^NoNewPrivs/ {print $2}' /proc/self/status)" "1"
expect "seccomp filtering" "$(docker exec "$name" awk '/^Seccomp:/ {print $2}' /proc/self/status)" "2"
expect "read-only root filesystem" "$(inspect '{{.HostConfig.ReadonlyRootfs}}')" "true"
expect "memory limit 2 GiB" "$(inspect '{{.HostConfig.Memory}}')" "2147483648"
expect "no swap beyond memory" "$(inspect '{{.HostConfig.MemorySwap}}')" "2147483648"
expect "2 CPUs" "$(inspect '{{.HostConfig.NanoCpus}}')" "2000000000"
expect "process limit" "$(inspect '{{.HostConfig.PidsLimit}}')" "128"
expect "host network off" "$(inspect '{{.HostConfig.NetworkMode}}' | grep -cx host || true)" "0"
expect "host pid namespace off" "$(inspect '{{.HostConfig.PidMode}}')" ""
expect "no bind mount or volume" "$(inspect '{{len .Mounts}}')" "0"

expect "root mounted read-only" \
  "$(docker exec "$name" awk '$2 == "/" {split($4, o, ","); print o[1]}' /proc/mounts)" "ro"
if docker exec "$name" sh -c 'touch /tmp/probe && rm /tmp/probe'; then
  ok "/tmp is writable"
else
  ko "/tmp is not writable"
fi
expect "/tmp is a bounded tmpfs" \
  "$(docker exec "$name" awk '$2 == "/tmp" {print $3}' /proc/mounts)" "tmpfs"
expect "/tmp is noexec" \
  "$(docker exec "$name" awk '$2 == "/tmp" {print ($4 ~ /noexec/)}' /proc/mounts)" "1"

if [ "$failures" -ne 0 ]; then
  echo "$failures check(s) failed" >&2
  exit 1
fi
echo "media-worker sandbox: all checks passed"
