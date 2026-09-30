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
| `infra/` | Terraform, Helm, Kubernetes (à venir) |
| `docs/` | Architecture, ADR, contrats d'API, runbooks |

## Démarrage rapide

```powershell
Copy-Item .env.example .env
docker compose up -d
```

Services : Postgres (bases `poro_auth` et `poro_user`), Redis, SeaweedFS, Redpanda
(topics + DLQ), auth `:8081`, user `:8082`, relais outbox. ClickHouse/Qdrant :
`docker compose --profile analytics up -d`.

## Documentation

- [État des lieux, architecture cible et backlog](docs/architecture/00-etat-des-lieux.md)
- [ADR](docs/architecture/adr/)
- [API auth (OpenAPI)](docs/api/auth.openapi.yaml)
- [API user (OpenAPI)](docs/api/user.openapi.yaml)
- [Runbook auth](docs/runbooks/auth.md)
- [Runbook user](docs/runbooks/user.md)
- [Runbook outbox / DLQ](docs/runbooks/outbox-relay.md)
