# init-project.ps1
param(
    [string]$ProjectName = "poro"
)

$ErrorActionPreference = "Stop"
$Root = Join-Path (Get-Location) $ProjectName

Write-Host "🚀 Création du projet : $ProjectName" -ForegroundColor Cyan

# ─── Structure racine ──────────────────────────────────────────
$folders = @(
    "apps/mobile-android/app/src/main/java/com/tiktokafrica",
    "apps/mobile-ios",
    "apps/admin-web",
    "apps/seller-web",
    "packages/proto",
    "packages/shared-go",
    "packages/shared-ts",
    "packages/shared-kotlin",
    "infra/terraform/environments/dev",
    "infra/terraform/environments/staging",
    "infra/terraform/environments/prod",
    "infra/terraform/modules",
    "infra/helm",
    "infra/k8s",
    "infra/docker",
    "data/migrations",
    "data/seeds",
    "docs/architecture",
    "docs/api",
    "docs/runbooks",
    ".github/workflows",
    ".cursor/rules"
)

# Services backend
$services = @("auth","user","social","video","feed","shop","order","payment","chat","notification","moderation","search","analytics","reco")
foreach ($svc in $services) {
    $folders += "services/$svc"
}

foreach ($folder in $folders) {
    $fullPath = Join-Path $Root $folder
    New-Item -ItemType Directory -Force -Path $fullPath | Out-Null
}

# ─── .gitignore ────────────────────────────────────────────────
$gitignore = @'
# Node
node_modules/
dist/
build/
.next/
.turbo/

# Go
*.exe
*.test
*.out
vendor/

# Kotlin/Android
*.iml
.gradle/
local.properties
.idea/
captures/
.externalNativeBuild/
.cxx/
*.apk
*.aab

# iOS
Pods/
DerivedData/

# Python
__pycache__/
*.pyc
.venv/
venv/

# Env
.env
.env.*
!.env.example

# Secrets
*.pem
*.key
*.p12
*.jks
*.keystore

# IDE
.vscode/*
!.vscode/settings.json
.cursor/*
!.cursor/rules

# OS
.DS_Store
Thumbs.db
'@
Set-Content -Path (Join-Path $Root ".gitignore") -Value $gitignore

# ─── README ────────────────────────────────────────────────────
$readme = @"
# $ProjectName

TikTok Afrique + Marketplace intégrée.

## Structure
- apps/ — Applications (mobile, web)
- services/ — Microservices backend
- packages/ — Librairies partagées
- infra/ — Terraform, Helm, K8s
- data/ — Migrations SQL
- docs/ — Documentation

## Démarrage rapide
docker compose up -d
"@
Set-Content -Path (Join-Path $Root "README.md") -Value $readme

# ─── .env.example ──────────────────────────────────────────────
$envExample = @'
# Database
POSTGRES_HOST=localhost
POSTGRES_PORT=5432
POSTGRES_USER=app
POSTGRES_PASSWORD=changeme
POSTGRES_DB=tiktok_africa

# Redis
REDIS_HOST=localhost
REDIS_PORT=6379

# Storage (Backblaze B2)
B2_KEY_ID=
B2_APP_KEY=
B2_BUCKET=

# CDN
BUNNY_API_KEY=
BUNNY_PULL_ZONE=

# Payment
CINETPAY_API_KEY=
CINETPAY_SITE_ID=
FLUTTERWAVE_SECRET_KEY=

# Auth
JWT_SECRET=changeme
OTP_PROVIDER_KEY=
'@
Set-Content -Path (Join-Path $Root ".env.example") -Value $envExample

# ─── docker-compose.yml ────────────────────────────────────────
$dockerCompose = @'
services:
  postgres:
    image: postgres:16-alpine
    environment:
      POSTGRES_USER: app
      POSTGRES_PASSWORD: changeme
      POSTGRES_DB: tiktok_africa
    ports: ["5432:5432"]
    volumes: [pgdata:/var/lib/postgresql/data]

  redis:
    image: redis:7-alpine
    ports: ["6379:6379"]

  minio:
    image: minio/minio
    command: server /data --console-address ":9001"
    environment:
      MINIO_ROOT_USER: minioadmin
      MINIO_ROOT_PASSWORD: minioadmin
    ports: ["9000:9000", "9001:9001"]
    volumes: [miniodata:/data]

  kafka:
    image: redpandadata/redpanda:latest
    command: redpanda start --overprovisioned --smp 1 --memory 1G --reserve-memory 0M --node-id 0 --check=false
    ports: ["9092:9092"]

  clickhouse:
    image: clickhouse/clickhouse-server:latest
    ports: ["8123:8123", "9000:9000"]
    volumes: [chdata:/var/lib/clickhouse]

  qdrant:
    image: qdrant/qdrant:latest
    ports: ["6333:6333"]
    volumes: [qdrantdata:/qdrant/storage]

volumes:
  pgdata:
  miniodata:
  chdata:
  qdrantdata:
'@
Set-Content -Path (Join-Path $Root "docker-compose.yml") -Value $dockerCompose

# ─── Makefile (équivalent PowerShell) ─────────────────────────
$makefile = @'
# Sur Windows, utiliser les commandes PowerShell :
# .\dev.ps1 up / down / logs / ps / clean
'@
Set-Content -Path (Join-Path $Root "Makefile") -Value $makefile

# ─── dev.ps1 — équivalent du Makefile Windows ─────────────────
$devPs1 = @'
param([string]$Command = "help")

switch ($Command) {
    "up"     { docker compose up -d }
    "down"   { docker compose down }
    "logs"   { docker compose logs -f }
    "ps"     { docker compose ps }
    "clean"  { docker compose down -v }
    "help"   {
        Write-Host "Commandes disponibles :" -ForegroundColor Cyan
        Write-Host "  .\dev.ps1 up      - Démarrer les services"
        Write-Host "  .\dev.ps1 down    - Arrêter les services"
        Write-Host "  .\dev.ps1 logs    - Voir les logs"
        Write-Host "  .\dev.ps1 ps      - État des services"
        Write-Host "  .\dev.ps1 clean   - Tout supprimer"
    }
    default  { Write-Host "Commande inconnue : $Command" }
}
'@
Set-Content -Path (Join-Path $Root "dev.ps1") -Value $devPs1

# ─── CI GitHub Actions ────────────────────────────────────────
$ciYaml = @'
name: CI

on:
  push:
    branches: [main, develop]
  pull_request:
    branches: [main, develop]

jobs:
  detect-changes:
    runs-on: ubuntu-latest
    outputs:
      backend: ${{ steps.filter.outputs.backend }}
      mobile: ${{ steps.filter.outputs.mobile }}
    steps:
      - uses: actions/checkout@v4
      - uses: dorny/paths-filter@v3
        id: filter
        with:
          filters: |
            backend:
              - 'services/**'
              - 'packages/shared-go/**'
              - 'packages/shared-ts/**'
            mobile:
              - 'apps/mobile-android/**'

  backend-test:
    needs: detect-changes
    if: ${{ needs.detect-changes.outputs.backend == 'true' }}
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - run: echo "Tests backend à configurer"

  mobile-build:
    needs: detect-changes
    if: ${{ needs.detect-changes.outputs.mobile == 'true' }}
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-java@v4
        with: { distribution: 'temurin', java-version: '17' }
      - run: echo "Build Android à configurer"
'@
Set-Content -Path (Join-Path $Root ".github/workflows/ci.yml") -Value $ciYaml

# ─── Cursor rules ─────────────────────────────────────────────
$cursorRules = @'
# Règles du projet

## Architecture
- Monorepo Turborepo
- Backend : Go (critique) + NestJS (métier)
- Mobile : Kotlin + Compose (Android), Swift (iOS)
- Infra : Terraform + k3s

## Conventions
- Commits : Conventional Commits
- Go : gofmt + golangci-lint
- TS : ESLint + Prettier strict
- Kotlin : ktlint + detekt
- Jamais de secrets en dur
- Toujours valider les inputs
'@
Set-Content -Path (Join-Path $Root ".cursor/rules/project.md") -Value $cursorRules

# ─── Git init ─────────────────────────────────────────────────
Push-Location $Root
git init -q
git add .
git commit -q -m "chore: initial project structure"
Pop-Location

Write-Host ""
Write-Host "✅ Projet créé : $Root" -ForegroundColor Green
Write-Host ""
Write-Host "👉 Prochaines étapes :" -ForegroundColor Yellow
Write-Host "   cd $ProjectName"
Write-Host "   .\dev.ps1 up"
Write-Host "   code ."