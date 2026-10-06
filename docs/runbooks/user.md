# Runbook — service user

Profils publics, usernames, avatars, activation créateur. Port 8082.
Contrat : [`docs/api/user.openapi.yaml`](../api/user.openapi.yaml).
Décision : [ADR-0006](../architecture/adr/0006-service-user.md).

## Santé

| Probe | Sens |
|---|---|
| `GET /health/live` | Processus HTTP up (pas de dépendance) |
| `GET /health/ready` | Postgres (+ Redis si `REDIS_URL`). **Pas Kafka** |
| `GET /metrics` | Prometheus ; ne pas exposer sur l'ingress public |

Un courtier down : les profils se servent ; les événements s'accumulent en outbox
et le consommateur `poro-user-profiles` réessaie.

## Profil manquant après inscription

1. Compte auth créé ? `GET /api/v1/auth/me` avec l'access token.
2. Outbox auth : `SELECT * FROM outbox_events WHERE event_key = '<user_id>';`
   dans `poro_auth`. `published_at` null → relais auth (runbook outbox).
3. Topic : `rpk topic consume poro.auth.user.created --brokers localhost:9092`.
4. Inbox user : `SELECT * FROM processed_events WHERE consumer = 'poro-user-profiles';`
5. Repli immédiat pour l'utilisateur : `GET /api/v1/users/me` crée le profil.

## Username « taken » à tort

L'unicité couvre aussi les profils `deleted_at IS NOT NULL` : un handle n'est
jamais réattribué. C'est voulu. Un support ne doit pas DELETE la ligne.

## Avatar : confirm échoue (`avatar_not_uploaded`)

Le client n'a pas POST le fichier vers `upload_url` (ou trop tard : 300 s).
Redemander `POST /me/avatar/upload-url`. `avatar_invalid` : pas une vraie image
jpeg/png/webp (ex. HTML). `avatar_upload_invalid` : `upload_key` d'un autre compte.

## Créateur sans rôle CREATOR dans le JWT

Normal tant que l'access token n'a pas été **rafraîchi**. Vérifier :

```sql
-- poro_user
SELECT is_creator, username FROM profiles WHERE user_id = '<id>';
SELECT * FROM outbox_events WHERE topic = 'poro.user.creator.activated' AND event_key = '<id>';
-- poro_auth
SELECT role FROM user_roles WHERE user_id = '<id>';
```

Si l'outbox user n'est pas publiée → relais user. Si publiée mais pas de rôle →
consommateur auth `poro-auth-creator-roles` / DLQ `poro.user.creator.activated.dlq`.

## Migrations

Image `user-migrate` : `prisma migrate deploy` sur `poro_user`. Un volume Postgres
existant n'a pas `poro_user` : le job `postgres-init` fait `CREATE DATABASE` de
façon idempotente.
