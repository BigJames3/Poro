# Audit technique et état d'avancement de Poro

- **Date** : 14/10/2026.
- **Référence** : `main` au commit `a949cea` (13/10/2026), 72 commits.
- **Méthode** : lecture du code, des migrations, des routes, des tests, de la configuration et des workflows, puis exécution non destructive de tous les tests, lints et contrôles de types (hors Android), dans le conteneur d'audit. Aucun code n'a été modifié.
- **Backlog associé** : [BACKLOG-PORO.md](BACKLOG-PORO.md). Les identifiants `PORO-xxx` cités ici y renvoient.

**Statuts utilisés :**

| Statut | Sens |
|---|---|
| **Terminé et vérifié** | Code présent, testé, et tests exécutés pendant l'audit. |
| **Implémenté, validation incomplète** | Code et tests présents, mais jamais validé dans un environnement réel (staging, appareil, fournisseur réel) ou trou fonctionnel mineur. |
| **Partiellement implémenté** | Une partie significative du périmètre attendu manque. |
| **En cours** | Travail ouvert sur une branche non fusionnée. |
| **Non commencé** | Aucun code ; au mieux une documentation, une ADR ou un dossier vide. |
| **Bloqué** | Dépend d'une décision ou d'un tiers. |
| **État à confirmer** | Impossible à prouver depuis le dépôt. |

---

## 1. Résumé exécutif

**Le backend est solide :**
- 11 services en Go, NestJS et Python, une gateway Traefik, une architecture événementielle (outbox, Redpanda, inbox, DLQ) et 32 contrats d'événements validés par des tests.
- Tous les tests passent : 7 modules Go, 4 services Node (535 tests), moderation (87 tests) et le test de contrat de la gateway (133 routes).
- Lints et contrôles de types sont propres.

Les parcours comptes, profils, vidéo, social, feed, recherche, modération, notifications et marketplace (paiement à la livraison) sont codés de bout en bout côté API.

**Ce qui manque :**
- **Aucun client** : l'application Android est le gabarit vierge d'Android Studio, le produit n'est donc pas utilisable par un humain.
- **Pas de staging, de déploiement, d'observabilité ni de sauvegardes.**
- **Paiement en ligne absent** : Wave, GeniusPay, commissions et reversements ne sont pas commencés.
- **Effacement de compte non propagé** : `poro.auth.user.deleted` n'est consommé par aucun service.
- **Chat, analytics et recommandation apprise** : non commencés.

**Risques de sécurité principaux :**
- sources vidéo dans un bucket servi publiquement, sous une clé prévisible ;
- access tokens des sessions révoquées encore valides ;
- configuration qui retombe en mode dev par défaut ;
- abus SMS non plafonné globalement ;
- FFmpeg sans isolation.

**Avancement estimé** :
- environ **51 %** du périmètre MVP, toutes couches confondues ;
- environ **64 %** du backend seul, ou **72 %** hors fonctionnalités reportées par ADR.

Le calcul est détaillé en section 2.

## 2. État global

### Méthode de calcul

Chaque domaine reçoit un poids (son importance relative pour le MVP) et un taux d'avancement selon son statut :

| Statut | Taux |
|---|---|
| Terminé et vérifié | 100 % |
| Implémenté, validation incomplète | 85 % |
| Partiellement implémenté | 50 % |
| En cours | 25 % |
| Non commencé | 0 % |

L'avancement est la moyenne pondérée. Le chiffre reste subjectif à cause des poids, qui sont explicites ci-dessous pour pouvoir être ajustés.

| Domaine | Poids | Statut | Taux | Contribution |
|---|---:|---|---:|---:|
| Auth (email, OTP, JWT, refresh, sessions) | 8 | Implémenté, validation incomplète | 85 % | 6,8 |
| Rôles | 2 | Partiellement implémenté | 50 % | 1,0 |
| Cycle de vie du compte (suppression et propagation) | 3 | Partiellement implémenté | 50 % | 1,5 |
| Profils, username, avatar, créateurs | 4 | Implémenté, validation incomplète | 85 % | 3,4 |
| Vidéo (upload, stockage, FFmpeg, HLS) | 8 | Implémenté, validation incomplète | 85 % | 6,8 |
| CDN | 2 | Non commencé | 0 % | 0 |
| Outbox, Redpanda, DLQ | 5 | Implémenté, validation incomplète | 85 % | 4,25 |
| Feed (heuristique) | 5 | Implémenté, validation incomplète | 85 % | 4,25 |
| Social (likes, commentaires, abonnements, partages, hashtags) | 5 | Implémenté, validation incomplète | 85 % | 4,25 |
| Recherche | 3 | Implémenté, validation incomplète | 85 % | 2,55 |
| Modération | 4 | Implémenté, validation incomplète | 85 % | 3,4 |
| Notifications push | 3 | Implémenté, validation incomplète | 85 % | 2,55 |
| Boutiques et catalogue | 5 | Implémenté, validation incomplète | 85 % | 4,25 |
| Panier, checkout, commandes, stock | 6 | Implémenté, validation incomplète | 85 % | 5,1 |
| Paiements (Wave, GeniusPay) | 6 | Non commencé | 0 % | 0 |
| Commissions et reversements | 3 | Non commencé | 0 % | 0 |
| Livraison (au-delà du suivi texte) | 1 | Non commencé | 0 % | 0 |
| Chat et présence | 3 | Non commencé | 0 % | 0 |
| Analytics | 2 | Non commencé | 0 % | 0 |
| Reco ML | 1 | Non commencé | 0 % | 0 |
| Gateway | 2 | Implémenté, validation incomplète | 85 % | 1,7 |
| Android (application et connexion API) | 15 | Non commencé | 0 % | 0 |
| Infra de déploiement (staging, IaC, sauvegardes) | 6 | Non commencé | 0 % | 0 |
| Observabilité | 3 | Partiellement implémenté (instrumentation sans collecte) | 50 % | 1,5 |
| CI/CD | 3 | Partiellement implémenté (CI oui, CD non) | 50 % | 1,5 |
| **Total** | **108** | | | **54,8 / 108 ≈ 51 %** |

Pour le **backend seul** (lignes Auth à Gateway, poids 81), le total est de 51,8 / 81 ≈ **64 %**. Si l'on retire aussi les domaines explicitement reportés par les ADR (CDN, livraison avancée, chat, analytics, reco ML ; poids 9), on obtient 51,8 / 72 ≈ **72 %**.

### Synthèse par couche

| Couche | État |
|---|---|
| Backend API et événements | Fonctionnel et testé localement ; jamais exécuté en environnement partagé |
| Client mobile | Inexistant |
| Paiement en ligne | Inexistant (paiement à la livraison seulement) |
| Exploitation (déploiement, supervision, sauvegardes) | Inexistante |
| Qualité (tests, lint, CI) | Bonne ; quelques trous (voir section 9) |

## 3. Architecture réelle

Cette section décrit l'architecture constatée dans le code, pas celle de la documentation.

```
Client (absent) ──▶ Traefik v3.7.14  127.0.0.1:8000  (infra/gateway)
                      │  routes /api/v1/<service>/…, CORS, gzip, limite 1 MiB, rate limit 100/200
                      ▼
 ┌──────── Go (Fiber, pgx) ──────────┐  ┌──── NestJS (Prisma 7) ────┐  ┌─ Python ─┐
 │ auth      video (+ media-worker)  │  │ user   shop   order       │  │moderation│
 │ social    feed    search          │  │ notification              │  │ (FastAPI)│
 └───────────────────────────────────┘  └───────────────────────────┘  └──────────┘
        │ outbox (même transaction)            │                               │
        ▼                                      ▼                               ▼
 outbox-relay × 7 (une instance par base : auth, user, video, social, moderation, shop, order)
        │ acks=all
        ▼
 Redpanda ── topics poro.<domaine>.<entité>.<évènement> + <topic>.dlq
        │
        ▼  consommateurs idempotents (table processed_events) avec DLQ
 Postgres (une base par service) · Redis (rate limit, sessions de feed, blacklist JWT, OTP)
 SeaweedFS S3 : poro-avatars, poro-videos, poro-quarantine, poro-shop
 ClickHouse et Qdrant : déclarés dans compose, utilisés par aucun service
```

**Points notables :**
- **JWT** : RS256 émis par auth. Les autres services vérifient localement avec le JWKS (`packages/shared-go/jwtauth`, `services/user/src/auth/token-verifier.ts`), sans consulter la blacklist.
- **Événements** : enveloppe JSON `{id,type,version,source,subject,occurred_at,data}`. Les 32 types de `packages/shared-go/events/catalog.go` correspondent exactement aux 32 schémas de `packages/contracts/events/`.
- **Écarts avec la documentation** :
  - ADR-0003 (content, commerce, reco, analytics) est remplacé par le découpage réel en 11 services, sans ADR qui trace le découpage de commerce en shop et order.
  - `docs/architecture/00-etat-des-lieux.md` décrit un état antérieur.
- **Compose** : 34 services, dont 7 seulement ont un healthcheck (infrastructure et gateway). Les ports sont liés à 127.0.0.1.

## 4. Fonctionnalités et preuves

| Domaine | Fonctionnalité | État réel | Preuve dans le dépôt | Travaux restants | Priorité |
|---|---|---|---|---|---|
| Auth | Inscription et connexion email | Implémenté, validation incomplète | `services/auth/internal/service/auth_service.go:149-206` (argon2id, `burnPasswordCheck`) ; tests `auth_service_test.go`, E2E `internal/e2e/account_flow_test.go` | Limite par compte (PORO-006), énumération (PORO-042) | P0 |
| Auth | Vérification email, mot de passe oublié, reset | Implémenté, validation incomplète | `email_service.go`, migration 000009, `email_code_repository.go:115-140` (reset qui révoque les sessions) ; tests verts | Envoi SMTP réel non testé hors fake ; révocation des access tokens (PORO-003) | P0 |
| Auth | OTP SMS | Implémenté, validation incomplète | `otp_service.go` (HMAC lié au numéro, `crypto/rand`, essais atomiques), `sms_africastalking.go`, `otp_delivery.go:59-80` | Africa's Talking jamais appelé en réel ; plafond global (PORO-005) ; purge `otp_codes` (PORO-019) | P0 |
| Auth | JWT RS256 et JWKS | Terminé et vérifié | `token_service.go:109-247`, `GET /.well-known/jwks.json` ; `token_service_test.go`, `jwtauth_test.go` | Rotation des clés en prod (PORO-034) | P1 |
| Auth | Refresh avec rotation et détection de réutilisation | Terminé et vérifié | `auth_service.go:213-331`, `refresh_token_repository.go:191-198` ; TestRefreshReuseRevokesFamily, TestRefreshGrace* | Purge `refresh_tokens` (PORO-019) | P1 |
| Auth | Logout et révocation | Implémenté, validation incomplète | Blacklist `jti` (`auth_service.go:334-346`, `middleware/auth.go:30-36`) | Sessions révoquées à distance encore valides ; autres services sans blacklist (PORO-003) | P0 |
| Auth | Sessions par appareil | Implémenté, validation incomplète | `account_service.go:64-114`, ADR-0013 | Même lacune (PORO-003) | P0 |
| Comptes | Suppression différée et purge | Implémenté (côté auth) | `account_service.go:119-194`, `repository/account_purge.go:40-114`, migration 000008, `e2e/account_deletion_test.go` | — | — |
| Comptes | Propagation de l'effacement | Non commencé | `grep poro.auth.user.deleted` : émetteur seulement ; ADR-0013 l.50-51 | Consommateurs dans 9 services (PORO-002) | P0 |
| Rôles | Stockage et claims | Implémenté | `user_roles` avec CHECK sur 7 rôles (migration 000004), claim `roles` | — | — |
| Rôles | CREATOR et BUSINESS automatiques | Terminé et vérifié | `consumer/creator.go`, `consumer/business.go` ; tests verts | — | — |
| Rôles | ADMIN, MODERATOR, SUPPORT | Partiellement implémenté | Contrôles dans shop (`shops.controller.ts:30,116`) et moderation (`http/auth.py:11,41`) ; attribution par SQL seulement ; `RequireRole` inutilisé | Administration avec piste d'audit (PORO-024) | P1 |
| Profils | Profil, username, profil public | Implémenté, validation incomplète | `services/user/src/profiles/*`, `username.ts` ; 118 tests verts | Effacement (PORO-002) | P0 |
| Profils | Avatar | Implémenté, validation incomplète | `avatars/avatar.service.ts` (POST présigné 5 Mo, Sharp en WebP, EXIF supprimé) | Uploads bruts orphelins et politique de bucket (PORO-001, PORO-008) | P0 |
| Profils | Activation créateur | Terminé et vérifié | `profiles.service.ts:193-217` puis `poro.user.creator.activated` puis rôle CREATOR | — | — |
| Vidéo | Upload multipart présigné | Implémenté, validation incomplète | `services/video/internal/service/videos.go:57-228` (taille, signature, nombre de parties) | Purge des abandons (PORO-008) | P0 |
| Vidéo | Stockage | Partiellement implémenté | Buckets créés par `seaweedfs-init` (`docker-compose.yml:125-142`) | Source privée, politiques (PORO-001) | P0 |
| Vidéo | FFmpeg et HLS | Implémenté, validation incomplète | `transcode/transcode.go` (360p/720p, segments de 4 s, miniature, délai de 10 min) ; tests verts, test ffmpeg conditionnel | Isolation (PORO-007) ; reprises sur erreur S3 (PORO-020) | P0 |
| Vidéo | Métadonnées, liste, suppression | Partiellement implémenté | `videos.go:231-318` ; pas de PATCH ni de visibilité privée | PORO-038 | P2 |
| Vidéo | CDN Bunny | Non commencé | ADR-0007 l.53 (documentation seulement) | PORO-029 | P1 |
| Événements | Outbox transactionnelle | Terminé et vérifié | `packages/shared-go/outbox/outbox.go:38-54`, `services/user/src/events/outbox.ts` ; `outbox_test.go` vert | — | — |
| Événements | Relais | Implémenté, validation incomplète | `outbox/relay.go` (verrou advisory, acks=all, nettoyage à 72 h) | Message toxique bloquant (PORO-017) | P1 |
| Événements | Inbox et DLQ | Implémenté, validation incomplète | `inbox/inbox.go`, `kafka/kafka.go:128-232`, `event-consumer.ts:56-140` | Outil de rejeu (PORO-018) ; purge `processed_events` (PORO-019) ; ACL (PORO-016) | P1 |
| Événements | Contrats | Terminé et vérifié | 32 schémas = 32 types ; `TestCatalogMatchesContracts` vert | Topics `poro.payment.*` sans émetteur | — |
| Feed | For You, Following, Trending | Implémenté, validation incomplète | `services/feed/internal/{mix,trending,service}` ; tests verts (couverture 71 à 100 % par paquet) | Signal de vue, tests de charge (PORO-033) | P1 |
| Feed | Reco ML | Non commencé | Aucun `services/reco` ; Qdrant inutilisé | PORO-043 | P3 |
| Social | Likes, commentaires, abonnements, partages | Implémenté, validation incomplète | `services/social/internal/service/*`, compteurs transactionnels ; tests verts | Édition de commentaire sur vidéo retirée (faible) | P2 |
| Social | Hashtags | Partiellement implémenté | Extraction dans video (`text.Hashtags`), indexation dans search ; pas de tendances | PORO-039 | P2 |
| Chat | Messagerie et présence | Non commencé | ADR-0010 (« reporté »), aucun code | PORO-040 | P2 |
| Marketplace | Boutiques | Implémenté, validation incomplète | `services/shop/src/shops/*`, CHECK handle, pays et devise ; `shops.e2e.spec.ts` | Effacement (PORO-002) | P0 |
| Marketplace | Catalogue, variantes, images | Implémenté, validation incomplète | `products.service.ts`, `media.service.ts` (Sharp, 5 Mo) | Modération (PORO-025), indexation (PORO-026) | P1 |
| Marketplace | Stock (réservation, libération, consommation) | Terminé et vérifié | `stock/stock.service.ts` (`FOR UPDATE` ordonné, CHECK) ; `stock.e2e.spec.ts:269` « never sells the last unit twice » vert | — | — |
| Marketplace | Panier, checkout, commandes | Implémenté, validation incomplète | `services/order/src/{cart,checkout,orders}` (verrou advisory, idempotence, scheduler) ; 119 tests verts | Prix modifié (PORO-022), boutique suspendue (PORO-021), contact (PORO-023) | P1 |
| Marketplace | Livraison | Non commencé (suivi texte libre seulement) | `ShipDto.tracking`, `DELIVERY_FEE_NOTICE` ; ADR-0012 | PORO-044 | P3 |
| Marketplace | Commissions | Non commencé | Aucune occurrence de commission ou de payout dans `services/` ; ADR-0012 §7 | PORO-027 | P1 |
| Paiements | Wave | Non commencé (côté consommateur seulement) | `saga.consumer.ts:22-28` consomme `poro.payment.*` ; aucun `services/payment` ; `PAYMENT_METHODS=cash_on_delivery` par défaut | PORO-015, PORO-016 | P0 |
| Paiements | GeniusPay | Non commencé | ADR-0004 et ADR-0012 seulement | PORO-035 | P2 |
| Paiements | Reversements | Non commencé | — | PORO-028 | P1 |
| Notifications | In-app | Terminé et vérifié | `services/notification` (idempotence `groupKey`, préférences, purge) ; 149 tests verts | — | — |
| Notifications | Push FCM | Implémenté, validation incomplète | `push/push.sender.ts:50-77`, nettoyage des jetons invalides | Jamais testé avec Firebase réel ; APNs via FCM non vérifié | P1 |
| Notifications | Email et SMS transactionnels | Non commencé | Aucun code dans `services/notification/src` | PORO-037 | P2 |
| Modération | Signalements, file, décisions | Implémenté, validation incomplète | `services/moderation/src/moderation/{api,cases}` ; 87 tests verts, couverture 96,45 % | Produits et boutiques (PORO-025) | P1 |
| Modération | Filtre automatique par règles | Implémenté, validation incomplète | `rules/engine.py`, `rules/fr.yml`, `scanner.py` | Couverture linguistique (wolof, nouchi…) : état à confirmer | P2 |
| Recherche | Vidéos, utilisateurs, hashtags | Implémenté, validation incomplète | `services/search` (Postgres FTS `search_fr`, `pg_trgm`) ; tests verts | Produits (PORO-026) | P1 |
| Analytics | Pipeline et tableaux de bord | Non commencé | ClickHouse déclaré dans compose (`docker-compose.yml:206`), utilisé nulle part | PORO-041 | P2 |
| Android | Application | Non commencé | `apps/mobile-android` = gabarit `com.example.myporo`, sans Kotlin dans `src/main` ni permission INTERNET ; un seul commit, `b869e8f` | PORO-013 | P0 |
| Android | Connexion à l'API | Non commencé | Idem | PORO-013 | P0 |
| Infra | Gateway | Implémenté, validation incomplète | `infra/gateway/*`, ADR-0014 ; contrat de 133 routes vert pendant l'audit | Vulnérabilités HIGH de Traefik, digest (PORO-030) | P1 |
| Infra | Compose local | Implémenté, validation incomplète | `docker-compose.yml` (34 services) | Healthchecks applicatifs (PORO-030) | P1 |
| Infra | Staging, IaC, déploiement | Non commencé | Aucun Terraform, Helm ou k8s ; un seul workflow, `ci.yml` | PORO-010 | P0 |
| Infra | Sauvegardes | Non commencé | — | PORO-012 | P0 |
| Observabilité | Instrumentation | Implémenté | `packages/shared-go/{metrics,tracing,logging}`, `/metrics` sur les services | — | — |
| Observabilité | Collecte, tableaux de bord, alertes | Non commencé | Plan P0-13 présenté, non validé | PORO-009 | P0 |
| CI/CD | CI | Implémenté, validation incomplète | `.github/workflows/ci.yml` : 17 jobs, filtre par chemin, actions épinglées par SHA, Trivy, npm audit, Dependabot | Seuils Go (PORO-031) ; checks obligatoires (PORO-011) | P1 |
| CI/CD | CD | Non commencé | Aucun job de publication ni de déploiement | PORO-011, PORO-010 | P0 |
| Sécurité | Secrets | Terminé et vérifié | Uniquement des `.env.example` versionnés ; aucune clé `.pem` ; mots de passe compose `${…:?}` | Coffre de secrets en prod (PORO-034) | P1 |

## 5. Tâches en cours

**Liste A — réellement en cours :**

| Élément | Preuve | État |
|---|---|---|
| Plan observabilité (P0-13) et staging (P0-12) | Plan présenté, en attente de validation par l'utilisateur ; aucun code | Bloqué en attente de décision (hébergeur, domaine, Postgres managé ou CloudNativePG) |
| 25 branches Dependabot | `git branch -r --no-merged origin/main` : Node 25, NestJS 12, actions (checkout 7.0.1, setup-node 7.1.0…), Go minor, uv minor | En attente de tri (PORO-046) |

Aucune autre branche de développement n'est active. Les PR #15, #39, #40, #42 et #46 sont fusionnées.

## 6. Travaux incomplets

**Liste B — commencés mais incomplets :**

| Élément | Ce qui existe | Ce qui manque | Backlog |
|---|---|---|---|
| Branche `claude/infra-hardening` | Commit `735bd68` du 29/09/2026 (compose, Makefile, README, `dev.ps1`) | Jamais fusionnée ; probablement dépassée par les PR suivantes | PORO-046 |
| Suppression de compte | Auth complet, événement émis | Aucun consommateur | PORO-002 |
| Révocation de sessions | Refresh révoqués, blacklist `jti` courant | `sid` non vérifié ; autres services sans révocation | PORO-003 |
| Paiement Wave | Énumération, saga order, contrats `poro.payment.*` | Service payment, webhooks, ACL | PORO-015, PORO-016 |
| Stockage vidéo | Buckets, quarantaine | Source privée, politiques, purge | PORO-001, PORO-008 |
| CDN | ADR-0007 | Intégration | PORO-029 |
| Rôles d'administration | Contrôles dans shop et moderation | Attribution, retrait, audit | PORO-024 |
| Hashtags | Extraction et recherche | Tendances | PORO-039 |
| Observabilité | Métriques et traces exposées | Collecte, alertes | PORO-009 |
| CI mobile | Job `mobile-build` (`echo`) | Build réel | PORO-014 |
| Purges | Codes email purgés | `otp_codes`, `refresh_tokens`, `processed_events` | PORO-019 |
| Documentation | 14 ADR, 12 runbooks, 10 OpenAPI | README et état des lieux périmés, scripts obsolètes | PORO-032 |
| ClickHouse et Qdrant | Conteneurs dans compose | Aucun usage | PORO-041, PORO-043 |

## 7. Backlog non commencé

**Liste C — non commencés :**
- application Android et connexion API (PORO-013, PORO-014) ;
- service payment Wave (PORO-015) et GeniusPay (PORO-035) ;
- commissions (PORO-027) et reversements (PORO-028) ;
- staging et déploiement (PORO-010), sauvegardes (PORO-012), publication d'images (PORO-011) ;
- collecte d'observabilité et alertes (PORO-009) ;
- modération des produits (PORO-025) ;
- indexation des produits (PORO-026) ;
- chat et présence (PORO-040) ;
- analytics (PORO-041) ;
- recommandation ML (PORO-043) ;
- livraison avancée (PORO-044) ;
- litiges et retours (PORO-036) ;
- notifications email et SMS (PORO-037) ;
- édition et visibilité des vidéos (PORO-038) ;
- tests de charge (PORO-033) ;
- gestion des secrets en prod (PORO-034) ;
- ACL Redpanda (PORO-016) ;
- outil de rejeu des DLQ (PORO-018).

Le détail est dans [BACKLOG-PORO.md](BACKLOG-PORO.md).

## 8. Constats sécurité et qualité

Les constats sont classés par criticité. Ceux marqués « vérifié » ont été relus directement dans le code pendant l'audit.

### Élevé

| # | Constat | Preuve | Recommandation |
|---|---|---|---|
| E1 | **Sources vidéo dans le bucket servi publiquement, sous une clé prévisible** (vérifié). Les fichiers bruts, y compris ceux des vidéos en échec, retirées ou non publiées, sont lisibles si le bucket est public, ce qu'impose le service du HLS par URL directe. | `services/video/internal/config/config.go:79-82` ; `internal/model/video.go:80-82` ; `docker-compose.yml:125-142` sans politique | Bucket source privé et politiques par préfixe (PORO-001) |
| E2 | **Absence de propagation de l'effacement de compte.** Après la purge, le profil public, les vidéos, les commentaires et la boutique restent servis : non-conformité aux obligations de protection des données (droit à l'effacement). | `poro.auth.user.deleted` sans consommateur ; ADR-0013 l.50-51 | PORO-002 |
| E3 | **Aucune sauvegarde, supervision ni environnement de staging** : un incident en production serait ni détecté ni récupérable. | Absence de fichiers correspondants dans le dépôt | PORO-009, PORO-010, PORO-012 |

### Moyen

| # | Constat | Preuve | Recommandation |
|---|---|---|---|
| M1 | Les access tokens des sessions révoquées (déconnexion des autres appareils, reset de mot de passe, suppression) restent valides jusqu'à 15 min, y compris dans auth (vérifié). | `account_service.go:104-114` ; `middleware/auth.go:20-40` ne vérifie que le `jti` | Blacklist par `sid` (PORO-003) |
| M2 | Les services autres qu'auth ignorent toute révocation. | `packages/shared-go/jwtauth/jwtauth.go:3-4` ; choix assumé par ADR-0013 | Décision par ADR (PORO-003) |
| M3 | `APP_ENV` vaut `dev` par défaut : un oubli active en prod l'OTP en clair dans les logs, `sslmode=disable` et le mot de passe par défaut. | auth `config.go:277,285`, user `app.config.ts:64`, relais `config.go:91` | PORO-004 |
| M4 | Abus SMS : aucun plafond global, quotidien ou par préfixe, ni défi. | `rate_limit.go`, `otp_delivery.go:59-80` | PORO-005 |
| M5 | Force brute répartie sur le login : limite par IP seulement. | `auth:rl:login:` 10/min | PORO-006 |
| M6 | FFmpeg sans `-protocol_whitelist`, sans format forcé et sans limite de ressources ; `media-worker` non durci. | `transcode.go` ; `docker-compose.yml:767-805` | PORO-007 |
| M7 | Uploads multipart abandonnés jamais purgés, stockage non borné. | `repository/video.go:35,94` | PORO-008 |
| M8 | Le prix réservé peut dépasser le prix vu par l'acheteur, et la commande est confirmée sans nouvel accord. | `services/order/src/orders/saga.service.ts:44-71` | PORO-022 |
| M9 | Les événements `poro.payment.*` sont acceptés de tout producteur ; aucune ACL Kafka. Le contrôle montant/devise existe. | `saga.service.ts:84-130` ; aucune configuration SASL ou ACL | PORO-016, avant d'activer Wave |
| M10 | Un message rejeté en permanence bloque toute l'outbox d'une base, avec des tentatives infinies (vérifié). | `packages/shared-go/outbox/relay.go:171-182` | PORO-017 |
| M11 | Les checks obligatoires recommandés sont limités à `detect-changes` et `security` : un service en échec ne bloque pas le merge si la protection suit SECURITY.md. | `SECURITY.md:45-48` ; réglage GitHub non vérifiable | PORO-011 |
| M12 | Traefik v3.7.14 porte 4 vulnérabilités HIGH (Go 1.26.9, x/net 0.60.0) corrigées en amont, sans digest épinglé. | Rapport Trivy CI non bloquant ; `docker-compose.yml` (gateway) | PORO-030 |

### Faible

| # | Constat | Preuve | Recommandation |
|---|---|---|---|
| F1 | Une boutique suspendue peut encore expédier et annuler ses commandes. | `OrdersService.act` (`services/order/src/orders/orders.service.ts`) | PORO-021 |
| F2 | Le vendeur voit le contact dès `pending`. | `order.dto.ts:220` | PORO-023 |
| F3 | Énumération de comptes : `register` renvoie 409 ; `forgot` a un temps de réponse différent ; `reset` renvoie 403 sur un compte bloqué. | `auth_handler.go:172-173`, `email_service.go:155` | PORO-042 |
| F4 | Tables jamais purgées : `otp_codes`, `refresh_tokens`, `processed_events` (vérifié : les `DeleteExpired` ne sont pas appelés). | `otp_repository.go:109`, `refresh_token_repository.go:143` | PORO-019 |
| F5 | Une erreur S3 transitoire rend l'échec de transcodage définitif. | `worker/handler.go:94-97,110-112` | PORO-020 |
| F6 | Un jeton FCM est réattribuable par quiconque le connaît. | `notifications.service.ts:146-155` | PORO-045 |
| F7 | L'édition d'un commentaire ne vérifie pas que la vidéo est toujours visible. | `services/social/internal/service/comments.go:96-121` | Ajouter le contrôle |
| F8 | Les rôles d'administration sont attribuables seulement par SQL, sans piste d'audit. | — | PORO-024 |
| F9 | Android : `allowBackup=true`, ni minification ni signature. | `apps/mobile-android/app/src/main/AndroidManifest.xml`, `build.gradle.kts` (lus, non modifiés) | PORO-014 |

### Points positifs vérifiés

- Aucun IDOR trouvé : l'identité vient toujours du JWT et chaque accès vérifie la propriété (vidéo, commentaires, commandes, boutiques, adresses).
- Concurrence du stock correcte (`FOR UPDATE` ordonné, CHECK) et checkout idempotent.
- OTP et codes email stockés en HMAC, refresh tokens en SHA-256, mots de passe en argon2id.
- Rate limiting dans tous les services, stocké dans Redis.
- Aucun secret versionné ; actions CI épinglées par SHA ; Trivy et npm audit en CI.

## 9. État des tests et de la CI

### Exécution pendant l'audit

Toutes les commandes ont été lancées dans le conteneur d'audit, séquentiellement, avec testcontainers (Postgres, Redpanda) via un dockerd local.

| Cible | Commande | Résultat | Durée | Couverture |
|---|---|---|---|---|
| packages/shared-go | `go test -count=1 -cover ./...` | OK | 204 s | 74,4 à 100 % par paquet ; total 90,0 % |
| outbox-relay | idem | OK | 4 s | total 50,5 % (seul `internal/config` est testé ; `cmd/relay` 0 %) |
| auth | idem | OK | 21 s | total 83,9 % |
| video | idem, puis `-race` | OK (aussi avec `-race`) | 19 s | **total 65,4 %** (handler 19,1 %, database 13,6 %) |
| social | idem | OK | 15 s | 71,2 à 100 % par paquet |
| feed | idem | OK | 19 s | 71,2 à 100 % par paquet |
| search | idem | OK | 14 s | 71,2 à 100 % par paquet |
| Go (7 modules) | `golangci-lint run ./...` (v2.14.0) | 0 problème | — | — |
| user | `npm run test:cov` | 118 tests OK (13 suites) | 41 s | lignes 94,56 %, branches 85,29 % |
| notification | idem | 149 tests OK (17 suites) | 50 s | lignes 96,55 %, branches 85,96 % |
| shop | idem | 149 tests OK (15 suites) | 45 s | lignes 94,83 %, branches 83,71 % |
| order | idem | 119 tests OK (13 suites) | 46 s | lignes 95,73 %, branches 83 % |
| 4 services Node | `npm run lint`, `npm run typecheck` | OK à la seconde exécution | — | — |
| moderation | `uv run --frozen pytest --cov` | 87 tests OK | 16 s | 96,45 % (seuil de 70 % atteint) |
| gateway | `infra/gateway/tests/run.sh` | OK : 133 routes et comportement | 26 s | — |

**Classement des échecs rencontrés :**
- **Environnement (1 cas, corrigé sans toucher au code)** : la première exécution de `lint` et `typecheck` a échoué pour les 4 services Node parce que `prisma generate` exige `DATABASE_URL`. Relancées avec une URL factice (aucune connexion), les 8 commandes passent. En CI, la variable est fournie.
- **Applicatif** : aucun.
- **Délai dépassé** : aucun.
- **Ressources** : aucun.

### CI (`.github/workflows/ci.yml`)

- **Jobs (17)** : `detect-changes`, `security` (Trivy fs, npm audit), `gateway`, `shared-go`, `outbox-relay`, `auth`, `user`, `shop`, `order`, `notification`, `moderation`, `video`, `social`, `feed`, `search`, `mobile-build`.
- **Exécution** : chaque job ne tourne que si ses chemins changent.
- **Ce qui manque** :
  - aucun seuil de couverture pour auth, video, shared-go et outbox-relay (seuls social, feed et search affichent un total ; les services Node et moderation imposent 70 %) ;
  - video tourne sans `-race` (`ci.yml:503`) ;
  - `mobile-build` se limite à un `echo` ;
  - govulncheck absent de la CI ;
  - aucun job de publication ni de déploiement.
- **Dernière CI connue** : verte sur les PR fusionnées (#15, #39, #40, #42, #46). Le statut de la CI sur `main` après le merge `a949cea` n'a pas été consulté pendant l'audit (voir section 12).

## 10. Roadmap priorisée

Cet ordre est proposé et peut être réorganisé. Les estimations viennent du backlog.

**P0 — avant tout lancement public (environ 74 j)**
1. Corriger les failles sans dépendance (≈ 10 j, parallélisable) : PORO-001, PORO-003, PORO-004, PORO-005, PORO-006, PORO-007, PORO-008.
2. Socle d'exploitation : PORO-011 (images et checks), PORO-009 (observabilité), puis PORO-010 (staging) et PORO-012 (sauvegardes).
3. Conformité : PORO-002 (effacement en aval).
4. Client : PORO-013 puis PORO-014 (Android), en parallèle des étapes 1 à 3 si une personne Android est disponible.
5. Paiement en ligne : PORO-016 (ACL) puis PORO-015 (Wave). Le lancement peut se faire d'abord en paiement à la livraison, ce qui rend cette étape reportable si l'utilisateur l'accepte.

**P1 — avant la montée en charge** : PORO-017 à PORO-034 et PORO-046. Ordre suggéré :
- PORO-031 et PORO-046 (rapides) ;
- PORO-019, PORO-021, PORO-023 ;
- PORO-017, PORO-018 ;
- PORO-022, PORO-024 ;
- PORO-025, PORO-026 ;
- PORO-029, PORO-030, PORO-032 ;
- PORO-027, PORO-028 ;
- PORO-033, PORO-034.

**P2 — produit** : PORO-035 à PORO-042 (GeniusPay, litiges, email et SMS, édition vidéo, hashtags tendance, chat, analytics, énumération).

**P3 — plus tard** : PORO-043 (reco ML), PORO-044 (transporteurs), PORO-045.

## 11. Dépendances et blocages

| Blocage | Nature | Débloque |
|---|---|---|
| Choix de l'hébergeur, du domaine de staging et de la stratégie Postgres | Décision utilisateur (plan P0-12/13 présenté) | PORO-009, PORO-010, PORO-012, PORO-016, PORO-033, PORO-034 |
| Contrat marchand Wave et accès au bac à sable | Tiers | PORO-015 puis PORO-027, PORO-028, PORO-036 |
| Contrat GeniusPay | Tiers | PORO-035 |
| Compte Firebase (FCM et APNs) | Tiers et configuration | Validation réelle du push |
| Compte Africa's Talking en production | Tiers | Validation réelle de l'OTP |
| Fournisseur SMTP de production | Tiers | Validation réelle des emails |
| Développement Android | Ressource et consigne (hors périmètre jusqu'ici) | Toute validation de bout en bout par un utilisateur, PORO-040 |
| Réglages GitHub (protection de branche) | Droits administrateur | PORO-011 |
| Règles métier (commandes en cours lors d'un effacement, prix modifié, boutique suspendue) | Décision produit | PORO-002, PORO-021, PORO-022 |

## 12. Hypothèses, limites et vérifications non effectuées

- **Android** : ni compilé, ni exécuté, ni modifié, conformément à la consigne. Le constat repose sur la lecture des fichiers et de l'historique git.
- **govulncheck** : non exécuté, car `vuln.go.dev` est inaccessible depuis le conteneur.
- **Images Docker** : non construites localement (pas d'accès réseau à apk ni à PyPI, limitation de Docker Hub). Les Dockerfiles et les scans Trivy d'images ne sont validés que par la CI des PR fusionnées.
- **Stack complète** : `docker compose up` n'a pas été lancé. Les E2E s'appuient sur testcontainers, et les tests de la gateway sur des stubs `whoami`.
- **Réglages GitHub** (protection de branche, secrets, environnements) : non vérifiables depuis le dépôt.
- **CI sur `main` après `a949cea`** : non consultée.
- **Fournisseurs réels** (Africa's Talking, SMTP, FCM et APNs, SeaweedFS en production) : jamais appelés. Le caractère public effectif des buckets en production est à confirmer.
- **Couverture** : mesurée par paquet ; le total Go est calculé avec `-coverpkg=./...`, qui inclut `cmd/` et peut différer du calcul de la CI (`-coverpkg=./internal/...`).
- **Tests ignorés** : le décompte des `t.Skip` n'a pas été fait. Les tests ont été lancés sans `-v` ; les durées (testcontainers effectifs) indiquent que les tests base de données ont bien tourné. Le test FFmpeg dépend de la présence de ffmpeg dans le conteneur : état à confirmer.
- **Analyse détaillée** : certaines analyses par domaine ont été menées par des agents de lecture, puis les constats majeurs (E1, M1, M10, F4, absence de commissions, branches, CI mobile, SECURITY.md, Android) ont été revérifiés directement dans le code. Les autres références de ligne proviennent de ces lectures et peuvent être décalées de quelques lignes.
- **Pourcentages** : ils reposent sur des poids subjectifs (section 2), ajustables.
