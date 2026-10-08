# Notification Service

Notifications PORO : in-app et push FCM. Email et SMS sont hors périmètre.
Le service ne publie aucun événement : il consomme ceux de social, video,
auth et user, et sert la boîte de notifications de chaque compte.

- Contrat HTTP : [`docs/api/notification.openapi.yaml`](../../docs/api/notification.openapi.yaml)
- Exploitation : [`docs/runbooks/notification.md`](../../docs/runbooks/notification.md)
- Événements : [ADR-0005](../../docs/architecture/adr/0005-evenements-json-outbox.md),
  contrats dans [`packages/contracts/events`](../../packages/contracts/events)

## Événements consommés

Groupe `poro-notification-dispatcher`. La notification et la ligne d'inbox
(`inbox_events`) sont écrites dans la même transaction ; un événement rejoué
n'a aucun effet. Un événement invalide part en DLQ (`<topic>.dlq`).

| Événement | Destinataire | Effet |
|---|---|---|
| `poro.social.like.created` | `video_owner_id` | Regroupé par vidéo et par heure UTC du like : « Awa et 12 autres personnes ont aimé ta vidéo ». Un nouvel acteur rend la notification non lue et la remonte ; un acteur déjà compté ne change rien. Un seul push, à la création du groupe. |
| `poro.social.comment.created` | `video_owner_id`, et `parent_author_id` pour une réponse | Réponse : « a répondu à ton commentaire ». Si le propriétaire est l'auteur du parent, il reçoit seulement la réponse. |
| `poro.social.follow.created` | `following_id` | Une fois par abonné, même après désabonnement puis réabonnement. |
| `poro.video.ready` | `user_id` (l'auteur) | « Ta vidéo est en ligne ». |
| `poro.social.comment.deleted` | — | Supprime les notifications de ce commentaire. |
| `poro.video.deleted` | — | Supprime les notifications de cette vidéo. |
| `poro.moderation.content.removed` | — | Supprime les notifications de la vidéo ou du commentaire retiré ; une restauration ne les recrée pas. |
| `poro.auth.user.created`, `poro.user.profile.updated` | — | Projection `user_projections` (nom, username, avatar des acteurs) ; le snapshot le plus récent gagne. |

Règles :

- l'acteur n'est jamais notifié de sa propre action ;
- un type désactivé dans les préférences ne crée ni notification ni push ;
  `push_enabled: false` garde l'in-app et coupe le push ;
- le texte est rendu à l'écriture depuis la projection (« Quelqu'un » si le
  profil n'est pas encore arrivé). Tous les textes français sont dans
  [`src/notifications/messages.fr.ts`](src/notifications/messages.fr.ts).

## Push FCM

- Envoyé **après le commit**, hors transaction, à tous les appareils du
  destinataire (`sendEachForMulticast`). Son échec est journalisé et compté
  (`notification_pushes_total{outcome="error"}`), sans retenter l'événement
  ni toucher à l'in-app.
- Les tokens que FCM déclare définitivement invalides
  (`registration-token-not-registered`, `invalid-registration-token`) sont
  supprimés. Les autres erreurs (quota, indisponibilité, payload) gardent le token.
- Le push porte `data` : `notification_id`, `type` et les identifiants de la
  cible (`video_id`, `comment_id`, `parent_id`, `user_id`).
- Sans `FCM_CREDENTIALS_B64`, le push est désactivé (avertissement au démarrage) ;
  l'in-app fonctionne.

## Démarrage local

Depuis la racine :

```powershell
docker compose up -d postgres redis redpanda postgres-init redpanda-init auth
docker compose up -d notification-migrate notification
```

Depuis `services/notification` (processus hôte, Postgres/Redpanda déjà up) :

```powershell
Copy-Item .env.example .env
npm ci
npx prisma migrate deploy
npm run start:dev
```

## Configuration

Voir `.env.example`.

| Variable | Défaut | Rôle |
|---|---|---|
| `PORT` | `8086` | Port HTTP |
| `DATABASE_URL` | — | `poro_notification` ; hors `dev`, `sslmode=require` ou `verify-*` |
| `REDIS_URL` | — | Compteurs de limite de débit (DB 5) ; obligatoire hors `dev` |
| `JWKS_URL` | `http://localhost:8081/.well-known/jwks.json` | Clés publiques d'auth |
| `KAFKA_BROKERS` | `localhost:9092` | Brokers Redpanda/Kafka |
| `KAFKA_CONSUMER_ENABLED` | `true` | `false` : HTTP seul |
| `FCM_CREDENTIALS_B64` | vide | JSON du compte de service Firebase en base64 ; vide = push désactivé |
| `NOTIFICATION_RETENTION_DAYS` | `90` | Purge des notifications sans activité depuis N jours (7 à 3650) |
| `NOTIFICATION_PURGE_ENABLED` | `true` | Purge quotidienne (première passe une minute après le démarrage) |
| `CORS_ORIGINS`, `TRUST_PROXY`, `OTEL_EXPORTER_OTLP_ENDPOINT`, `LOG_LEVEL`, `APP_ENV` | | Comme les autres services NestJS |

Encoder les identifiants Firebase (PowerShell) :

```powershell
[Convert]::ToBase64String([IO.File]::ReadAllBytes("service-account.json"))
```

## Endpoints

Tous sous `/api/v1/notifications`, Bearer obligatoire.

| Méthode | Chemin | Limite |
|---|---|---|
| `GET` | `/` (`cursor`, `limit` 1–50, défaut 20) | 120/min |
| `GET` | `/unread-count` | 120/min |
| `PATCH` | `/:id/read` | 120/min |
| `POST` | `/read-all` | 120/min |
| `PUT` | `/devices` | 10/min |
| `DELETE` | `/devices/:token` | 120/min |
| `GET`, `PATCH` | `/preferences` | 120/min |
| `GET` | `/health`, `/health/live`, `/health/ready`, `/api/v1/health`, `/metrics` | — |

- La liste est triée par dernière activité : un groupe de likes qui reçoit un
  nouvel acteur remonte en tête. Marquer comme lu ne réordonne rien.
- Un token FCM appartient à un seul compte : l'enregistrer depuis un autre
  compte le déplace. Un compte garde ses 10 appareils les plus récents.
- `DELETE /devices/:token` répond 204 même si le token n'est pas le sien.

## Tests

```powershell
npm run format:check
npm run lint
npm run typecheck
npm run test:cov
```

Les tests d'intégration démarrent Postgres et Redpanda via testcontainers ;
FCM est remplacé par un faux émetteur. Couverture minimale 70 %.

## Limites

- **Push au plus une fois** : un arrêt entre le commit et l'envoi perd le push
  (l'in-app reste). Pas de file de push ni de nouvel essai.
- **Textes figés à l'écriture** : un changement de nom de l'acteur ne modifie pas
  les notifications existantes (l'objet `actor` de la liste, lui, est à jour).
- **Pas de rattrapage** : la rétention Kafka est de 7 jours ; un compte créé
  avant le déploiement reste « Quelqu'un » jusqu'à sa prochaine mise à jour de profil.
- **Un like retiré n'est pas retiré** de la notification ; un désabonnement
  non plus.
- **Pagination** : une notification qui remonte en tête pendant qu'on pagine
  peut apparaître deux fois ou être sautée sur la page suivante.
- **Tokens FCM** : firebase-admin 14 annonce l'abandon des tokens
  d'enregistrement au profit des identifiants d'installation (FID). Le passage
  aux FID demande d'abord une évolution de l'application Android.
- **Dépendances** : `npm audit` signale `uuid` (transitive via firebase-admin,
  appels `v4` non concernés) et `deepmerge-ts`/`mysql2` (CLI Prisma, comme
  dans user) ; aucune n'est appelée sur un chemin exploitable du service.
- Pas d'email, de SMS, d'APNs direct ni de badge iOS (hors périmètre).
