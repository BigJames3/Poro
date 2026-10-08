# Feed Service

Flux abonnements, tendances et For You de PORO (Go 1.26, Fiber v2, pgx v5,
port **8084**, base `poro_feed`, Redis DB 4).

- Contrat HTTP : [`docs/api/feed.openapi.yaml`](../../docs/api/feed.openapi.yaml)
- Exploitation : [`docs/runbooks/feed.md`](../../docs/runbooks/feed.md)
- Décisions : [ADR-0009](../../docs/architecture/adr/0009-feed-v1.md) (algorithmes,
  seuil de bascule du fan-out), [ADR-0008](../../docs/architecture/adr/0008-services-separes.md)

## Endpoints (JWT obligatoire)

| Route | Contenu |
|---|---|
| `GET /api/v1/feed/following` | Vidéos des comptes suivis, plus récentes d'abord (page 1 en cache 60 s par utilisateur) |
| `GET /api/v1/feed/trending` | Score `(likes + 2·comments + 3·shares) / (âge_h + 2)^1.5` sur 72 h (page 1 en cache 60 s) |
| `GET /api/v1/feed/for-you` | Session de 200 vidéos : 60 % abonnements, 30 % tendances, 10 % découverte ; démarrage à froid 70/30 |

`?cursor=` (opaque) et `?limit=` (10 par défaut, 30 au plus). Sondes :
`/health`, `/health/live`, `/health/ready` (aussi sous `/api/v1`), `/metrics`.

Chaque élément : `video_id`, `title`, `hashtags`, `duration_ms`,
`published_at`, `thumbnail_url`, `hls_url`, `stats` et `author` (nom et avatar).

## Événements consommés

Groupe `poro-feed-projector` (inbox, DLQ). Le feed ne publie rien.

| Topic | Effet |
|---|---|
| `poro.video.ready` | Vidéo projetée (titre, hashtags, clés média, `published_at`), score calculé |
| `poro.video.deleted` | Vidéo retirée de tous les flux ; une tombe empêche un `ready` tardif de la ressusciter |
| `poro.social.follow.created` / `.deleted` | Abonnements et nombre d'abonnés par auteur |
| `poro.social.like.*`, `poro.social.comment.*`, `poro.social.share.created` | Compteurs et score de tendance |
| `poro.user.profile.updated` | Nom, nom affiché et avatar des auteurs (instantané le plus récent) |
| `poro.moderation.content.removed` (vidéo) | `moderation_status = 'rejected'` : vidéo masquée de tous les flux, score à zéro ; un `ready` tardif la laisse masquée |
| `poro.moderation.content.restored` | Vidéo de nouveau `approved` et rescorée (sauf si supprimée entre-temps) |

Les commentaires retirés arrivent par `poro.social.comment.deleted`. La page 1
en cache (60 s) peut encore montrer une vidéo retirée pendant une minute ;
les sessions For You la filtrent à l'hydratation.

Un ticker de 5 minutes recalcule la décroissance des scores (arrêt propre avec
le service).

## Variables d'environnement

| Variable | Défaut | Rôle |
|---|---|---|
| `APP_ENV` | `dev` | `dev`, `staging` ou `prod` (hors dev : TLS Postgres obligatoire) |
| `APP_PORT` | `8084` | Port HTTP |
| `LOG_LEVEL` | `debug` | Niveau zap |
| `POSTGRES_*` | `localhost`, `5433`, `poro`, —, `poro_feed`, `disable` | Base |
| `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`, `REDIS_DB` | `localhost`, `6380`, vide, `4` | Sessions For You, cache, rate limit |
| `JWKS_URL` | `http://localhost:8081/.well-known/jwks.json` | Clés publiques d'auth |
| `KAFKA_BROKERS` | `localhost:9092` | Projections |
| `S3_PUBLIC_ENDPOINT`, `S3_BUCKET` | `http://localhost:9000`, `poro-videos` | URL publiques des miniatures et playlists HLS |
| `TRUSTED_PROXIES`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `MIGRATIONS_PATH` | vides, `./migrations` | Comme les autres services |

## Démarrage et tests

```powershell
docker compose up -d feed
go test ./... -count=1 -race -coverpkg=./internal/... -coverprofile=coverage.out
golangci-lint run ./...
```

Les tests d'intégration démarrent Postgres et Redpanda (testcontainers) et
utilisent miniredis.

## Limites connues

- **Pas de watch time ni de vues** : aucun événement de visionnage n'existe ;
  le score de tendance ne repose que sur likes, commentaires et partages.
- **`liked_by_me` absent du feed v1** : le client le lit dans les stats de social.
- **Pas de `/nearby`** : aucune donnée de localisation.
- **Projections limitées à la rétention Kafka (7 jours)** : les vidéos, abonnements
  et profils antérieurs au premier démarrage du feed et absents des topics ne
  sont pas projetés. Un backfill reste à faire.
- **Compteurs approximatifs** : projection de social, quelques secondes de retard,
  bornés à zéro si un événement manque.
- **Pagination trending** : les scores évoluent entre deux pages ; une vidéo peut
  rarement monter ou descendre d'une page à l'autre. For You n'a pas ce problème
  (session figée).
- Modération : toutes les vidéos sont `approved` tant que le service de modération
  n'existe pas.
