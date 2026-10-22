# User Service

Profils publics PORO : username unique, bio, avatar (upload présigné +
ré-encodage serveur) et activation créateur. L'identité (téléphone, email,
rôles, sessions) reste dans auth.

- Contrat HTTP : [`docs/api/user.openapi.yaml`](../../docs/api/user.openapi.yaml)
- Exploitation : [`docs/runbooks/user.md`](../../docs/runbooks/user.md)
- Décision : [ADR-0006](../../docs/architecture/adr/0006-service-user.md)
- Événements : [ADR-0005](../../docs/architecture/adr/0005-evenements-json-outbox.md)
  - consommé : `poro.auth.user.created` (groupe `poro-user-profiles`)
  - consommé : `poro.auth.user.deleted` (groupe `poro-user-account-erasure`,
    [ADR-0018](../../docs/architecture/adr/0018-effacement-du-compte-dans-les-services.md)) :
    profil anonymisé (id et `deleted_at` seuls), pseudo libéré, avatars supprimés du stockage
  - produits : `poro.user.creator.activated`, `poro.user.profile.updated`
    (contrats dans [`packages/contracts/events`](../../packages/contracts/events))

## Prérequis

- Node 24+
- Docker (Postgres, Redis, Redpanda, SeaweedFS, tests testcontainers)

## Démarrage local

Depuis la racine :

```powershell
docker compose up -d postgres redis redpanda seaweedfs postgres-init redpanda-init seaweedfs-init
docker compose up -d user-migrate user
```

Ou tout le socle (auth + user + relais) :

```powershell
docker compose up -d
```

Depuis `services/user` (processus hôte, Postgres/Redpanda déjà up) :

```powershell
Copy-Item .env.example .env
npm ci
npx prisma migrate deploy
npm run start:dev
```

## Configuration

Voir `.env.example`. Les originaux d'avatars vont dans le bucket privé
`S3_UPLOAD_BUCKET` (`poro-avatar-uploads`), l'avatar ré-encodé dans `S3_BUCKET`
(`poro-avatars`, public) ; identité S3 `poro-user`
([ADR-0015](../../docs/architecture/adr/0015-controle-acces-stockage-objet.md)).
Hors `dev` : TLS Postgres (`sslmode=require` ou `verify-*`),
`REDIS_URL`, HTTPS sur `S3_PUBLIC_ENDPOINT`, CORS en origines https exactes,
clés S3 obligatoires.

## Endpoints

| Méthode | Chemin | Auth |
|---|---|---|
| `GET` | `/api/v1/users/me` | Bearer |
| `PATCH` | `/api/v1/users/me` | Bearer |
| `GET` | `/api/v1/users/username-availability?username=` | Bearer |
| `GET` | `/api/v1/users?ids=a,b,c` (1 à 100 comptes, profils publics en un appel) | — |
| `GET` | `/api/v1/users/by-username/:username` | — |
| `POST` | `/api/v1/users/me/avatar/upload-url` | Bearer |
| `PUT` | `/api/v1/users/me/avatar` | Bearer |
| `DELETE` | `/api/v1/users/me/avatar` | Bearer |
| `POST` | `/api/v1/users/me/creator` | Bearer |
| `GET` | `/health/live`, `/health/ready`, `/metrics` | — |

`POST /me/creator` exige un username. Le rôle CREATOR apparaît dans le JWT
après un **refresh**, pas sur l'access token déjà émis.

## Tests

```powershell
npm run lint
npm run typecheck
npm run test:cov
```

Les tests d'intégration démarrent Postgres et Redpanda via testcontainers.
Couverture minimale 70 %.

## Sessions révoquées

Les access tokens des sessions révoquées par auth (déconnexion, « déconnecter les autres
appareils », réinitialisation du mot de passe, suppression du compte, vol de refresh token) sont
refusés grâce au registre partagé `REVOCATION_REDIS_URL`
([ADR-0016](../../docs/architecture/adr/0016-revocation-des-sessions.md)) : obligatoire hors dev,
réponse `503` s'il ne répond pas.
