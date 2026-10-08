# PORO — État des lieux technique et backlog

Date de l'analyse : 29 septembre 2026. Ce document est la référence de départ ;
il est mis à jour à chaque jalon P0.

## 1. Architecture actuelle (constatée)

| Élément | État réel |
|---|---|
| `services/auth` (Go 1.26, Fiber v2, pgx v5, Redis) | Identité, JWT RS256, OTP, outbox `poro.auth.user.created`, consommateur CREATOR. |
| `services/user` (NestJS 11, Prisma 7, port 8082) | Profils, username, avatars présignés, activation créateur. |
| `services/video` (Go 1.26, Fiber, port 8083) + `media-worker` | Upload multipart, HLS 360p/720p, `poro.video.ready`. |
| `services/feed` (Go 1.26, Fiber, port 8084) | Flux abonnements (fan-out à la lecture), tendances (72 h, ticker 5 min), For You (sessions Redis de 200 vidéos) ; projections vidéos, abonnements, compteurs, profils. |
| `services/social` (Go 1.26, Fiber, port 8085) | Likes, commentaires (1 niveau de réponse), abonnements, partages ; compteurs transactionnels ; outbox `poro.social.*` ; projections vidéos/comptes. |
| `services/notification` (NestJS 11, Prisma 7, port 8086) | Notifications in-app (likes regroupés par vidéo et par heure, commentaires, réponses, abonnements, vidéo en ligne), push FCM après commit, préférences par type, purge à 90 jours. |
| `packages/shared-go` | HTTP envelope, JWKS, health, Prometheus, OTel, outbox/inbox, Kafka. |
| `services/outbox-relay` | Relais SQL → Redpanda (un processus par base). |
| `services/moderation` (Python 3.13, FastAPI, port 8087) | Signalements, filtre automatique par règles (retrait immédiat des contenus bloquants), file des modérateurs ; outbox `poro.moderation.*`. |
| `services/search` (Go 1.26, Fiber, port 8088) | Recherche vidéos, comptes, hashtags et autocomplétion en Postgres plein texte (français sans accents, trigrammes) ; projection par événements ([ADR-0011](adr/0011-search-postgres-v1.md)). |
| Autres services (`chat`, `shop`, …) | Aucun code. Seulement listés dans `.cursor/rules/project.md`. |
| `packages/`, `infra/`, `data/`, `docs/` | Vides avant ce jalon. |
| `apps/mobile-android` | Projet Android Studio (36 fichiers suivis), ne consomme encore aucune API. Non modifié. |
| `docker-compose.yml` | Postgres 16 (`poro_auth`, `poro_user`, `poro_video`), Redis 7, SeaweedFS (avatars + videos), Redpanda, auth, user, video, media-worker, relais outbox. |
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
- SMS : Africa's Talking est intégré, mais aucun compte de production n'est encore configuré. Hors `dev`, le service refuse de démarrer sans identifiants (`SMS_PROVIDER=log` interdit).
- Clés JWT sur disque. En production : secret Kubernetes chiffré (SOPS/Sealed Secrets) ou Vault, rotation via plusieurs `kid` dans le JWKS.
- Pas encore de scan de conteneur (Trivy), de SAST (golangci-lint/gosec), de DAST, de signature d'image.

**Architecture**
- 13 microservices prévus pour une petite équipe, en 3 langages : coût d'exploitation et de CI élevé. Proposition de regroupement : [ADR-0003](adr/0003-frontieres-services.md).
- Backbone d'événements en place (JSON + outbox + DLQ, [ADR-0005](adr/0005-evenements-json-outbox.md)). Avro/Schema Registry reporté.

**Infrastructure**
- Images `latest` (SeaweedFS, ClickHouse, Qdrant) : builds non reproductibles. Redpanda est épinglé `v24.2.7`. À épingler par digest.
- Les règles Cursor citent MinIO, le compose utilise SeaweedFS. SeaweedFS est conservé (compatible S3, plus léger) ; les règles devront être alignées.
- Un volume Postgres créé avant ce jalon n'a que `poro_auth` : le job `postgres-init` crée `poro_user` s'il manque.
- Un volume Redpanda créé avec l'ancien listener unique (`localhost:9092`) peut laisser des partitions internes sans leader. Recréer le volume : `docker compose down` puis `docker volume rm <projet>_poro_redpanda_data`.
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
 │ video (Go)        upload multipart, métadonnées HLS     │
 │ media-worker (Go+FFmpeg) transcodage HLS                │
 │ content (Go)     social, feed — P0-6/7                  │
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
(cache 5 min) : pas d'appel réseau vers auth par requête. Un token révoqué au logout
reste acceptable hors auth jusqu'à 15 min ([ADR-0005](adr/0005-evenements-json-outbox.md)).

## 5. Backlog priorisé

### P0 — indispensable au MVP social

| ID | Tâche | État |
|---|---|---|
| P0-1 | Socle identité : rôles multiples, sessions, rotation sûre, OTP durci, JWKS, fail-fast, CI auth | **Fait** (ce jalon) |
| P0-2 | Package `packages/shared-go` : logger, request ID, enveloppe d'erreur, vérification JWT via JWKS, health, métriques Prometheus, OpenTelemetry | **Fait** |
| P0-3 | Backbone d'événements : table outbox + relais vers Redpanda, schémas versionnés, table d'idempotence consommateur, DLQ ; premier événement `poro.auth.user.created` | **Fait** — [ADR-0005](adr/0005-evenements-json-outbox.md) |
| P0-4 | Service user : profil, username unique, avatar (upload présigné), attribution du rôle CREATOR | **Fait** — [ADR-0006](adr/0006-service-user.md) |
| P0-5 | Pipeline vidéo : init upload → multipart présigné → complete → événement → FFmpeg HLS multi-résolutions → miniatures → `poro.video.ready` | **Fait** — [ADR-0007](adr/0007-pipeline-video.md) |
| P0-6 | Social : follow, like, commentaire, partage, compteurs | **Fait** — [ADR-0008](adr/0008-services-separes.md) |
| P0-7 | Feed : following, chronologique, trending, For You à règles ; pagination par curseur ; cache Redis | **Fait** — [ADR-0009](adr/0009-feed-v1.md) |
| P0-8 | Notifications push FCM | **Fait** — in-app + FCM ; [runbook](../runbooks/notification.md) |
| P0-9 | Modération de base : signalement, filtre texte, actions ADMIN/MODERATOR | **Fait** — [ADR-0010](adr/0010-moderation-v1-vague-2.md) ; médias des vidéos retirées ou supprimées en quarantaine (bucket privé) |
| P0-10 | Fournisseur SMS réel (Africa's Talking), liste des pays autorisés, anti-SMS pumping | **Fait** — [ADR-0004](adr/0004-decisions-produit-lancement.md) ; reste : compte, Sender ID, tarifs |
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

## 6. Décisions du propriétaire

Prises le 30 septembre 2026, détaillées dans [ADR-0004](adr/0004-decisions-produit-lancement.md) :
social d'abord puis marketplace en P1 ; lancement en Côte d'Ivoire, Sénégal, Cameroun et
Nigeria ; SMS via Africa's Talking ; paiements Wave et GeniusPay en premier.

Conformité à prévoir avant le lancement : lois de protection des données de chaque pays
(CI : loi 2013-450 / ARTCI ; SN : loi 2008-12 / CDP ; CM : loi 2024-017 ; NG : NDPA 2023 / NDPC).
À faire valider par un juriste.
