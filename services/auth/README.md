# Auth Service

Service d'identité de PORO : connexion par OTP SMS ou email/mot de passe, comptes à rôles
multiples, sessions par appareil, émission de JWT RS256 et publication des clés (JWKS).

- Contrat HTTP : [`docs/api/auth.openapi.yaml`](../../docs/api/auth.openapi.yaml)
- Exploitation : [`docs/runbooks/auth.md`](../../docs/runbooks/auth.md)
- Décisions : [ADR-0001 rôles](../../docs/architecture/adr/0001-roles-multiples.md),
  [ADR-0002 refresh tokens](../../docs/architecture/adr/0002-rotation-refresh-tokens.md)

## Prérequis

- Go 1.26+
- Docker (Postgres et Redis locaux, tests d'intégration, image)
- OpenSSL (génération des clés)

## Démarrage local

Depuis la racine du dépôt :

```powershell
docker compose up -d postgres redis
```

Depuis `services/auth` :

```powershell
Copy-Item .env.example .env
./scripts/generate-keys.ps1        # refuse d'écraser une paire existante (-Force pour forcer)
go run ./cmd/server
```

Au démarrage, le service se connecte à Postgres, applique les migrations, se connecte à Redis et
charge les clés. **Si l'une de ces étapes échoue, le processus s'arrête** : pas de mode dégradé.

En `dev`, les codes OTP sont écrits dans les logs (`dev sms otp`).

## Configuration

| Variable | Défaut | Description |
|---|---|---|
| `APP_ENV` | `dev` | `dev`, `staging`, `prod` |
| `APP_PORT` | `8081` | Port HTTP |
| `LOG_LEVEL` | `debug` | `debug`, `info`, `warn`, `error` |
| `TRUSTED_PROXIES` | *(vide)* | IP/CIDR des reverse proxies autorisés à fournir `X-Forwarded-For` |
| `POSTGRES_HOST` / `_PORT` / `_USER` / `_PASSWORD` / `_DB` | `localhost` / `5433` / `poro` / `poro_dev_password` / `poro_auth` | Connexion Postgres |
| `POSTGRES_SSLMODE` | `disable` | `require` ou `verify-*` obligatoire hors `dev` |
| `REDIS_HOST` / `_PORT` / `_PASSWORD` / `_DB` | `localhost` / `6380` / *(vide)* / `0` | Connexion Redis |
| `JWT_PRIVATE_KEY_PATH` / `JWT_PUBLIC_KEY_PATH` | `./keys/*.pem` | Paire RSA ≥ 2048 bits, vérifiée au démarrage |
| `JWT_ACCESS_TTL` | `15m` | Durée de l'access token |
| `JWT_REFRESH_TTL` | `720h` | Durée du refresh token (30 jours) |
| `JWT_REFRESH_REUSE_GRACE` | `30s` | Fenêtre de nouvel essai après réponse perdue (`0` désactive) |
| `OTP_TTL` | `5m` | Validité d'un code |
| `OTP_MAX_ATTEMPTS` | `3` | Essais par code |
| `OTP_HMAC_SECRET` | *(vide)* | ≥ 32 octets, obligatoire hors `dev` |
| `OTP_REQUEST_COOLDOWN` | `60s` | Délai entre deux codes pour un numéro |
| `OTP_MAX_REQUESTS_PER_HOUR` | `5` | Codes par numéro et par heure |
| `OTP_ALLOWED_CALLING_CODES` | `+225,+221,+237,+234` | Pays autorisés à recevoir un code ; vide = tous (déconseillé : fraude SMS) |
| `SMS_PROVIDER` | `log` | `log` (dev uniquement, codes dans les logs) ou `africastalking` |
| `AFRICASTALKING_USERNAME` | *(vide)* | Nom d'application ; `sandbox` = bac à sable (interdit en prod) |
| `AFRICASTALKING_API_KEY` | *(vide)* | Clé API, à fournir par le gestionnaire de secrets |
| `AFRICASTALKING_SENDER_ID` | *(vide)* | Sender ID enregistré (ex. `PORO`) ; vide = expéditeur par défaut du compte |
| `MIGRATIONS_PATH` | recherche de `migrations/` | Défini à `/app/migrations` dans l'image |

La configuration est validée au démarrage ; hors `dev`, `SMS_PROVIDER=log`, un secret OTP absent
ou Postgres sans TLS empêchent le lancement.

Pour tester l'envoi réel sans SMS réel : `SMS_PROVIDER=africastalking`,
`AFRICASTALKING_USERNAME=sandbox` et la clé du bac à sable ; les messages apparaissent dans le
simulateur Africa's Talking.

## Endpoints

| Méthode | Chemin | Auth | Limite par IP |
|---|---|---|---|
| `POST` | `/api/v1/auth/otp/request` | — | 5/min (+ 1/60 s et 5/h par numéro) |
| `POST` | `/api/v1/auth/otp/verify` | — | 20/min |
| `POST` | `/api/v1/auth/email/register` | — | 10/min |
| `POST` | `/api/v1/auth/email/login` | — | 10/min |
| `POST` | `/api/v1/auth/refresh` | — | 100/min |
| `POST` | `/api/v1/auth/logout` | Bearer | 100/min |
| `GET` | `/api/v1/auth/me` | Bearer | 100/min |
| `GET` | `/.well-known/jwks.json` | — | — |
| `GET` | `/health/live`, `/health/ready` (`/health` = ready) | — | — |

Toutes les réponses JSON suivent `{data, error, meta}` avec `meta.request_id`.

## Tests

```powershell
go test ./...                        # unitaires + intégration (Docker requis pour testcontainers)
go test $(go list ./... | Select-String -NotMatch repository)   # sans Docker
go test -race ./...                  # nécessite CGO (fait en CI)
./scripts/smoke-test.ps1 -BaseUrl http://localhost:8081 -Container poro-auth   # parcours HTTP complet
```

Le smoke test lit le code OTP dans les logs du conteneur : il ne fonctionne qu'en `dev`, et
déclenche la limite de 5 OTP/min : attendre une minute entre deux exécutions.

## Image Docker

```powershell
docker build -t poro-auth .
```

Multi-stage `golang:1.26-alpine` → `alpine:3.22`, utilisateur non-root, migrations embarquées,
`HEALTHCHECK` sur `/health/ready`. Les clés sont montées à l'exécution, jamais copiées dans l'image.
