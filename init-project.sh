#!/usr/bin/env bash
set -euo pipefail

PROJECT_NAME="${1:-poro}"
ROOT="$(pwd)/$PROJECT_NAME"

echo "🚀 Création du projet : $PROJECT_NAME"

# ─── Structure racine ──────────────────────────────────────────
mkdir -p "$ROOT"/{apps,services,packages,infra,data,docs,.github/workflows}

# ─── Apps ──────────────────────────────────────────────────────
mkdir -p "$ROOT"/apps/{mobile-android,mobile-ios,admin-web,seller-web}
mkdir -p "$ROOT"/apps/mobile-android/app/src/main/{java,res}
mkdir -p "$ROOT"/apps/mobile-android/app/src/main/java/com/tiktokafrica

# ─── Services backend ──────────────────────────────────────────
for svc in auth user social video feed shop order payment chat notification moderation search analytics reco; do
  mkdir -p "$ROOT"/services/$svc
done

# ─── Packages partagés ─────────────────────────────────────────
mkdir -p "$ROOT"/packages/{proto,shared-go,shared-ts,shared-kotlin}

# ─── Infra ─────────────────────────────────────────────────────
mkdir -p "$ROOT"/infra/{terraform,helm,k8s,docker}
mkdir -p "$ROOT"/infra/terraform/{environments,modules}
mkdir -p "$ROOT"/infra/terraform/environments/{dev,staging,prod}

# ─── Data ──────────────────────────────────────────────────────
mkdir -p "$ROOT"/data/{migrations,seeds}

# ─── Docs ──────────────────────────────────────────────────────
mkdir -p "$ROOT"/docs/{architecture,api,runbooks}

# ─── Fichiers racine ───────────────────────────────────────────
cat > "$ROOT/.gitignore" <<'EOF'
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
*.xcworkspace/xcuserdata/

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
!.vscode/extensions.json
.cursor/*
!.cursor/rules

# OS
.DS_Store
Thumbs.db
EOF

cat > "$ROOT/README.md" <<EOF
# $PROJECT_NAME

TikTok Afrique + Marketplace intégrée.

## Structure
- \`apps/\` — Applications (mobile, web)
- \`services/\` — Microservices backend
- \`packages/\` — Librairies partagées
- \`infra/\` — Terraform, Helm, K8s
- \`data/\` — Migrations SQL
- \`docs/\` — Documentation

## Démarrage rapide
\`\`\`bash
docker compose up -d
\`\`\`
EOF

cat > "$ROOT/.env.example" <<'EOF'
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
EOF

cat > "$ROOT/docker-compose.yml" <<'EOF'
version: "3.9"

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
    command: >
      redpanda start --overprovisioned --smp 1 --memory 1G
      --reserve-memory 0M --node-id 0 --check=false
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
EOF

cat > "$ROOT/Makefile" <<'EOF'
.PHONY: help up down logs ps clean

help:
	@echo "Commandes disponibles :"
	@echo "  make up      - Démarrer les services"
	@echo "  make down    - Arrêter les services"
	@echo "  make logs    - Voir les logs"
	@echo "  make ps      - État des services"
	@echo "  make clean   - Tout supprimer"

up:
	docker compose up -d

down:
	docker compose down

logs:
	docker compose logs -f

ps:
	docker compose ps

clean:
	docker compose down -v
EOF

# ─── GitHub Actions CI de base ────────────────────────────────
cat > "$ROOT/.github/workflows/ci.yml" <<'EOF'
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
EOF

# ─── Cursor rules ─────────────────────────────────────────────
mkdir -p "$ROOT/.cursor/rules"
cat > "$ROOT/.cursor/rules/project.md" <<'EOF'
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
EOF

# ─── Git init ─────────────────────────────────────────────────
cd "$ROOT"
git init -q
git add .
git commit -q -m "chore: initial project structure"

echo ""
echo "✅ Projet créé : $ROOT"
echo ""
echo "👉 Prochaines étapes :"
echo "   cd $PROJECT_NAME"
echo "   make up"
echo "   code ."