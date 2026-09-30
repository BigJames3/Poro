# PORO — État des lieux technique et backlog

Date de l'analyse : 29 septembre 2026. Ce document est la référence de départ ;
il est mis à jour à chaque jalon P0.

## 1. Architecture actuelle (constatée)

| Élément | État réel |
|---|---|
| `services/auth` (Go 1.26, Fiber v2, pgx v5, Redis) | Seul service existant. OTP SMS, email/mot de passe, JWT RS256, refresh tokens, migrations golang-migrate, tests unitaires et d'intégration (testcontainers). |
| Autres services (`user`, `video`, `feed`, `social`, `shop`, …) | Aucun code. Seulement listés dans `.cursor/rules/project.md`. |
| `packages/`, `infra/`, `data/`, `docs/` | Vides avant ce jalon. |
| `apps/mobile-android` | Projet Android Studio (36 fichiers suivis), ne consomme encore aucune API. Non modifié. |
| `docker-compose.yml` | Postgres 16, Redis 7, SeaweedFS (S3), Redpanda, ClickHouse, Qdrant, service auth. |
| CI (`.github/workflows/ci.yml`) | Avant ce jalon : jobs `echo` uniquement, aucun test exécuté. |
| iOS, Terraform, Helm, k8s, observabilité | Inexistants. |

## 2. Problèmes détectés et corrigés dans ce jalon (P0-1)

| # | Problème | Risque | Correction |
|---|---|---|---|
| 1 | Rôle unique (`users.role` enum) | Impossible d'être PERSONAL + CREATOR + BUSINESS, exigence produit explicite | Table `user_roles` (N rôles), JWT `roles: []` — [ADR-0001](adr/0001-roles-multiples.md) |
| 2 | Rotation du refresh token en deux requêtes sans transaction | Deux requêtes concurrentes produisaient deux sessions valides ; aucune détection de vol | Transaction + `SELECT … FOR UPDATE`, familles de session, détection de réutilisation, fenêtre de grâce réseau — [ADR-0002](adr/0002-rotation-refresh-tokens.md) |
| 3 | OTP : lecture puis incrément des tentatives | Des requêtes parallèles contournaient la limite de 3 essais | Incrément conditionnel atomique (`UPDATE … WHERE attempts < max RETURNING`) |
| 4 | OTP : aucune limite par numéro | Brute force en changeant d'IP, coût SMS non maîtrisé | Cooldown 60 s + 5 codes/heure par numéro (Redis), `Retry-After` |
| 5 | OTP et mots de passe hachés avec Argon2id 64 MiB | ~6 Go de RAM pour 100 connexions simultanées : DoS facile sur petits serveurs | OTP : HMAC-SHA256 avec secret serveur ; mots de passe : paramètres OWASP (19 MiB, t=2, p=1) |
| 6 | Démarrage « dégradé » si Postgres/Redis/clés absents, `/health` toujours 200 | Le service recevait du trafic qu'il ne pouvait pas servir | Fail-fast au démarrage, `/health/live` et `/health/ready` (503) |
| 7 | `pgx` v5.7.4 : GO-2026-5004 (injection SQL), plus 3 autres CVE atteignables | Sécurité | Dépendances mises à jour, `govulncheck` : 0 vulnérabilité atteignable |
| 8 | Image Docker sans les migrations (montées seulement par compose) | Déploiement k8s impossible | Migrations embarquées dans l'image, `MIGRATIONS_PATH` |
| 9 | Aucune corrélation des requêtes, erreurs 4xx loggées en `error` | Observabilité | `X-Request-ID`, access log structuré, `meta.request_id`, codes d'erreur stables |
| 10 | Email non normalisé | Doublons `Ada@x` / `ada@x` | Normalisation + contrainte `CHECK` en base |
| 11 | `c.IP()` sans configuration de proxy | Rate limit par IP inefficace derrière un reverse proxy, ou contournable via `X-Forwarded-For` | `TRUSTED_PROXIES` explicite |
| 12 | Blacklist JWT indexée par le token complet, pas de `jti`/`aud`/`kid` | Autres services incapables de vérifier les tokens | Claims `jti`, `aud=poro-api`, `sid`, en-tête `kid`, endpoint JWKS |
| 13 | Login : statut du compte révélé avant vérification du mot de passe, timing différent si email inconnu | Énumération de comptes | Vérification du mot de passe d'abord, comparaison factice |
| 14 | Scripts de génération de clés écrasant la paire existante | Déconnexion de tous les utilisateurs par erreur | Refus sans `-Force` |
| 15 | CI factice | Régressions non détectées | Job `auth` : gofmt, vet, tests `-race`, govulncheck, build Docker |

## 3. Risques et dette restants

**Sécurité**
- Aucun fournisseur SMS réel : le service refuse volontairement de démarrer hors `dev` (`SMS_PROVIDER=log` interdit). Choix du fournisseur à faire (décision coût/contrat).
- Clés JWT sur disque. En production : secret Kubernetes chiffré (SOPS/Sealed Secrets) ou Vault, rotation via plusieurs `kid` dans le JWKS.
- Pas encore de scan de conteneur (Trivy), de SAST (golangci-lint/gosec), de DAST, de signature d'image.

**Architecture**
- 13 microservices prévus pour une petite équipe, en 3 langages : coût d'exploitation et de CI élevé. Proposition de regroupement : [ADR-0003](adr/0003-frontieres-services.md).
- Aucun contrat d'événements, aucun pattern outbox : publier sur Kafka depuis le code métier sans outbox perdrait des événements.

**Infrastructure**
- Images `latest` (SeaweedFS, Redpanda, ClickHouse, Qdrant) : builds non reproductibles. À épingler par digest.
- Les règles Cursor citent MinIO, le compose utilise SeaweedFS. SeaweedFS est conservé (compatible S3, plus léger) ; les règles devront être alignées.
- Pas de sauvegarde Postgres, pas d'environnement staging.

**Incohérences documentaires**
- `.cursor/rules/project.md` : « demander confirmation avant de coder » et « marketplace hors MVP » contredisent le prompt maître (autonomie, marketplace centrale). Décision produit à prendre (voir §6).
- `README.md` racine mêlait deux encodages ; réécrit.

## 4. Architecture cible

```
Mobile (Android Compose / iOS SwiftUI)
        │ HTTPS, JWT
Cloudflare (DNS, WAF, cache) ── Bunny CDN (HLS, images)
        │
Ingress k3s (Traefik) ── /api/v1/{domaine}
        │
 ┌──────┴──────────────────────────────────────────────┐
 │ auth (Go)        identité, rôles, sessions, JWKS    │
 │ user (NestJS)    profils, onboarding créateur/vendeur│
 │ content (Go)     vidéos, social, feed               │
 │ media-worker (Go+FFmpeg) transcodage HLS            │
 │ commerce (NestJS) boutiques, produits, commandes,   │
 │                   paiements, livraisons (modules)   │
 │ notification     FCM / APNs                         │
 │ moderation, reco, analytics (Python) — plus tard    │
 └──────┬──────────────────────────────────────────────┘
        │ outbox → Redpanda/Kafka (poro.{domaine}.{entité}.{action})
PostgreSQL (une base par service) · Redis · S3 (SeaweedFS → Backblaze B2)
ClickHouse / Qdrant quand les volumes le justifient
```

Chaque service vérifie les JWT localement via `GET /.well-known/jwks.json` du service auth
(cache 5 min) : pas d'appel réseau vers auth par requête.

## 5. Backlog priorisé

### P0 — indispensable au MVP social

| ID | Tâche | État |
|---|---|---|
| P0-1 | Socle identité : rôles multiples, sessions, rotation sûre, OTP durci, JWKS, fail-fast, CI auth | **Fait** (ce jalon) |
| P0-2 | Package `packages/shared-go` : logger, request ID, enveloppe d'erreur, vérification JWT via JWKS, health, métriques Prometheus, OpenTelemetry | À faire |
| P0-3 | Backbone d'événements : table outbox + relais vers Redpanda, schémas versionnés, table d'idempotence consommateur, DLQ ; premier événement `poro.user.created` | À faire |
| P0-4 | Service user : profil, username unique, avatar (upload présigné), attribution du rôle CREATOR | À faire |
| P0-5 | Pipeline vidéo : init upload → multipart présigné → complete → événement → FFmpeg HLS multi-résolutions → miniatures → `poro.video.ready` | À faire |
| P0-6 | Social : follow, like, commentaire, partage, compteurs | À faire |
| P0-7 | Feed : following, chronologique, trending, For You à règles ; pagination par curseur ; cache Redis | À faire |
| P0-8 | Notifications push FCM | À faire |
| P0-9 | Modération de base : signalement, filtre texte, actions ADMIN/MODERATOR | À faire |
| P0-10 | Fournisseur SMS réel derrière l'interface `SMSSender` | Bloqué : choix du fournisseur |
| P0-11 | CI : golangci-lint, gosec, Trivy (actions épinglées par SHA), Dependabot | À faire |
| P0-12 | Staging : Terraform + k3s + Helm + ingress, sauvegardes Postgres (après vérification des prix actuels) | À faire |
| P0-13 | Observabilité minimale : Prometheus, Grafana, Loki, Tempo | À faire |

### P1 — important
Marketplace (boutique, produits, variantes, stock, panier, checkout), abstraction `PaymentProvider`
et premier fournisseur, livraison, liste et révocation des sessions par appareil, suppression de
compte, vérification d'email, réinitialisation du mot de passe, tests de charge k6, DAST (ZAP),
signature d'image (cosign), ArgoCD, intégration Auth côté Android (Android Studio), squelette iOS.

### P2 — amélioration
Recommandation ML, analytics ClickHouse, recherche, chat, ranking personnalisé.

### P3 — futur
Live, économie des créateurs, publicité.

## 6. Décisions qui reviennent au propriétaire

1. **Périmètre du MVP** : la marketplace fait-elle partie du premier MVP (prompt maître) ou vient-elle après le social (règles Cursor) ?
2. **Fournisseur SMS** (Africa's Talking, Twilio, Termii, opérateur local…) : coût par SMS et couverture par pays.
3. **Fournisseurs de paiement** à contractualiser en premier (CinetPay, Flutterwave, Paystack, Wave, Stripe, GeniusPay…) : décision financière et juridique.
4. **Pays de lancement** : impacte les indicatifs, les langues, les fournisseurs et la conformité (protection des données).
