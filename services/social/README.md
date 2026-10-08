# Social Service

Likes, commentaires, abonnements et partages sur les vidéos PORO (Go 1.26,
Fiber v2, pgx v5, port **8085**, base `poro_social`).

- Contrat HTTP : [`docs/api/social.openapi.yaml`](../../docs/api/social.openapi.yaml)
- Exploitation : [`docs/runbooks/social.md`](../../docs/runbooks/social.md)
- Décisions : [ADR-0008](../../docs/architecture/adr/0008-services-separes.md),
  événements [ADR-0005](../../docs/architecture/adr/0005-evenements-json-outbox.md)

## Règles métier

- Toute action exige une vidéo `ready` dans la projection, sinon `404 video_not_found`.
  Une vidéo supprimée refuse actions et lectures ; ses données sont conservées.
- Like, unlike, follow, unfollow, like de commentaire : **idempotents**. Répéter
  l'action renvoie 200, ne change rien et ne publie aucun événement.
- Les compteurs (`video_counters`, `user_counters`, `likes_count`,
  `replies_count`) changent dans la **même transaction** que l'action et que
  l'outbox. Contraintes `CHECK >= 0` en base.
- Pas d'auto-follow (`400 cannot_follow_self`, contrainte en base aussi).
- Commentaire : 1 à 1000 caractères après trim, 20 sauts de ligne au plus,
  pas de caractères de contrôle ni invisibles (bidi, zero-width). **Une seule
  profondeur** de réponse (`422 comment_reply_depth`).
- Édition : auteur seulement, 15 minutes après la publication
  (`403 comment_edit_window_closed`). Une édition qui change le texte publie
  `poro.social.comment.updated` (texte complet).
- Suppression (soft delete) : auteur ou propriétaire de la vidéo, idempotente.
  Un commentaire supprimé reste dans le fil, sans auteur ni texte, tant qu'il
  a des réponses visibles.
- Partages : non dédoublonnés, chaque partage compte. Canal `whatsapp`,
  `copy_link` ou `other`.
- Rate limit Redis par utilisateur et par minute : likes 60, commentaires 10,
  follows 30, partages 30, toutes routes 120. Au-delà : `429 rate_limited`
  avec `Retry-After`.

## Événements

| Sens | Topic | Clé (`subject`) |
|---|---|---|
| Produit | `poro.social.like.created`, `poro.social.like.deleted` | video_id |
| Produit | `poro.social.comment.created` (extrait et texte complet), `poro.social.comment.updated`, `poro.social.comment.deleted` | video_id |
| Produit | `poro.social.follow.created`, `poro.social.follow.deleted` | follower_id |
| Produit | `poro.social.share.created` | video_id |
| Consommé | `poro.video.ready`, `poro.video.deleted` → `videos_projection` | groupe `poro-social-projections` |
| Consommé | `poro.auth.user.created` → `users_projection` | groupe `poro-social-projections` |
| Consommé | `poro.moderation.content.removed` / `.restored` | groupe `poro-social-projections` |

Modération : une vidéo retirée passe à `removed` (plus aucune action ni lecture,
`404 video_not_found`) jusqu'à sa restauration ; un `poro.video.ready` tardif ne
la republie pas. Un commentaire retiré est supprimé comme par son auteur :
compteurs mis à jour et `poro.social.comment.deleted` publié. Un commentaire
retiré ne peut pas être restauré.

Contrats : [`packages/contracts/events`](../../packages/contracts/events).
Les événements produits passent par l'outbox et le relais `outbox-relay-social`.
Un appelant avec un JWT valide est aussi enregistré dans `users_projection`.

## Variables d'environnement

| Variable | Défaut | Rôle |
|---|---|---|
| `APP_ENV` | `dev` | `dev`, `staging` ou `prod` (hors dev : TLS Postgres obligatoire) |
| `APP_PORT` | `8085` | Port HTTP |
| `LOG_LEVEL` | `debug` | Niveau zap |
| `POSTGRES_HOST`, `POSTGRES_PORT`, `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB`, `POSTGRES_SSLMODE` | `localhost`, `5433`, `poro`, —, `poro_social`, `disable` | Base |
| `REDIS_HOST`, `REDIS_PORT`, `REDIS_PASSWORD`, `REDIS_DB` | `localhost`, `6380`, vide, `3` | Rate limit |
| `JWKS_URL` | `http://localhost:8081/.well-known/jwks.json` | Clés publiques d'auth |
| `KAFKA_BROKERS` | `localhost:9092` | Consommation des projections |
| `TRUSTED_PROXIES` | vide | Proxies autorisés à poser `X-Forwarded-For` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | vide | Traces OpenTelemetry (désactivées si vide) |
| `MIGRATIONS_PATH` | `./migrations` (image : `/app/migrations`) | Migrations appliquées au démarrage |

## Démarrage local

```powershell
docker compose up -d social outbox-relay-social
```

Depuis `services/social` (processus hôte, socle déjà démarré) :

```powershell
Copy-Item .env.example .env
go run ./cmd/server
```

## Tests

```powershell
go test ./... -count=1 -race -coverpkg=./internal/... -coverprofile=coverage.out
go tool cover -func=coverage.out
golangci-lint run ./...
```

Les tests d'intégration démarrent Postgres et Redpanda avec testcontainers
(Docker requis) ; sans Docker ils sont ignorés.

## Limites connues

- **Projections limitées à la rétention Kafka (7 jours).** Une vidéo devenue
  `ready` ou un compte créé avant le premier démarrage de social, et absents
  des topics, sont inconnus : `404`. Un compte se rattrape dès qu'il appelle
  social avec un JWT. Un backfill depuis video et auth reste à faire.
- **Les listes renvoient des identifiants**, pas des profils : le client
  résout noms et avatars auprès de user (pas encore d'API de lecture par lot).
- Pas d'événement à l'édition d'un commentaire ni sur les likes de commentaire.
- Pas de modération : un commentaire n'est filtré que sur sa forme.
- Le rate limit est une fenêtre fixe d'une minute (limiter Fiber).
