# Search Service

Recherche PORO : vidéos, comptes et hashtags, en Postgres plein texte
([ADR-0011](../../docs/architecture/adr/0011-search-postgres-v1.md)). Le service
ne possède aucune donnée : il projette les événements des autres services.

- Contrat HTTP : [`docs/api/search.openapi.yaml`](../../docs/api/search.openapi.yaml)
- Exploitation : [`docs/runbooks/search.md`](../../docs/runbooks/search.md)

## Événements consommés

Groupe `poro-search-indexer` (inbox `processed_events`, DLQ). Le service ne publie rien.

| Topic | Effet |
|---|---|
| `poro.video.ready` | Vidéo indexée (titre, description, hashtags, miniature) |
| `poro.video.deleted` | Vidéo retirée des résultats pour de bon ; une tombe empêche un `ready` tardif de la ramener |
| `poro.moderation.content.removed` / `.restored` | Vidéo masquée puis de nouveau cherchable (les commentaires sont ignorés) |
| `poro.auth.user.created`, `poro.user.profile.updated` | Comptes ; un compte devient cherchable avec un username ; le snapshot le plus récent gagne |
| `poro.social.like.*`, `poro.social.comment.*`, `poro.social.share.created` | Compteurs de classement (jamais négatifs) |
| `poro.social.follow.*` | Nombre d'abonnés des comptes |

`hashtags.videos_count` suit en permanence le nombre de vidéos cherchables par tag.

## API

| Méthode | Chemin | Rôle |
|---|---|---|
| `GET` | `/api/v1/search?q=&type=videos\|users\|hashtags&cursor=&limit=` | Résultats classés ; `type` par défaut `videos` ; `limit` 1–50 (20) ; 500 résultats au plus |
| `GET` | `/api/v1/search/suggest?q=` | Autocomplétion : 5 comptes et 5 hashtags par préfixe (`@` et `#` ignorés) |
| `GET` | `/health`, `/health/live`, `/health/ready` (aussi sous `/api/v1`), `/metrics` | Sondes et métriques |

- Jeton facultatif ; 60 requêtes par minute par compte, sinon par IP (`429 rate_limited`).
- `q` : 1 à 100 caractères sans caractère de contrôle (`400 invalid_query`) ;
  `invalid_type`, `invalid_cursor` sinon.
- Insensible aux accents et à la casse ; racines françaises (« danses » trouve
  « danse ») ; tolère une faute de frappe dans un mot du titre ou du nom.

## Variables d'environnement

| Variable | Défaut | Rôle |
|---|---|---|
| `APP_ENV` | `dev` | `dev`, `staging` ou `prod` (hors dev : TLS Postgres obligatoire) |
| `APP_PORT` | `8088` | Port HTTP |
| `LOG_LEVEL` | `debug` | Niveau zap |
| `POSTGRES_*` | `localhost`, `5433`, `poro`, —, `poro_search`, `disable` | Base (extensions `unaccent` et `pg_trgm` créées par la migration) |
| `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`, `REDIS_DB` | `localhost`, `6380`, vide, `7` | Limite de débit |
| `JWKS_URL` | `http://localhost:8081/.well-known/jwks.json` | Clés publiques d'auth |
| `KAFKA_BROKERS` | `localhost:9092` | Indexeur |
| `S3_PUBLIC_ENDPOINT`, `S3_BUCKET` | `http://localhost:9000`, `poro-videos` | URL des miniatures |
| `TRUSTED_PROXIES`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `MIGRATIONS_PATH` | | Comme les autres services Go |

## Démarrage local

```powershell
docker compose up -d search
```

## Tests

```powershell
go test ./... -count=1 -race -coverpkg=./internal/... -coverprofile=coverage.out
```

Postgres et Redpanda via testcontainers. Couverture minimale 70 %.

## Limites

- Pagination par décalage : un classement qui bouge entre deux pages peut
  répéter ou sauter un résultat ; 500 résultats au plus.
- Pas de recherche dans les commentaires ; pas de synonymes ni de langues locales.
- Un compte sans username n'apparaît pas.
- La rétention Kafka de 7 jours empêche un rattrapage d'historique plus ancien.
