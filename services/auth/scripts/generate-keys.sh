#!/usr/bin/env sh
# Generates the RSA key pair used by the auth service.
# Writes keys/private.pem and keys/public.pem next to this service.
# Refuses to overwrite an existing pair (that would sign out every user) unless --force is given.
set -eu

root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
mkdir -p "$root/keys"

if [ -f "$root/keys/private.pem" ] && [ "${1:-}" != "--force" ]; then
  echo "$root/keys/private.pem already exists. Re-run with --force to replace it and invalidate every issued token." >&2
  exit 1
fi

openssl genrsa -out "$root/keys/private.pem" 4096
openssl rsa -in "$root/keys/private.pem" -pubout -out "$root/keys/public.pem"

echo "wrote $root/keys/private.pem"
echo "wrote $root/keys/public.pem"
