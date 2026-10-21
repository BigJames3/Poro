# PORO

Plateforme sociale vidéo africaine avec marketplace intégrée.

## Structure

| Dossier | Contenu |
|---|---|
| `apps/mobile-android` | Application Android (Kotlin, Compose) |
| `services/` | Services de l'API, détaillés ci-dessous |
| `packages/shared-go` | HTTP, JWT/JWKS, santé, métriques, tracing, outbox/inbox, Kafka, catalogue d'événements |
| `packages/contracts` | Schémas JSON des événements (`poro.{domaine}.{entité}.{action}`) |
| `infra/` | Gateway Traefik, contrôle d'accès du stockage objet, sauvegardes, observabilité, isolement de FFmpeg ; Terraform, Helm, Kubernetes (à venir) |
| `deploy/` | Livraison en staging avec retour automatique en arrière ([runbook](docs/runbooks/deploy.md)) |
| `docs/` | Architecture, ADR, contrats d'API, runbooks |

## Services

| Service | Rôle | Pile | Port | Docs |
|---|---|---|---|---|
| [`auth`](services/auth/README.md) | Identité : OTP SMS, email et mot de passe, rôles multiples, sessions par appareil, suppression de compte, JWT RS256 et JWKS | Go | 8081 | [API](docs/api/auth.openapi.yaml), [runbook](docs/runbooks/auth.md) |
| [`user`](services/user/README.md) | Profils, usernames, avatars, activation créateur | NestJS | 8082 | [API](docs/api/user.openapi.yaml), [runbook](docs/runbooks/user.md) |
| [`video`](services/video/README.md) | Upload multipart présigné, métadonnées, lecture ; `media-worker` pour le transcodage HLS | Go + FFmpeg | 8083 | [API](docs/api/video.openapi.yaml), [runbook](docs/runbooks/video.md) |
| [`feed`](services/feed/README.md) | Flux abonnements, tendances, For You | Go | 8084 | [API](docs/api/feed.openapi.yaml), [runbook](docs/runbooks/feed.md) |
| [`social`](services/social/README.md) | Likes, commentaires, abonnements, partages, compteurs | Go | 8085 | [API](docs/api/social.openapi.yaml), [runbook](docs/runbooks/social.md) |
| [`notification`](services/notification/README.md) | Notifications in-app et push FCM | NestJS | 8086 | [API](docs/api/notification.openapi.yaml), [runbook](docs/runbooks/notification.md) |
| [`moderation`](services/moderation/README.md) | Signalements, filtre automatique, file des modérateurs | Python (FastAPI) | 8087 | [API](docs/api/moderation.openapi.yaml), [runbook](docs/runbooks/moderation.md) |
| [`search`](services/search/README.md) | Recherche vidéos, comptes, hashtags (Postgres plein texte) | Go | 8088 | [API](docs/api/search.openapi.yaml), [runbook](docs/runbooks/search.md) |
| [`shop`](services/shop/README.md) | Boutiques, produits, variantes, photos, stock | NestJS | 8090 | [API](docs/api/shop.openapi.yaml), [runbook](docs/runbooks/shop.md) |
| [`order`](services/order/README.md) | Panier, passage en caisse, saga et cycle de vie des commandes | NestJS | 8091 | [API](docs/api/order.openapi.yaml), [runbook](docs/runbooks/order.md) |
| `outbox-relay` | Publication de l'outbox SQL vers Redpanda, un processus par base | Go | — | [runbook](docs/runbooks/outbox-relay.md) |

Pas encore de service `payment` : seules les commandes payées à la livraison aboutissent
(voir [order](services/order/README.md#limites)).

## Démarrage rapide

```powershell
Copy-Item .env.example .env
docker compose up -d
```

Chaque service exige `APP_ENV` (`dev`, `staging` ou `prod`) et ses secrets (mot de passe Postgres,
clés S3) : aucune valeur par défaut dans le code, pour qu'un oubli ne lance jamais la production
en mode dev. Compose les fournit ; pour lancer un service seul, copier son `.env.example`.

Mise à jour d'un `.env` existant : le stockage objet exige un identifiant S3 par service
(`S3_USER_*`, `S3_VIDEO_*`, `S3_SHOP_*`, voir `.env.example` et le
[runbook](docs/runbooks/storage.md)).

Services : Postgres (une base par service), Redis, SeaweedFS, Redpanda (topics + DLQ),
les services de l'API et leurs relais outbox. ClickHouse/Qdrant :
`docker compose --profile analytics up -d`.

Les clients passent par la gateway, **`http://127.0.0.1:8000/api/v1/...`**, point d'entrée
unique de l'API ([`infra/gateway`](infra/gateway/README.md)). Les ports directs des services
(`:8081` à `:8091`) restent ouverts en local pour le débogage.

## Documentation

- [État des lieux, architecture cible et backlog](docs/architecture/00-etat-des-lieux.md)
- [ADR](docs/architecture/adr/)
- Contrats d'API (OpenAPI) et runbooks : liens dans le tableau [Services](#services)
- [Runbook outbox / DLQ](docs/runbooks/outbox-relay.md)
- [Gateway](infra/gateway/README.md), [runbook gateway](docs/runbooks/gateway.md)
- Runbooks transverses : [stockage objet](docs/runbooks/storage.md), [sauvegardes](docs/runbooks/backup.md), [staging et rollback](docs/runbooks/deploy.md)
- [Intégration Android](docs/api/ANDROID-INTEGRATION.md)
- [Sécurité](SECURITY.md)
