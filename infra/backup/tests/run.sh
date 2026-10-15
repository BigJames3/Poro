#!/usr/bin/env bash
# Backup and restore round trip against a real Postgres (PORO-012).
# Needs docker, pg_dump/pg_restore/psql 16+, age and curl; runs in CI.
#
# It checks that a backup is encrypted, restores identically, is rejected
# when altered or opened with the wrong key, never leaves a partial backup,
# and reaches an S3 bucket (SeaweedFS) when configured.
set -euo pipefail

root=$(cd "$(dirname "$0")/../../.." && pwd)
work=$(mktemp -d)
pg="poro-backup-test-$$"
s3="poro-backup-s3-$$"
cleanup() {
  docker rm -f "$pg" "$s3" >/dev/null 2>&1 || true
  rm -rf "$work"
}
trap cleanup EXIT

fail() { echo "FAIL: $*" >&2; exit 1; }
pass() { echo "ok    $*"; }

image=$(awk '/^  postgres:$/ { found = 1 } found && /image:/ { print $2; exit }' "$root/docker-compose.yml")
password=$(head -c 18 /dev/urandom | base64 | tr -dc 'A-Za-z0-9')
docker run -d --name "$pg" -e POSTGRES_USER=poro -e POSTGRES_PASSWORD="$password" \
  -e POSTGRES_DB=postgres -p 127.0.0.1::5432 "$image" >/dev/null
port=$(docker port "$pg" 5432/tcp | head -1 | sed 's/.*://')
export PGHOST=127.0.0.1 PGPORT="$port" PGUSER=poro PGPASSWORD="$password"
for _ in $(seq 1 60); do
  psql -X -q -d postgres -c 'SELECT 1' >/dev/null 2>&1 && break
  sleep 1
done

secret="phone-+2250700000000-$$"
for db in poro_auth poro_order; do
  psql -X -q -d postgres -c "CREATE DATABASE $db"
  psql -X -q -d "$db" -c "CREATE TABLE schema_migrations (version bigint, dirty boolean);
    INSERT INTO schema_migrations VALUES (7, false);
    CREATE TABLE users (id serial PRIMARY KEY, phone text);
    INSERT INTO users (phone) SELECT '$secret' FROM generate_series(1, 500);"
done
psql -X -q -d postgres -c "CREATE DATABASE other_app"

age-keygen -o "$work/key.txt" 2>/dev/null
age-keygen -y "$work/key.txt" >"$work/recipients.txt"
age-keygen -o "$work/wrong.txt" 2>/dev/null

# 1. Refuses to run without encryption.
if BACKUP_DIR="$work/b" "$root/infra/backup/backup.sh" >/dev/null 2>&1; then
  fail "backup ran without BACKUP_AGE_RECIPIENTS"
fi
pass "no backup without encryption"

# 2. Backs up every poro_* database, encrypted.
dir=$(BACKUP_DIR="$work/b" BACKUP_AGE_RECIPIENTS="$work/recipients.txt" "$root/infra/backup/backup.sh" | tail -1)
for db in poro_auth poro_order; do
  [ -f "$dir/$db.dump.age" ] || fail "missing $db dump in $dir"
done
[ ! -e "$dir/other_app.dump.age" ] || fail "backed up a database outside poro_*"
if grep -a -q -e "$secret" -e PGDMP "$dir"/*.dump.age; then fail "plaintext found in the backup"; fi
[ "$(stat -c %a "$dir")" = 700 ] || fail "backup directory readable by others"
pass "every poro_* database dumped and encrypted (no plaintext, mode 700)"

# 3. Restores with the right key, with the same rows.
export RESTORE_ALLOW_HOST=127.0.0.1
BACKUP_AGE_IDENTITY="$work/key.txt" "$root/infra/backup/restore_verify.sh" "$dir" | tee "$work/restore.log"
grep -q "poro_auth: ok, 2 tables, migrations: schema_migrations" "$work/restore.log" || fail "restore report"
psql -X -q -d postgres -c "CREATE DATABASE roundtrip"
age -d -i "$work/key.txt" "$dir/poro_auth.dump.age" | pg_restore --no-owner -d roundtrip
[ "$(psql -X -At -d roundtrip -c "SELECT count(*) FROM users WHERE phone = '$secret'")" = 500 ] ||
  fail "restored rows differ"
[ "$(psql -X -At -d postgres -c "SELECT count(*) FROM pg_database WHERE datname LIKE 'verify\_%'")" = 0 ] ||
  fail "restore-verify left scratch databases behind"
pass "restored identically; scratch databases dropped"

# 4. Wrong key, altered file, guard against a non-scratch host.
if BACKUP_AGE_IDENTITY="$work/wrong.txt" "$root/infra/backup/restore_verify.sh" "$dir" >/dev/null 2>&1; then
  fail "restored with the wrong key"
fi
cp -r "$dir" "$work/tampered"
printf 'X' | dd of="$work/tampered/poro_order.dump.age" bs=1 seek=200 conv=notrunc 2>/dev/null
if BACKUP_AGE_IDENTITY="$work/key.txt" "$root/infra/backup/restore_verify.sh" "$work/tampered" >/dev/null 2>&1; then
  fail "accepted an altered backup"
fi
if RESTORE_ALLOW_HOST=prod-db BACKUP_AGE_IDENTITY="$work/key.txt" \
  "$root/infra/backup/restore_verify.sh" "$dir" >/dev/null 2>&1; then
  fail "ran without the scratch-host confirmation"
fi
pass "wrong key, altered file and unconfirmed host refused"

# 5. A failing dump leaves no partial backup behind.
before=$(find "$work/b" -mindepth 1 -maxdepth 1 -type d | wc -l)
if BACKUP_DIR="$work/b" BACKUP_DATABASES="poro_auth poro_missing" BACKUP_AGE_RECIPIENTS="$work/recipients.txt" \
  "$root/infra/backup/backup.sh" >/dev/null 2>&1; then
  fail "backup of a missing database succeeded"
fi
sleep 1
[ "$(find "$work/b" -mindepth 1 -maxdepth 1 -type d | wc -l)" = "$before" ] || fail "partial backup left on disk"
pass "failed backup leaves nothing behind"

# 6. Local retention removes old copies only.
mkdir -p "$work/b/20200101T000000Z" "$work/b/keep-me"
touch -d '30 days ago' "$work/b/20200101T000000Z" "$work/b/keep-me"
BACKUP_DIR="$work/b" BACKUP_AGE_RECIPIENTS="$work/recipients.txt" BACKUP_RETENTION_DAYS=14 \
  "$root/infra/backup/backup.sh" >/dev/null
[ ! -e "$work/b/20200101T000000Z" ] || fail "old backup kept"
[ -d "$work/b/keep-me" ] || fail "retention removed a directory it does not own"
pass "retention removes old backups only"

# 7. Off-site copy to an S3 bucket.
s3image=$(awk '/^  seaweedfs:$/ { found = 1 } found && /image:/ { print $2; exit }' "$root/docker-compose.yml")
docker run -d --name "$s3" -p 127.0.0.1::8333 "$s3image" server -s3 -dir=/data >/dev/null
s3port=$(docker port "$s3" 8333/tcp | head -1 | sed 's/.*://')
for _ in $(seq 1 60); do
  curl -s -o /dev/null "http://127.0.0.1:$s3port/" && break
  sleep 1
done
curl -s -X PUT "http://127.0.0.1:$s3port/poro-backups" >/dev/null
dir=$(BACKUP_DIR="$work/c" BACKUP_AGE_RECIPIENTS="$work/recipients.txt" \
  BACKUP_S3_ENDPOINT="http://127.0.0.1:$s3port" BACKUP_S3_BUCKET=poro-backups \
  BACKUP_S3_ACCESS_KEY_ID=backup-writer BACKUP_S3_SECRET_ACCESS_KEY=not-a-real-secret \
  "$root/infra/backup/backup.sh" | tail -1)
stamp=$(basename "$dir")
curl -sf "http://127.0.0.1:$s3port/poro-backups/$stamp/MANIFEST" -o "$work/remote-manifest"
cmp -s "$work/remote-manifest" "$dir/MANIFEST" || fail "remote manifest differs"
curl -sf "http://127.0.0.1:$s3port/poro-backups/$stamp/poro_order.dump.age" | cmp -s - "$dir/poro_order.dump.age" ||
  fail "remote dump differs"
pass "off-site copy uploaded, identical to the local one"

echo "backup round trip: all checks passed"
