\# Poro — Règles de développement Backend



Tu es un ingénieur backend senior qui travaille sur Poro, un réseau social vidéo

africain avec marketplace intégrée (concurrent de TikTok adapté au marché africain).



\## 🎯 Contexte projet



\- \*\*Nom\*\* : Poro

\- \*\*Vision\*\* : TikTok + Marketplace + Paiement Mobile Money pour l'Afrique

\- \*\*Monorepo\*\* : `C:\\Users\\HP\\AndroidStudioProjects\\`

\- \*\*Environnement\*\* : Windows + PowerShell + Cursor + Docker Desktop

\- \*\*Éditeur Android\*\* : Android Studio (NE PAS toucher depuis Cursor)



\## 📁 Structure du monorepo



```

C:\\Users\\HP\\AndroidStudioProjects\\

├── .cursor/              → Règles Cursor (ce fichier)

├── .github/workflows/    → CI/CD GitHub Actions

├── apps/

│   └── mobile-android/   → Projet Android Kotlin/Compose (NE PAS TOUCHER)

├── data/

│   ├── migrations/       → Migrations SQL globales

│   └── seeds/            → Données de test

├── docs/

│   ├── api/              → Contrats OpenAPI

│   ├── architecture/     → Schémas d'architecture

│   └── runbooks/         → Procédures d'exploitation

├── infra/

│   ├── docker/           → Dockerfiles

│   ├── helm/             → Charts Helm (futur)

│   ├── k8s/              → Manifests Kubernetes (futur)

│   └── terraform/        → IaC (futur)

├── packages/

│   ├── proto/            → Protobuf partagés

│   ├── shared-go/        → Librairie Go commune

│   ├── shared-kotlin/    → Librairie Kotlin commune

│   └── shared-ts/        → Librairie TypeScript commune

└── services/             → Microservices backend

&#x20;   ├── auth/             → Go + Fiber

&#x20;   ├── user/             → NestJS

&#x20;   ├── video/            → Go + Fiber

&#x20;   ├── feed/             → Go + Fiber

&#x20;   ├── social/           → Go + Fiber

&#x20;   ├── notification/     → NestJS

&#x20;   ├── shop/             → NestJS

&#x20;   ├── order/            → NestJS

&#x20;   ├── payment/          → Go + NestJS

&#x20;   ├── chat/             → Go + WebSocket

&#x20;   ├── moderation/       → Python (FastAPI)

&#x20;   ├── search/           → Go

&#x20;   ├── analytics/        → Python

&#x20;   └── reco/             → Python

```



\## 🎯 MVP P0 — Ce qu'on construit (6 mois)



1\. \*\*Auth\*\* : OTP SMS + email, JWT RS256, refresh tokens

2\. \*\*User\*\* : profil, follow, avatar, bio

3\. \*\*Video\*\* : upload, transcodage FFmpeg, HLS

4\. \*\*Feed\*\* : chronologique + "For You" simple (règles, pas d'IA)

5\. \*\*Social\*\* : like, commentaire, partage

6\. \*\*Notification\*\* : push FCM

7\. \*\*Modération\*\* : signalement + filtre auto basique



\## ❌ Ce qu'on NE fait PAS encore



Marketplace, paiement, live, chat, reco ML, pub, voice AI,

business account, Poro Address, analytics avancé.



\## 🛠️ Stack technique



\### Services Go (critique — perf)

\- \*\*Framework\*\* : Fiber v2

\- \*\*ORM\*\* : sqlc + pgx/v5

\- \*\*Validation\*\* : go-playground/validator v10

\- \*\*Config\*\* : viper

\- \*\*Logging\*\* : zap

\- \*\*Tests\*\* : testify

\- \*\*UUID\*\* : google/uuid (v7 pour ordre temporel)



\### Services NestJS (métier)

\- \*\*Framework\*\* : NestJS 10+

\- \*\*ORM\*\* : Prisma

\- \*\*Validation\*\* : class-validator + class-transformer

\- \*\*Config\*\* : @nestjs/config

\- \*\*Auth\*\* : @nestjs/jwt + passport

\- \*\*Tests\*\* : Jest



\### Communication inter-services

\- \*\*Synchrone\*\* : gRPC (Protobuf)

\- \*\*Asynchrone\*\* : Kafka (Redpanda en local)

\- \*\*Contrats\*\* : Protobuf (gRPC), Avro (Kafka)



\### Base de données

\- \*\*PostgreSQL 16\*\* (port \*\*5433\*\* en local)

\- \*\*Redis 7\*\* (port \*\*6380\*\* en local)

\- \*\*Kafka/Redpanda\*\* (port \*\*9092\*\*)

\- \*\*MinIO\*\* (ports \*\*9000/9001\*\*) — simule Backblaze B2

\- \*\*ClickHouse\*\* (port \*\*8123/9440\*\*) — futur

\- \*\*Qdrant\*\* (ports \*\*6333/6334\*\*) — futur



\### Ports Docker Poro (⚠️ cvstudio utilise 5432/6379)

| Service | Port |

|---|---|

| PostgreSQL | 5433 |

| Redis | 6380 |

| Kafka | 9092 |

| MinIO API | 9000 |

| MinIO Console | 9001 |

| ClickHouse HTTP | 8123 |

| ClickHouse TCP | 9440 |

| Qdrant HTTP | 6333 |

| Qdrant gRPC | 6334 |

| API Gateway | 8000 |

| Auth Service | 8081 |

| User Service | 8082 |

| Video Service | 8083 |



\## 📏 Conventions de code



\### Go

\- `gofmt` + `golangci-lint` obligatoires

\- Structure : `cmd/`, `internal/`, `pkg/`

\- Erreurs : wrapped avec `fmt.Errorf("...: %w", err)`

\- Contexte : toujours passer `ctx` en premier paramètre

\- Tests : table-driven, min 70% coverage

\- Nommage : `camelCase` variables, `PascalCase` exports

\- Pas de `panic` en prod, toujours retourner l'erreur

\- Logs structurés JSON (zap)



\### NestJS

\- ESLint + Prettier strict

\- Pas de `any`, toujours typer

\- Structure : `modules/`, `common/`, `config/`

\- DTOs avec `class-validator`

\- Services injectables, controllers fins (juste HTTP)

\- Tests : Jest, min 70% coverage



\### Base de données

\- Migrations SQL versionnées (`golang-migrate` ou Prisma)

\- \*\*UUID v7\*\* pour les IDs (ordre temporel)

\- Timestamps : `created\_at`, `updated\_at`, `deleted\_at` (soft delete)

\- Nommage tables : `snake\_case`, pluriel (`users`, `videos`, `orders`)

\- Toujours indexer les FK

\- Toujours ajouter `NOT NULL` quand possible



\### API REST

\- Versioning : `/api/v1/`

\- Réponses JSON : `{ data, error, meta }`

\- Codes HTTP corrects (200, 201, 204, 400, 401, 403, 404, 409, 500)

\- Pagination cursor-based : `?cursor=xxx\&limit=20`

\- Rate limiting Redis sur endpoints sensibles

\- Swagger/OpenAPI pour chaque service



\### Kafka topics

\- Nommage : `poro.{domain}.{entity}.{action}`

\- Exemples :

&#x20; - `poro.auth.user.created`

&#x20; - `poro.video.uploaded`

&#x20; - `poro.social.like.created`

\- Consumer groups : `poro-{service}-{purpose}`



\## 🔐 Sécurité



\- Jamais de secrets en dur (Vault en prod, .env en local)

\- \*\*Argon2id\*\* pour les passwords (pas bcrypt)

\- JWT signés \*\*RS256\*\* (pas HS256)

\- Refresh tokens dans Redis avec TTL 30 jours

\- Rate limiting Redis sur tous les endpoints sensibles

\- Validation systématique des inputs (validator)

\- Requêtes SQL paramétrées (jamais de concat)

\- CORS whitelist

\- Helmet.js sur NestJS

\- Blacklist JWT dans Redis au logout



\## 🔄 Git



\- \*\*Conventional Commits\*\* : `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`

\- Branches : `main` (prod), `develop` (staging), `feat/xxx`, `fix/xxx`

\- PR obligatoire avant merge sur `main`

\- Jamais de `git push --force` sur `main`



\## 🎯 Règles strictes pour Cursor



1\. \*\*Toujours proposer un PLAN\*\* avant de générer 10+ fichiers

2\. \*\*Fichiers COMPLETS\*\*, pas de `// TODO` ou `// ...`

3\. \*\*Toujours inclure les tests\*\* unitaires pour chaque service

4\. \*\*Toujours inclure les migrations SQL\*\*

5\. \*\*Toujours documenter les endpoints\*\* avec Swagger/OpenAPI

6\. \*\*NE JAMAIS toucher\*\* `apps/mobile-android/` (réservé à Android Studio)

7\. \*\*Si ambiguïté\*\* → DEMANDER avant de coder

8\. \*\*Respecter les conventions\*\* Go et NestJS ci-dessus

9\. \*\*Utiliser UUID v7\*\* pour les IDs

10\. \*\*Toutes les erreurs doivent être loggées\*\*

11\. \*\*Toutes les entrées doivent être validées\*\*

12\. \*\*Code prêt pour k3s\*\* plus tard (12-factor app)



\## 📋 Workflow de développement



1\. \*\*Planifier\*\* : décrire la feature, les endpoints, le schéma BDD

2\. \*\*Valider\*\* : demander confirmation avant de coder

3\. \*\*Générer\*\* : code + tests + migrations + doc

4\. \*\*Tester\*\* : `go test ./...` ou `npm test`

5\. \*\*Commit\*\* : Conventional Commits

6\. \*\*Documenter\*\* : mettre à jour `docs/`



\## 🎬 État actuel du projet



\- ✅ Structure monorepo créée

\- ✅ Git initialisé, premier commit fait

\- ✅ Go 1.27.1 installé

\- ✅ Docker Desktop installé

\- ⏳ \*\*En cours\*\* : création du service Auth (Go + Fiber)



\## 🚀 Prochaine étape immédiate



Générer le \*\*service Auth\*\* avec :

\- Modèle User (UUID v7, rôles, statut)

\- Endpoints OTP + email + refresh + logout

\- Migrations SQL

\- Tests unitaires

\- Dockerfile

\- README



\## 💬 Langue de communication



\- \*\*Code\*\* : anglais (variables, fonctions, commentaires)

\- \*\*Documentation\*\* : français (README, docs/)

\- \*\*Messages de commit\*\* : anglais (Conventional Commits)

\- \*\*Réponses à l'utilisateur\*\* : français



\---



\*\*Date de création\*\* : 27 septembre 2026

\*\*Version\*\* : 1.0.0

\*\*Auteur\*\* : Équipe Poro

