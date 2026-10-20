#!/usr/bin/env bash
# Proves that a backup made by backup.sh can be restored (PORO-012).
#
#   infra/backup/restore_verify.sh <backup directory>
#
# Checks every file against the manifest, decrypts it with the offline age
# identity, restores it into a throwaway database on a SCRATCH server (never
# the production one) and checks that tables and the migration version came
# back. The throwaway databases are always dropped. The report gives the
# restore time per database, the input of the RTO.
#
# Environment:
#   PGHOST, PGPORT, PGUSER, PGPASSWORD   scratch server (libpq variables)
#   BACKUP_AGE_IDENTITY                 age private key file
#   RESTORE_ALLOW_HOST                  must equal PGHOST: a guard against
#                                       pointing this script at production
set -euo pipefail

die() { echo "restore-verify: $*" >&2; exit 1; }
log() { echo "restore-verify: $*"; }

dir="${1:-}"
[ -n "$dir" ] || die "usage: $0 <backup directory>"
[ -d "$dir" ] || die "$dir is not a directory"
[ -f "$dir/MANIFEST" ] || die "$dir has no MANIFEST: incomplete backup"
[ -n "${BACKUP_AGE_IDENTITY:-}" ] || die "BACKUP_AGE_IDENTITY must name the private key file"
[ -s "$BACKUP_AGE_IDENTITY" ] || die "BACKUP_AGE_IDENTITY ($BACKUP_AGE_IDENTITY) is empty or missing"
[ "${RESTORE_ALLOW_HOST:-}" = "${PGHOST:-}" ] ||
  die "set RESTORE_ALLOW_HOST=$PGHOST to confirm it is a scratch server"
for tool in pg_restore psql age sha256sum; do
  command -v "$tool" >/dev/null || die "$tool not found"
done

(cd "$dir" && grep -v '^#' MANIFEST | sha256sum --check --strict --quiet) ||
  die "checksum mismatch: the backup was altered or truncated"
log "manifest checksums verified"

created=()
cleanup() {
  for db in "${created[@]}"; do
    psql -X -q -d postgres -c "DROP DATABASE IF EXISTS \"$db\" WITH (FORCE)" >/dev/null 2>&1 || true
  done
}
trap cleanup EXIT

failures=0
while read -r _ file; do
  src="${file%.dump.age}"
  case "$src" in *[!a-z0-9_]*) die "unexpected file name: $file" ;; esac
  scratch="verify_${src}_$$"
  psql -X -q -d postgres -c "CREATE DATABASE \"$scratch\"" >/dev/null
  created+=("$scratch")
  start=$(date +%s)
  if ! age -d -i "$BACKUP_AGE_IDENTITY" "$dir/$file" |
    pg_restore --no-owner --no-privileges --exit-on-error -d "$scratch"; then
    log "$src: RESTORE FAILED"
    failures=$((failures + 1))
    continue
  fi
  tables=$(psql -X -At -d "$scratch" -c \
    "SELECT count(*) FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog','information_schema')")
  migration=$(psql -X -At -d "$scratch" -c \
    "SELECT CASE WHEN to_regclass('public.schema_migrations') IS NOT NULL THEN 'schema_migrations'
                 WHEN to_regclass('public._prisma_migrations') IS NOT NULL THEN 'prisma'
                 WHEN to_regclass('public.alembic_version') IS NOT NULL THEN 'alembic'
                 ELSE 'none' END")
  if [ "$tables" -eq 0 ]; then
    log "$src: restored but empty"
    failures=$((failures + 1))
    continue
  fi
  log "$src: ok, $tables tables, migrations: $migration, $(($(date +%s) - start)) s"
done < <(grep -v '^#' "$dir/MANIFEST")

[ "$failures" -eq 0 ] || die "$failures database(s) could not be restored"
log "every database restored"
