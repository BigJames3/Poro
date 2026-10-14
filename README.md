# PORO

Plateforme sociale vidéo africaine avec marketplace intégrée.

## Structure

| Dossier | Contenu |
|---|---|
| `apps/mobile-android` | Application Android (Kotlin, Compose) |
| `services/auth` | Service d'identité (Go) — [README](services/auth/README.md) |
| `services/user` | Profils, usernames, avatars, créateurs (NestJS) — [README](services/user/README.md) |
| `services/outbox-relay` | Publication de l'outbox SQL vers Kafka |
| `packages/shared-go` | HTTP, JWT/JWKS, santé, métriques, tracing, événements |
| `packages/contracts` | Schémas JSON des événements |
| `infra/` | Gateway Traefik ; Terraform, Helm, Kubernetes (à venir) |
| `docs/` | Architecture, ADR, contrats d'API, runbooks |

## Démarrage rapide

```powershell
Copy-Item .env.example .env
docker compose up -d
```

Chaque service exige `APP_ENV` (`dev`, `staging` ou `prod`) et ses secrets (mot de passe Postgres,
clés S3) : aucune valeur par défaut dans le code, pour qu'un oubli ne lance jamais la production
en mode dev. Compose les fournit ; pour lancer un service seul, copier son `.env.example`.

Services : Postgres (une base par service), Redis, SeaweedFS, Redpanda (topics + DLQ),
les services de l'API et leurs relais outbox. ClickHouse/Qdrant :
`docker compose --profile analytics up -d`.

Les clients passent par la gateway, **`http://127.0.0.1:8000/api/v1/...`**, point d'entrée
unique de l'API ([`infra/gateway`](infra/gateway/README.md)). Les ports directs des services
(`:8081` à `:8091`) restent ouverts en local pour le débogage.

## Documentation

- [État des lieux, architecture cible et backlog](docs/architecture/00-etat-des-lieux.md)
- [ADR](docs/architecture/adr/)
- [API auth (OpenAPI)](docs/api/auth.openapi.yaml)
- [API user (OpenAPI)](docs/api/user.openapi.yaml)
- [Runbook auth](docs/runbooks/auth.md)
- [Runbook user](docs/runbooks/user.md)
- [Runbook outbox / DLQ](docs/runbooks/outbox-relay.md)
- [Gateway](infra/gateway/README.md), [runbook gateway](docs/runbooks/gateway.md)
