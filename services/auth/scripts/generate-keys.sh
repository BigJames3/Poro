#!/usr/bin/env sh
# Generates the RSA key pair used by the auth service.
# Writes keys/private.pem and keys/public.pem next to this service.
set -eu

root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
mkdir -p "$root/keys"

openssl genrsa -out "$root/keys/private.pem" 4096
openssl rsa -in "$root/keys/private.pem" -pubout -out "$root/keys/public.pem"

echo "wrote $root/keys/private.pem"
echo "wrote $root/keys/public.pem"
