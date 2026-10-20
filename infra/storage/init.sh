#!/bin/sh
# Configures SeaweedFS access control for Poro (ADR-0015). Idempotent: run by
# the seaweedfs-init Compose service on every `docker compose up`.
#
# 1. Creates the buckets. Public buckets hold only re-encoded media; originals
#    and quarantined media live in private buckets.
# 2. Deletes raw uploads left in public buckets by earlier versions.
# 3. Replaces the whole S3 IAM configuration: one identity per service, each
#    limited to its own buckets, and anonymous read on the public buckets only.
#    Anonymous clients can neither list, write nor create buckets.
#
# Runs in the chrislusf/seaweedfs image (busybox sh). Secrets come from the
# environment and never reach the logs: the IAM file lives in a private
# temporary directory and every command that echoes it is silenced.
#
# Environment:
#   WEED_MASTER                       master address (default seaweedfs:9333)
#   S3_USER_ACCESS_KEY_ID  / S3_USER_SECRET_ACCESS_KEY
#   S3_VIDEO_ACCESS_KEY_ID / S3_VIDEO_SECRET_ACCESS_KEY
#   S3_SHOP_ACCESS_KEY_ID  / S3_SHOP_SECRET_ACCESS_KEY
set -eu

master="${WEED_MASTER:-seaweedfs:9333}"
public_buckets="poro-videos poro-avatars poro-shop"
private_buckets="poro-video-sources poro-quarantine poro-avatar-uploads poro-shop-uploads"

shell() {
  echo "$1" | weed shell -master="$master" 2>&1
}

fail() {
  echo "storage-init: $*" >&2
  exit 1
}

# Keys go into a JSON document: refuse anything that could break out of a string.
check_credential() {
  name="$1"
  value="$2"
  [ -n "$value" ] || fail "$name is required"
  case "$value" in
    *[!A-Za-z0-9._~+/=-]*) fail "$name may only contain A-Z a-z 0-9 . _ ~ + / = -" ;;
  esac
  [ "${#value}" -ge 8 ] || fail "$name must be at least 8 characters"
}

check_credential S3_USER_ACCESS_KEY_ID "${S3_USER_ACCESS_KEY_ID:-}"
check_credential S3_USER_SECRET_ACCESS_KEY "${S3_USER_SECRET_ACCESS_KEY:-}"
check_credential S3_VIDEO_ACCESS_KEY_ID "${S3_VIDEO_ACCESS_KEY_ID:-}"
check_credential S3_VIDEO_SECRET_ACCESS_KEY "${S3_VIDEO_SECRET_ACCESS_KEY:-}"
check_credential S3_SHOP_ACCESS_KEY_ID "${S3_SHOP_ACCESS_KEY_ID:-}"
check_credential S3_SHOP_SECRET_ACCESS_KEY "${S3_SHOP_SECRET_ACCESS_KEY:-}"
if [ "$S3_USER_ACCESS_KEY_ID" = "$S3_VIDEO_ACCESS_KEY_ID" ] ||
  [ "$S3_USER_ACCESS_KEY_ID" = "$S3_SHOP_ACCESS_KEY_ID" ] ||
  [ "$S3_VIDEO_ACCESS_KEY_ID" = "$S3_SHOP_ACCESS_KEY_ID" ]; then
  fail "each service needs its own S3 access key id"
fi

until shell "cluster.check" >/dev/null; do sleep 2; done

existing=$(shell "s3.bucket.list" | awk '{print $1}')
for bucket in $public_buckets $private_buckets; do
  if ! printf '%s\n' "$existing" | grep -qx "$bucket"; then
    shell "s3.bucket.create -name $bucket" >/dev/null
    shell "s3.bucket.list" | awk '{print $1}' | grep -qx "$bucket" || fail "could not create bucket $bucket"
    echo "storage-init: created bucket $bucket"
  fi
done

# Earlier versions stored the originals next to the public media.
shell "fs.rm -r /buckets/poro-avatars/avatars/uploads" >/dev/null
shell "fs.rm -r /buckets/poro-shop/uploads" >/dev/null

# actions BUCKET... prints the JSON list of Read, Write and List on each bucket.
actions() {
  out=""
  for bucket in "$@"; do
    for action in Read Write List; do
      out="$out${out:+,}\"$action:$bucket\""
    done
  done
  printf '[%s]' "$out"
}

identity() {
  printf '{"name":"%s","credentials":[{"accessKey":"%s","secretKey":"%s"}],"actions":%s}' "$1" "$2" "$3" "$4"
}

anonymous=""
for bucket in $public_buckets; do
  anonymous="$anonymous${anonymous:+,}\"Read:$bucket\""
done

workdir=$(mktemp -d)
trap 'rm -rf "$workdir"' EXIT
umask 077
iam="$workdir/iam.json"
{
  printf '{"identities":['
  printf '{"name":"anonymous","actions":[%s]},' "$anonymous"
  identity poro-user "$S3_USER_ACCESS_KEY_ID" "$S3_USER_SECRET_ACCESS_KEY" "$(actions poro-avatars poro-avatar-uploads)"
  printf ','
  identity poro-video "$S3_VIDEO_ACCESS_KEY_ID" "$S3_VIDEO_SECRET_ACCESS_KEY" "$(actions poro-videos poro-video-sources poro-quarantine)"
  printf ','
  identity poro-shop "$S3_SHOP_ACCESS_KEY_ID" "$S3_SHOP_SECRET_ACCESS_KEY" "$(actions poro-shop poro-shop-uploads)"
  printf ']}\n'
} >"$iam"

shell "s3.iam.import -file $iam -apply" >/dev/null
summary=$(shell "s3.config.show")
for user in anonymous poro-user poro-video poro-shop; do
  printf '%s\n' "$summary" | grep -q "^ *$user " || fail "S3 identity $user is missing after import"
done
echo "storage-init: buckets and S3 access control are configured"
