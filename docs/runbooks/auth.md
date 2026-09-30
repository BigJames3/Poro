# Runbook — service auth

## Sondes

| Sonde | Usage | Échec signifie |
|---|---|---|
| `GET /health/live` | liveness Kubernetes | processus bloqué : redémarrer |
| `GET /health/ready` | readiness, `HEALTHCHECK` Docker | Postgres ou Redis injoignable (`checks` indique lequel) : l'instance ne reçoit plus de trafic |

## Le service ne démarre pas

Le démarrage est volontairement strict. Lire la première ligne `fatal` des logs :

| Message | Cause | Action |
|---|---|---|
| `invalid config: OTP_HMAC_SECRET …` | secret absent ou < 32 octets hors dev | générer : `openssl rand -base64 48`, le stocker dans le gestionnaire de secrets |
| `invalid config: SMS_PROVIDER=log …` | fournisseur de dev hors dev | configurer un fournisseur réel (non encore intégré : staging/prod bloqués) |
| `invalid config: POSTGRES_SSLMODE …` | TLS désactivé hors dev | `require` ou `verify-full` |
| `postgres …` / `migrations …` | base injoignable ou migration en échec | vérifier réseau et identifiants ; en cas de migration « dirty », voir plus bas |
| `redis …` | Redis injoignable | vérifier Redis ; le service n'accepte pas de fonctionner sans (blacklist et limites) |
| `token service …` | clés absentes, < 2048 bits, ou privée et publique non appariées | remonter la bonne paire |

## Clés JWT

- Génération : `scripts/generate-keys.ps1` (Windows) ou `scripts/generate-keys.sh`. Les scripts
  refusent d'écraser une paire existante sans `-Force` / `--force`.
- Les clés ne sont **jamais** commitées (`keys/` est ignoré) ni copiées dans l'image.
- Le `kid` est l'empreinte RFC 7638 de la clé publique ; les autres services vérifient les
  tokens via `/.well-known/jwks.json` (cache 5 minutes).

**Rotation (procédure actuelle, avec coupure)** : remplacer la paire et redémarrer toutes les
instances. Les access tokens en cours (≤ 15 min) deviennent invalides et les clients se
réauthentifient via refresh (les refresh tokens sont opaques et restent valides).
Une rotation sans coupure (publication de deux clés dans le JWKS) est prévue au backlog.

**Compromission de la clé privée** : générer une nouvelle paire, redéployer, puis révoquer
toutes les sessions :

```sql
UPDATE refresh_tokens SET revoked_at = now() WHERE revoked_at IS NULL;
```

## Incidents de sécurité courants

- **Compte compromis** : révoquer ses sessions
  `UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = '<uuid>' AND revoked_at IS NULL;`
  (les access tokens expirent en 15 min au plus).
- **Pic de `session_revoked`** : réutilisation de refresh tokens détectée. Soit un vol, soit un
  client qui envoie des refresh en parallèle. Vérifier la version de l'application ; le client
  doit sérialiser ses refresh (voir ADR-0002).
- **Pic de `otp_throttled` / coût SMS** : ajuster `OTP_REQUEST_COOLDOWN` et
  `OTP_MAX_REQUESTS_PER_HOUR` ; les compteurs sont dans Redis (`auth:otp:*`).

## Rate limiting derrière un proxy

Sans `TRUSTED_PROXIES`, `X-Forwarded-For` est ignoré : derrière un load balancer, tous les
clients partagent alors la même IP et atteignent vite les limites. Renseigner les CIDR du
proxy (ex. `10.0.0.0/8`) ; ne jamais mettre `0.0.0.0/0`.

## Migrations

- Appliquées automatiquement au démarrage, dans une transaction par fichier.
- État : `SELECT version, dirty FROM schema_migrations;`
- Migration « dirty » : corriger la cause, vérifier manuellement l'état du schéma, puis
  `UPDATE schema_migrations SET dirty = false, version = <dernière version complète>;`
- `000004` (rôles multiples) a une migration descendante **avec perte** : ne pas redescendre
  en production sans sauvegarde.

## Corrélation

Chaque réponse porte `X-Request-ID` (fourni par le client ou généré) et `meta.request_id`.
Chercher cette valeur dans les logs (`request_id`) pour retrouver l'access log et l'erreur interne.
Les numéros de téléphone sont masqués dans les logs, sauf la ligne `dev sms otp` en dev.
