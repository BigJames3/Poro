# Generates the RSA key pair used by the auth service.
# Writes keys/private.pem and keys/public.pem next to this service.
$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$keyDir = Join-Path $root "keys"
New-Item -ItemType Directory -Force -Path $keyDir | Out-Null

$private = Join-Path $keyDir "private.pem"
$public = Join-Path $keyDir "public.pem"

& openssl genrsa -out $private 4096
if ($LASTEXITCODE -ne 0) { throw "openssl genrsa failed" }
& openssl rsa -in $private -pubout -out $public
if ($LASTEXITCODE -ne 0) { throw "openssl rsa failed" }

Write-Output "wrote $private"
Write-Output "wrote $public"
