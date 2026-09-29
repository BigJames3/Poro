# Auth Service

Service d'authentification de Poro (OTP SMS et email, JWT RS256, refresh tokens).
Ce dépôt contient uniquement le squelette : configuration, logs et sonde de santé.
Le métier (modèles, handlers, persistance) n'est pas encore implémenté.

## Prérequis

- Go 1.22+
- Docker (build de l'image)
- PostgreSQL 16, accessible sur le port **5433** en local
- Redis 7, accessible sur le port **6380** en local

Postgres et Redis ne sont pas encore utilisés par le binaire. Les variables sont déjà chargées pour les prochaines étapes.

## Variables d'environnement

Copier le modèle puis l'adapter :

```powershell
Copy-Item .env.example .env
```

| Variable | Défaut | Description |
|---|---|---|
| `APP_ENV` | `dev` | Environnement : `dev`, `staging`, `prod` |
| `APP_PORT` | `8081` | Port HTTP |
| `LOG_LEVEL` | `debug` | Niveau zap : `debug`, `info`, `warn`, `error` |
| `POSTGRES_HOST` | `localhost` | Hôte PostgreSQL |
| `POSTGRES_PORT` | `5433` | Port PostgreSQL |
| `POSTGRES_USER` | `poro` | Utilisateur PostgreSQL |
| `POSTGRES_PASSWORD` | `poro_dev_password` | Mot de passe PostgreSQL |
| `POSTGRES_DB` | `poro_auth` | Base du service |
| `POSTGRES_SSLMODE` | `disable` | Mode SSL du client Postgres |
| `REDIS_HOST` | `localhost` | Hôte Redis |
| `REDIS_PORT` | `6380` | Port Redis |
| `REDIS_PASSWORD` | *(vide)* | Mot de passe Redis |
| `REDIS_DB` | `0` | Index de base Redis |
| `JWT_PRIVATE_KEY_PATH` | `./keys/private.pem` | Clé privée RS256 |
| `JWT_PUBLIC_KEY_PATH` | `./keys/public.pem` | Clé publique RS256 |
| `JWT_ACCESS_TTL` | `15m` | Durée de vie de l'access token |
| `JWT_REFRESH_TTL` | `720h` | Durée de vie du refresh token (30 jours) |
| `OTP_TTL` | `5m` | Durée de vie d'un code OTP |
| `OTP_MAX_ATTEMPTS` | `3` | Tentatives avant invalidation |
| `OTP_PROVIDER_URL` | *(vide)* | URL du fournisseur SMS |
| `OTP_PROVIDER_KEY` | *(vide)* | Clé du fournisseur SMS |

Les variables d'environnement du processus écrasent le fichier `.env`. L'absence de `.env` n'empêche pas le démarrage : les défauts ci-dessus s'appliquent.

## Lancement local

Depuis `services/auth` :

```powershell
go run ./cmd/server
```

Le processus écoute `http://localhost:8081`.

## Lancement Docker

Depuis `services/auth` :

```powershell
docker build -t poro-auth .
docker run --rm -p 8081:8081 --env-file .env poro-auth
```

L'image est multi-stage (`golang:1.22-alpine` puis `alpine`), le processus tourne avec l'utilisateur non-root `app`. Un `HEALTHCHECK` interroge `GET /health`.

## Endpoints

| Méthode | Chemin | Réponse |
|---|---|---|
| `GET` | `/health` | `{"status":"ok"}` |

## Tests

À venir, avec le code métier.
