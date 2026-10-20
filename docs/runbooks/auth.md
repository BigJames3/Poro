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
| `invalid config: APP_ENV is required …` / `POSTGRES_PASSWORD is required` | variable absente : aucune valeur par défaut, pour qu'un oubli ne lance pas la production en mode dev | fixer `APP_ENV=dev`, `staging` ou `prod` et fournir le mot de passe (compose, `.env` ou gestionnaire de secrets) |
| `invalid config: OTP_HMAC_SECRET …` | secret absent ou < 32 octets hors dev | générer : `openssl rand -base64 48`, le stocker dans le gestionnaire de secrets |
| `invalid config: SMS_PROVIDER=log …` | fournisseur de dev hors dev | configurer un fournisseur réel (non encore intégré : staging/prod bloqués) |
| `invalid config: POSTGRES_SSLMODE …` | TLS désactivé hors dev | `require` ou `verify-full` |
| `postgres …` / `migrations …` | base injoignable ou migration en échec | vérifier réseau et identifiants ; en cas de migration « dirty », voir plus bas |
| `redis …` | Redis injoignable | vérifier Redis ; le service n'accepte pas de fonctionner sans (limites de débit) |
| `revocation registry unavailable` / `REVOCATION_REDIS_URL …` | registre des sessions révoquées injoignable ou non configuré | vérifier le Redis du registre ; sans lui, auth ne démarre pas |
| `token service …` | clés absentes, < 2048 bits, ou privée et publique non appariées | remonter la bonne paire |
| `read private key: … permission denied` | le processus tourne en non-root (`app`) et ne peut pas lire la clé | Kubernetes : `securityContext.fsGroup` + `defaultMode: 0440` sur le secret ; Docker : clé lisible par l'UID de `app` |
| `invalid config: AFRICASTALKING_…` | identifiants SMS absents, ou bac à sable en prod | fournir `AFRICASTALKING_USERNAME` / `AFRICASTALKING_API_KEY` |

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
  (les access tokens expirent en 15 min au plus). L'utilisateur peut aussi le faire lui-même
  depuis l'app (`POST /sessions/revoke-others`).
- **Pic de `session_revoked`** : réutilisation de refresh tokens détectée. Soit un vol, soit un
  client qui envoie des refresh en parallèle. Vérifier la version de l'application ; le client
  doit sérialiser ses refresh (voir ADR-0002).
- **Pic de `otp_throttled` / coût SMS** : voir « Budget SMS » plus bas.
- **Pic de `login_throttled`** : comptes verrouillés après des mots de passe faux (ADR-0017). Un
  pic sur beaucoup de comptes signale une attaque par dictionnaire ; sur un seul compte, quelqu'un
  vise ce compte. Voir « Verrouillage de la connexion » plus bas.

## Emails (vérification, mot de passe oublié)

- Logs : `email code sent` (`user_id`, `purpose`), `password reset email failed`,
  `verification email after sign-up failed`. Le client reçoit `email_unavailable` (503) pour une
  vérification ; pour un mot de passe oublié il reçoit toujours `202`, l'échec n'est que dans les
  logs.
- Causes fréquentes : identifiants SMTP invalides (`auth: 535`), relais sans STARTTLS
  (`relay does not offer STARTTLS` : utiliser le port 587 ou 465 du fournisseur), certificat
  refusé (nom d'hôte `SMTP_HOST` différent du certificat), expéditeur non validé chez le
  fournisseur (`mail from: 5xx`).
- Un utilisateur ne reçoit rien : vérifier les spams, puis
  `SELECT purpose, created_at, attempts, consumed_at FROM email_codes WHERE user_id = '<uuid>' ORDER BY created_at DESC LIMIT 5;`
  et la limite `auth:email:*` dans Redis (1 email/min, 5/h par adresse).
- Délivrabilité : configurer SPF, DKIM et DMARC du domaine de `EMAIL_FROM` chez le fournisseur.

## Suppression de compte

- Comptes en attente : `SELECT id, deletion_scheduled_at FROM users WHERE deletion_scheduled_at IS NOT NULL AND deleted_at IS NULL ORDER BY deletion_scheduled_at;`
- La tâche d'effacement tourne au démarrage puis toutes les `ACCOUNT_PURGE_INTERVAL` sur chaque
  réplica. Logs : `account erased` (un par compte, `user_id`), `account purge failed` (la passe
  suivante réessaie). Un compte en retard de plus d'une heure sur sa date signale une tâche
  bloquée.
- Vérifier la publication : `SELECT published_at, last_error FROM outbox_events WHERE topic = 'poro.auth.user.deleted' AND event_key = '<uuid>';`
  (`published_at` vide : voir le runbook outbox-relay).
- Vérifier la propagation : chaque service a un groupe `poro-<service>-account-erasure`.
  Un échec part en DLQ `poro.auth.user.deleted.dlq` (alerte `EventsDeadLettered`) ; rejouer
  l'événement est sans danger (ADR-0018).
- **Annuler à la demande du support** (avant l'échéance seulement) :
  `UPDATE users SET deletion_scheduled_at = NULL WHERE id = '<uuid>' AND deleted_at IS NULL;`
  Après l'effacement, rien n'est récupérable : le compte n'a plus d'identifiant.
- Un utilisateur qui se reconnecte pendant les 30 jours annule sa suppression sans intervention.
- L'effacement supprime aussi ses codes email ; la même tâche horaire supprime les codes expirés
  depuis plus d'un jour, les codes OTP et les refresh tokens expirés (logs `expired rows deleted`,
  `table`). Un refresh token révoqué mais pas encore expiré est gardé : il sert à détecter sa
  réutilisation.

## SMS (Africa's Talking)

Les échecs d'envoi sont loggés en `error` avec le message `request failed` et la cause
`africastalking: …` ; le client reçoit seulement `sms_unavailable` (503) ou `phone_unreachable` (422).

| Cause dans les logs | Signification | Action |
|---|---|---|
| `http 401` | clé API invalide ou mauvais nom d'application | vérifier le secret et `AFRICASTALKING_USERNAME` |
| `405 InsufficientBalance` | solde épuisé : **plus aucun utilisateur ne peut se connecter par SMS** | recharger le compte ; configurer l'alerte de solde bas |
| `402 InvalidSenderId` | Sender ID non enregistré pour ce pays | enregistrer le Sender ID ou vider `AFRICASTALKING_SENDER_ID` |
| `401 RiskHold`, `407 CouldNotRoute` | blocage anti-fraude ou route indisponible vers l'opérateur | contacter le support Africa's Talking |
| `http 5xx`, timeout | panne du fournisseur | aucun renvoi automatique ; les clients peuvent réessayer tout de suite |

Chaque SMS accepté produit un log `sms otp sent` avec `message_id`, utile pour les réclamations
auprès du fournisseur. Les numéros sont masqués.

**Suspicion de SMS pumping** (hausse soudaine du volume de `otp issued` vers des numéros
consécutifs ou un seul opérateur) : le budget SMS la borne déjà (ci-dessous). Pour aller plus
loin, réduire `OTP_ALLOWED_CALLING_CODES` au pays concerné ou abaisser les plafonds, puis
redémarrer.

## Budget SMS (ADR-0017)

| Plafond | Variable | Défaut | Clé Redis |
|---|---|---|---|
| Numéro, jour UTC | `SMS_DAILY_LIMIT_PER_PHONE` | 10 | `auth:sms:phone:<numéro>:<AAAAMMJJ>` |
| Plage (numéro sans les 3 derniers chiffres), heure | `SMS_HOURLY_LIMIT_PER_PREFIX` | 20 | `auth:sms:prefix:<plage>:<AAAAMMJJHH>` |
| Indicatif pays, heure | `SMS_HOURLY_LIMIT_PER_COUNTRY` | 1 000 | `auth:sms:country:<indicatif>:<AAAAMMJJHH>` |
| Global, jour UTC | `SMS_DAILY_LIMIT` | 5 000 | `auth:sms:global:<AAAAMMJJ>` |

Métriques : `auth_sms_sent_total{country}` et `auth_sms_blocked_total{reason}` (`phone_daily`,
`prefix_hourly`, `country_hourly`, `global_daily`).

| Symptôme | Cause | Action |
|---|---|---|
| Log d'erreur `daily sms budget exhausted`, OTP en 503 `sms_unavailable` | budget global du jour atteint : attaque ou croissance réelle | regarder `auth_sms_sent_total` par pays ; attaque : laisser le disjoncteur jusqu'à minuit UTC ; croissance : relever `SMS_DAILY_LIMIT` et redémarrer |
| `auth_sms_blocked_total{reason="prefix_hourly"}` en hausse | une plage de numéros reçoit des codes en rafale (pumping typique) | rien à faire, la plage est bornée ; signaler les numéros au fournisseur |
| `country_hourly` atteint pendant un lancement marketing | trafic réel dans un pays | relever `SMS_HOURLY_LIMIT_PER_COUNTRY` |

Consulter un compteur :

```powershell
docker compose exec redis redis-cli GET auth:sms:global:$(Get-Date -AsUTC -Format yyyyMMdd)
```

## Verrouillage de la connexion (ADR-0017)

Après 5 mots de passe faux en 15 minutes pour une même adresse, quelle que soit l'IP, la
connexion est bloquée 1 minute, puis 2, 4, 8 et jusqu'à 15 minutes par échec supplémentaire
(`LOGIN_MAX_FAILURES`, `LOGIN_FAILURE_WINDOW`, `LOGIN_LOCK_BASE`, `LOGIN_LOCK_MAX`). La re-saisie
du mot de passe avant suppression du compte partage ce verrou.

- **Un utilisateur légitime est bloqué** : il peut attendre la fin du blocage, se connecter par
  OTP, ou réinitialiser son mot de passe, ce qui lève le blocage.
- **Débloquer manuellement** (support) : les clés sont l'empreinte SHA-256 de l'adresse
  normalisée (minuscules, sans espaces).

```bash
h=$(printf '%s' 'awa@poro.test' | sha256sum | cut -d' ' -f1)
docker compose exec redis redis-cli DEL auth:login:fail:$h auth:login:lock:$h
```

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

## Registre des sessions révoquées (ADR-0016)

Auth écrit une clé `poro:revoked:sid:<sid>` par session révoquée (TTL 16 min) dans
`REVOCATION_REDIS_URL`. Tous les services la lisent à chaque requête authentifiée.

| Symptôme | Cause probable | Action |
|---|---|---|
| Tous les services répondent `503 unavailable` sur les routes authentifiées | Redis du registre en panne : un jeton qu'on ne peut pas vérifier est refusé | rétablir Redis ; readiness `revocation` en échec dans chaque service |
| Un utilisateur reste connecté sur un appareil après « déconnecter les autres » | service sans registre (dev lancé seul) ou base Redis différente | vérifier `REVOCATION_REDIS_URL` du service (même hôte, même base `/15`) |
| Un utilisateur est déconnecté alors qu'il ne l'a pas demandé | session révoquée par une réutilisation de refresh token (vol présumé) | logs `refresh token reuse detected` avec le `user_id` |

Vérifier une session (sans secret) :

```powershell
docker compose exec redis redis-cli -n 15 TTL poro:revoked:sid:<sid>
```
