#!/usr/bin/env bash
# Encrypted logical backup of every PORO database (PORO-012).
#
# Each database is dumped with pg_dump (custom format) and encrypted with age
# for the public keys in BACKUP_AGE_RECIPIENTS before touching the disk: the
# backup host and the backup bucket never hold readable data, and the private
# key stays offline. A manifest with the SHA-256 of every file is written last
# and uploaded last: a backup without its manifest is incomplete.
#
# Environment:
#   PGHOST, PGPORT, PGUSER, PGPASSWORD   source server (libpq variables)
#   BACKUP_AGE_RECIPIENTS               file of age public keys (required)
#   BACKUP_DIR                          local directory (default ./backups)
#   BACKUP_DATABASES                    space-separated list (default: every poro_* database)
#   BACKUP_RETENTION_DAYS               local copies kept (default 14)
#   BACKUP_S3_ENDPOINT, BACKUP_S3_BUCKET, BACKUP_S3_REGION,
#   BACKUP_S3_ACCESS_KEY_ID, BACKUP_S3_SECRET_ACCESS_KEY
#                                       optional off-site copy, with write-only
#                                       credentials (no read, no delete)
set -euo pipefail

die() { echo "backup: $*" >&2; exit 1; }
log() { echo "backup: $*"; }

[ -n "${BACKUP_AGE_RECIPIENTS:-}" ] || die "BACKUP_AGE_RECIPIENTS is required: backups are never written unencrypted"
[ -s "$BACKUP_AGE_RECIPIENTS" ] || die "BACKUP_AGE_RECIPIENTS ($BACKUP_AGE_RECIPIENTS) is empty or missing"
for tool in pg_dump psql age sha256sum; do
  command -v "$tool" >/dev/null || die "$tool not found"
done

backup_dir="${BACKUP_DIR:-./backups}"
retention="${BACKUP_RETENTION_DAYS:-14}"
case "$retention" in '' | *[!0-9]*) die "BACKUP_RETENTION_DAYS must be a number of days" ;; esac

databases="${BACKUP_DATABASES:-}"
if [ -z "$databases" ]; then
  databases=$(psql -X -At -d postgres -c \
    "SELECT datname FROM pg_database WHERE datname LIKE 'poro\_%' AND NOT datistemplate ORDER BY datname")
fi
# One line, space separated (psql prints one name per line).
databases=$(printf "%s\n" "$databases" | tr -s "[:space:]" " " | sed "s/ $//")
[ -n "$databases" ] || die "no database to back up"

stamp=$(date -u +%Y%m%dT%H%M%SZ)
target="$backup_dir/$stamp"
mkdir -p "$target"
chmod 700 "$target"
umask 077
# An interrupted backup leaves nothing that could be mistaken for a good one.
complete=false
trap '[ "$complete" = true ] || rm -rf -- "$target"' EXIT

server_version=$(psql -X -At -d postgres -c 'SHOW server_version')
start=$(date +%s)
for db in $databases; do
  case "$db" in *[!a-z0-9_]*) die "unexpected database name: $db" ;; esac
  out="$target/$db.dump.age"
  # pipefail: a failed pg_dump fails the backup even though age succeeded.
  pg_dump --format=custom --no-owner --no-privileges -d "$db" 2>"$target/$db.err" |
    age -R "$BACKUP_AGE_RECIPIENTS" -o "$out" ||
    die "$db: $(head -c 500 "$target/$db.err")"
  rm -f "$target/$db.err"
  log "$db: $(wc -c <"$out") bytes"
done

{
  echo "# PORO backup $stamp"
  echo "# server_version $server_version"
  echo "# databases $databases"
  (cd "$target" && sha256sum -- *.dump.age)
} >"$target/MANIFEST"
log "manifest written ($(($(date +%s) - start)) s)"

if [ -n "${BACKUP_S3_ENDPOINT:-}" ]; then
  for var in BACKUP_S3_BUCKET BACKUP_S3_ACCESS_KEY_ID BACKUP_S3_SECRET_ACCESS_KEY; do
    [ -n "${!var:-}" ] || die "$var is required with BACKUP_S3_ENDPOINT"
  done
  region="${BACKUP_S3_REGION:-us-east-1}"
  upload() {
    # Credentials go through stdin, never the command line (visible in ps).
    printf 'user = "%s:%s"\n' "$BACKUP_S3_ACCESS_KEY_ID" "$BACKUP_S3_SECRET_ACCESS_KEY" |
      curl --config - --fail --silent --show-error --retry 3 --retry-all-errors \
        --aws-sigv4 "aws:amz:$region:s3" \
        --upload-file "$1" "${BACKUP_S3_ENDPOINT%/}/$BACKUP_S3_BUCKET/$stamp/$(basename "$1")" >/dev/null
  }
  for f in "$target"/*.dump.age; do upload "$f"; done
  upload "$target/MANIFEST"
  log "uploaded to $BACKUP_S3_BUCKET/$stamp/"
fi

complete=true

# Local retention only. Remote copies expire through the bucket's lifecycle
# rule: these credentials cannot delete anything.
find "$backup_dir" -mindepth 1 -maxdepth 1 -type d -name '????????T??????Z' -mtime +"$retention" -print |
  while read -r old; do
    rm -rf -- "$old"
    log "removed local copy $(basename "$old")"
  done

echo "$target"
