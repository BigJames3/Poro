# Backlog Poro

Ce backlog est issu de l'audit du 14/10/2026, réalisé sur `main` au commit `a949cea`. Le rapport complet est dans [AUDIT-ETAT-AVANCEMENT.md](AUDIT-ETAT-AVANCEMENT.md).

Les identifiants `PORO-xxx` sont stables : une tâche terminée garde le sien, et une tâche abandonnée est barrée, jamais renumérotée. Les estimations sont indicatives, en jours-personne pour un développeur qui connaît le dépôt, tests et documentation compris.

**Priorités :**
- **P0** : bloque un lancement public ou expose des données.
- **P1** : nécessaire avant de monter en charge.
- **P2** : amélioration produit importante.
- **P3** : plus tard.

## Vue d'ensemble

| ID | Domaine | Titre | Priorité | Estim. | Dépend de |
|---|---|---|---|---|---|
| PORO-001 | Vidéo / stockage | Rendre privées les sources vidéo et définir les politiques de bucket | P0 | 3 j | — |
| PORO-002 | Comptes / RGPD | Effacer les données dans tous les services sur `poro.auth.user.deleted` | P0 | 6 j | — |
| PORO-003 | Auth | Révoquer les access tokens des sessions révoquées | P0 | 2 j | — |
| PORO-004 | Config | Refuser de démarrer sans `APP_ENV` explicite | P0 | 1 j | — |
| PORO-005 | Auth / OTP | Plafonner l'abus SMS (SMS pumping) | P0 | 3 j | — |
| PORO-006 | Auth | Limiter le login par compte et verrouiller progressivement | P0 | 1,5 j | — |
| PORO-007 | Vidéo | Isoler FFmpeg et limiter les ressources de `media-worker` | P0 | 2 j | — |
| PORO-008 | Stockage | Purger les uploads abandonnés (vidéo, avatar, boutique) | P0 | 2,5 j | PORO-001 |
| PORO-009 | Infra | Observabilité : métriques, logs, traces, alertes (P0-13) | P0 | 6 j | — |
| PORO-010 | Infra | Environnement de staging et déploiement continu (P0-12) | P0 | 10 j | PORO-009, PORO-011 |
| PORO-011 | CI/CD | Publier les images et rendre obligatoires tous les checks | P0 | 2 j | — |
| PORO-012 | Infra | Sauvegardes Postgres et stockage objet, avec restauration testée | P0 | 3 j | PORO-010 |
| PORO-013 | Android | Socle de l'application et connexion à l'API auth | P0 | 15 j | — |
| PORO-014 | Android / CI | Build Android réel en CI : signature, minification, sauvegardes désactivées | P0 | 2 j | PORO-013 |
| PORO-015 | Paiements | Service payment avec Wave et webhooks signés | P0 | 12 j | PORO-016 |
| PORO-016 | Événements | Restreindre qui peut produire sur les topics sensibles (ACL Redpanda) | P0 | 3 j | PORO-010 |
| PORO-017 | Événements | Outbox : écarter les messages toxiques et alerter | P1 | 2 j | PORO-009 |
| PORO-018 | Événements | Outil de rejeu des DLQ | P1 | 2 j | — |
| PORO-019 | Base de données | Purger `otp_codes`, `refresh_tokens` et `processed_events` | P1 | 1,5 j | — |
| PORO-020 | Vidéo | Réessayer les erreurs S3 transitoires et nettoyer les objets partiels | P1 | 2 j | — |
| PORO-021 | Commandes | Bloquer les actions d'une boutique suspendue sur ses commandes | P1 | 0,5 j | — |
| PORO-022 | Commandes | Redemander l'accord de l'acheteur si le prix a changé | P1 | 2 j | — |
| PORO-023 | Commandes | Ne montrer le contact au vendeur qu'après la réservation du stock | P1 | 1 j | — |
| PORO-024 | Rôles | Administration des rôles ADMIN / MODERATOR / SUPPORT avec piste d'audit | P1 | 3 j | — |
| PORO-025 | Modération | Modérer les produits et les boutiques | P1 | 5 j | — |
| PORO-026 | Recherche | Indexer les produits et les boutiques | P1 | 4 j | — |
| PORO-027 | Marketplace | Commissions et journal des montants dus | P1 | 6 j | PORO-015 |
| PORO-028 | Paiements | Reversements aux vendeurs | P1 | 6 j | PORO-027 |
| PORO-029 | Vidéo | CDN Bunny devant HLS, avec purge au retrait | P1 | 3 j | PORO-001 |
| PORO-030 | Infra | Healthchecks des services applicatifs, `depends_on` et digest Traefik | P1 | 1,5 j | — |
| PORO-031 | CI | Seuils de couverture Go partout, `-race` sur video, govulncheck | P1 | 1 j | — |
| PORO-032 | Documentation | Mettre à jour README, état des lieux et ADR, et retirer les scripts obsolètes | P1 | 1,5 j | — |
| PORO-033 | Qualité | Tests de charge sur le feed, l'upload et le checkout | P1 | 4 j | PORO-010 |
| PORO-034 | Sécurité | Gestion des secrets et rotation des clés JWT en production | P1 | 3 j | PORO-010 |
| PORO-035 | Paiements | Second fournisseur GeniusPay | P2 | 6 j | PORO-015 |
| PORO-036 | Commandes | Litiges, retours et remboursements | P2 | 8 j | PORO-015 |
| PORO-037 | Notifications | Email et SMS transactionnels (commandes) | P2 | 4 j | — |
| PORO-038 | Vidéo | Édition des métadonnées et visibilité privée ou non listée | P2 | 3 j | — |
| PORO-039 | Social | Hashtags tendance | P2 | 2 j | — |
| PORO-040 | Chat | Messagerie privée et présence | P2 | 15 j | PORO-013 |
| PORO-041 | Analytics | Pipeline d'événements vers ClickHouse et tableaux de bord | P2 | 8 j | PORO-009 |
| PORO-042 | Auth | Réduire l'énumération de comptes | P2 | 1 j | — |
| PORO-043 | Feed | Recommandation apprise (embeddings, Qdrant) | P3 | 20 j | PORO-041 |
| PORO-044 | Livraison | Transporteurs et calcul des frais | P3 | 10 j | — |
| PORO-045 | Notifications | Lier un jeton FCM à la session qui l'a enregistré | P3 | 1 j | — |
| PORO-046 | Dépôt | Trier les 25 branches Dependabot et la branche `claude/infra-hardening` | P1 | 1 j | — |

**Total indicatif :** environ 202 jours-personne (P0 : 74, P1 : 50, P2 : 47, P3 : 31).

## Détail des tâches

### PORO-001 — Rendre privées les sources vidéo et définir les politiques de bucket

- **Domaine** : Vidéo / stockage. **Priorité** : P0. **Estimation** : 3 j.
- **Objectif** : seuls les flux HLS et les miniatures d'une vidéo publiée sont lisibles publiquement.
- **Constat** :
  - La source `videos/{user}/{video}/source.{ext}` est dans le même bucket `poro-videos` que le HLS servi publiquement (`services/video/internal/config/config.go:79-82`, `internal/model/video.go:80-82`).
  - Sa clé se déduit des identifiants renvoyés par l'API.
  - `seaweedfs-init` crée les buckets sans aucune politique (`docker-compose.yml:125-142`).
- **Sous-tâches** :
  1. Créer un bucket privé `poro-video-sources`, que le worker lit et dont la source est supprimée après transcodage ou conservée N jours (à décider).
  2. Définir des politiques de bucket : `poro-videos` en lecture publique sur `hls/` et `thumb/` seulement ; `poro-avatars` et `poro-shop` en lecture publique sur les sorties ré-encodées seulement ; `uploads/` privé.
  3. Migrer les objets existants avec un script idempotent.
  4. Écrire un ADR qui complète l'ADR-0007.
- **Critères d'acceptation** :
  - Un GET anonyme sur l'URL d'une source renvoie 403, en compose comme en staging.
  - Un GET anonyme sur `master.m3u8` d'une vidéo publiée renvoie 200.
  - Un GET anonyme sur un upload brut d'avatar renvoie 403.
- **Tests** : test d'intégration qui fait un GET anonyme sur SeaweedFS (testcontainer) ; test unitaire des clés.
- **Risques** : migration des objets existants ; URL déjà distribuées.

### PORO-002 — Effacer les données dans tous les services sur `poro.auth.user.deleted`

- **Domaine** : Comptes / protection des données. **Priorité** : P0. **Estimation** : 6 j.
- **Objectif** : un compte effacé ne laisse plus aucune donnée personnelle servie par l'API.
- **Constat** : l'événement est émis (`services/auth/internal/repository/account_purge.go:106-113`) mais aucun service ne le consomme. L'ADR-0013 (l.50-51) le reconnaît. `GET /users/by-username` continue de servir le profil.
- **Sous-tâches**, un consommateur idempotent (inbox + DLQ) par service :
  - **user** : anonymiser le profil, supprimer l'avatar.
  - **video** : mettre les vidéos en quarantaine.
  - **social** : retirer commentaires, likes, abonnements et partages, et recalculer les compteurs.
  - **feed** et **search** : désindexer.
  - **shop** : fermer la boutique et archiver les produits.
  - **order** : effacer le carnet d'adresses et les contacts des commandes terminées ; garder les commandes en cours selon une règle à décider.
  - **notification** : supprimer les jetons et l'historique.
  - **moderation** : pseudonymiser le signaleur.
- **Critères d'acceptation** :
  - Un test E2E Kafka par service : après l'événement, aucune ligne ne contient l'identifiant du compte en clair, hors tables d'audit documentées.
  - `GET /users/by-username/<ancien>` renvoie 404.
- **Tests** : E2E Kafka par service ; rejeu de l'événement (idempotence) ; DLQ.
- **Risques** : règles métier pour les commandes en cours et les obligations comptables ; ordre des événements.

### PORO-003 — Révoquer les access tokens des sessions révoquées

- **Domaine** : Auth. **Priorité** : P0. **Estimation** : 2 j.
- **Objectif** : après « déconnecter les autres appareils », un reset de mot de passe ou une demande de suppression, les access tokens de ces sessions sont refusés.
- **Constat** :
  - `RevokeOtherSessions` (`services/auth/internal/service/account_service.go:104-114`) et `ResetPassword` ne révoquent que les refresh tokens.
  - Le middleware (`internal/middleware/auth.go:20-40`) ne vérifie que le `jti`.
  - Les autres services ne vérifient que la signature (`packages/shared-go/jwtauth/jwtauth.go:3-4`).
- **Sous-tâches** :
  1. Mettre le `sid` en liste noire Redis pendant la durée de vie d'un access token.
  2. Faire vérifier le `sid` par le middleware d'auth.
  3. Choisir entre propager le contrôle (lecture Redis partagée ou topic de révocation) ou réduire le TTL ; trancher par ADR.
- **Critères d'acceptation** :
  - Un access token d'une session révoquée renvoie 401 sur auth.
  - Le comportement des autres services est décidé et testé.
- **Tests** : unitaires du middleware ; E2E revoke-others.
- **Risques** : Redis devient sur le chemin critique des autres services.

### PORO-004 — Refuser de démarrer sans `APP_ENV` explicite

- **Domaine** : Configuration. **Priorité** : P0. **Estimation** : 1 j.
- **Objectif** : un oubli de configuration ne peut pas lancer la production en mode dev (OTP écrit en clair dans les logs, `sslmode=disable`, mot de passe par défaut).
- **Constat** : `APP_ENV` vaut `dev` par défaut dans auth (`config.go:277`), user (`app.config.ts:64`) et outbox-relay (`config.go:91`). Les autres services sont à vérifier un par un.
- **Sous-tâches** :
  1. Rendre `APP_ENV` obligatoire dans tous les services.
  2. Garder `dev` explicite dans `docker-compose.yml`.
  3. Supprimer le mot de passe Postgres par défaut dans le code.
- **Critères d'acceptation** : chaque service quitte en erreur si `APP_ENV` est absent, et un test de configuration par service le prouve.
- **Tests** : tests de config Go, Node et Python.
- **Risques** : faibles.

### PORO-005 — Plafonner l'abus SMS

- **Domaine** : Auth / OTP. **Priorité** : P0. **Estimation** : 3 j.
- **Objectif** : borner le coût SMS même avec des IP et des numéros tournants.
- **Constat** : les limites actuelles sont 5/min par IP, et 60 s de délai puis 5/h par numéro (`services/auth/internal/middleware/rate_limit.go`, `service/otp_delivery.go:59-80`). Il n'existe ni plafond global, ni plafond par préfixe, ni défi.
- **Sous-tâches** :
  1. Plafonds global et quotidien par pays et par préfixe, en Redis.
  2. Disjoncteur qui coupe l'envoi au-delà d'un budget.
  3. Défi (Play Integrity ou captcha) au-delà d'un seuil.
  4. Métriques et alertes (dépend de PORO-009 pour les alertes).
- **Critères d'acceptation** :
  - Au-delà du budget configuré, l'API répond 429 ou 503, sans appel au fournisseur.
  - Un test le démontre avec un fournisseur simulé.
- **Tests** : unitaires avec miniredis ; E2E HTTP.
- **Risques** : bloquer de vrais utilisateurs lors d'un pic légitime.

### PORO-006 — Limiter le login par compte et verrouiller progressivement

- **Domaine** : Auth. **Priorité** : P0. **Estimation** : 1,5 j.
- **Objectif** : empêcher la force brute répartie sur un même compte.
- **Constat** : seule une limite de 10/min par IP existe, avec un compteur partagé `auth:rl:login:`.
- **Sous-tâches** :
  1. Compteur d'échecs par compte (email normalisé) dans Redis.
  2. Délai exponentiel et verrouillage temporaire.
  3. Notification à l'utilisateur après N échecs (optionnelle, à décider).
- **Critères d'acceptation** : N échecs depuis N IP différentes produisent un 429 sur le compte visé, sans bloquer les autres comptes.
- **Tests** : service et E2E.
- **Risques** : verrouillage volontaire d'un compte par un tiers ; le délai progressif est préférable au blocage dur.

### PORO-007 — Isoler FFmpeg et limiter les ressources de `media-worker`

- **Domaine** : Vidéo. **Priorité** : P0. **Estimation** : 2 j.
- **Objectif** : un fichier piégé ne peut ni lire de ressource réseau ou locale, ni épuiser l'hôte.
- **Constat** :
  - Il manque `-protocol_whitelist`, un format d'entrée forcé et `-nostdin` (`services/video/internal/transcode/transcode.go`).
  - `media-worker` n'a ni `mem_limit`, ni `cpus`, ni `read_only`, ni `cap_drop` (`docker-compose.yml:767-805`).
- **Sous-tâches** :
  1. Ajouter `-nostdin -protocol_whitelist file` et forcer le démuxeur d'après la signature.
  2. Fixer les limites et le durcissement du conteneur.
  3. Tester avec un fichier qui référence une URL externe.
- **Critères d'acceptation** : le fichier de test qui référence une URL externe est rejeté sans aucune connexion sortante, et le conteneur tourne en lecture seule.
- **Tests** : transcode, `TestFFmpegTranscodeIfInstalled` étendu, et un test en CI avec ffmpeg installé.
- **Risques** : rejeter des formats légitimes.

### PORO-008 — Purger les uploads abandonnés

- **Domaine** : Stockage. **Priorité** : P0. **Estimation** : 2,5 j. **Dépend de** : PORO-001.
- **Objectif** : borner le stockage consommé par des uploads jamais finalisés.
- **Constat** :
  - Aucune purge des lignes `uploading` ni des multiparts S3 incomplets (`services/video/internal/repository/video.go:35,94`).
  - Les uploads bruts d'avatar et de boutique orphelins ne sont jamais supprimés.
- **Sous-tâches** :
  1. Tâche périodique avec `SKIP LOCKED` qui abandonne les multiparts de plus de 24 h et marque la ligne `abandoned`.
  2. Règle de cycle de vie S3 sur les préfixes `uploads/`.
  3. Plafond de taille par partie au moment de la présignature, si le stockage le permet.
- **Critères d'acceptation** : après le délai, ni la ligne `uploading` ni le multipart ne subsistent.
- **Tests** : repository et service avec SeaweedFS en testcontainer.
- **État (2026-10-22)** : **terminé** (sous réserve de la CI).
  - Sous-tâche 1 : nettoyage applicatif horaire des envois vidéo, avatar et boutique de plus de 24 h (PR #58 et #60). Suppression logique de la ligne vidéo plutôt qu'un statut `abandoned`, que la contrainte `videos_status_chk` refuserait sans migration.
  - Sous-tâche 2 : durée de vie d'un jour posée par le filer sur `poro-avatar-uploads` et `poro-shop-uploads` (`init.sh`), et service `seaweedfs-janitor` qui annule les multiparts inachevés de plus de 24 h. Testé par `infra/storage/tests/run.sh`. La règle de cycle de vie S3 n'est pas appliquée par SeaweedFS 4.45 sans mode `fastpath` : non utilisée.
  - Sous-tâche 3 : longueur exacte de chaque partie vidéo signée dans l'URL présignée ; SeaweedFS refuse toute autre taille (403).
  - Production (Backblaze B2) : règles documentées dans `docs/runbooks/storage.md`, à appliquer à la mise en place.
- **Risques** : supprimer un upload lent mais légitime ; choisir un délai généreux.

### PORO-009 — Observabilité (plan P0-13 en attente de validation)

- **Domaine** : Infra. **Priorité** : P0. **Estimation** : 6 j.
- **Objectif** : voir et être alerté sur les erreurs, la latence, le lag Kafka, l'outbox en attente et les DLQ.
- **Constat** :
  - Les services exposent `/metrics` (`packages/shared-go/metrics`) et le tracing existe (`packages/shared-go/tracing`).
  - Aucune pile de collecte ni aucune alerte n'existe dans le dépôt.
- **Sous-tâches** :
  1. Prometheus, Grafana (127.0.0.1:3300), Loki et Tempo dans compose, sous un profil `observability`.
  2. Tableaux de bord par service.
  3. Alertes : taux de 5xx, outbox en attente, messages en DLQ, lag des consommateurs, échecs SMS.
  4. Runbook.
- **Critères d'acceptation** :
  - Chaque service apparaît `up` dans Prometheus.
  - Une alerte de test se déclenche quand un message arrive en DLQ.
- **Tests** : script de vérification des cibles ; job CI qui valide les règles (`promtool check rules`).
- **Risques** : ports réservés sur le poste du développeur (3000, 3001, 5432, 6379, 1025, 8025 à éviter).

### PORO-010 — Environnement de staging et déploiement continu (plan P0-12 en attente de validation)

- **Domaine** : Infra. **Priorité** : P0. **Estimation** : 10 j. **Dépend de** : PORO-009, PORO-011.
- **Objectif** : un environnement de staging reproductible, déployé automatiquement depuis `main`.
- **Constat** : aucun Terraform, Helm ou manifeste k8s, et aucun job de déploiement (`.github/workflows/ci.yml` est le seul workflow).
- **Sous-tâches** :
  1. Choisir l'hébergeur (Hetzner proposé, prix à vérifier) et la base de données (CloudNativePG ou base managée).
  2. Infrastructure as code.
  3. Secrets, DNS et TLS.
  4. Déploiement par digest d'image.
  5. Smoke tests après déploiement (`infra/gateway/tests/e2e_auth.sh`).
- **Critères d'acceptation** :
  - Un merge sur `main` déploie en staging sans action manuelle.
  - Les smoke tests passent.
  - Le retour arrière vers le digest précédent est documenté et testé.
- **Tests** : smoke tests et vérification de la configuration.
- **Risques** : coûts ; décisions de l'utilisateur requises (hébergeur, domaine).

### PORO-011 — Publier les images et rendre obligatoires tous les checks

- **Domaine** : CI/CD. **Priorité** : P0. **Estimation** : 2 j.
- **Constat** :
  - Aucune publication d'images.
  - SECURITY.md (l.45-48) ne recommande comme checks obligatoires que `detect-changes` et `security`.
- **Sous-tâches** :
  1. Job de publication vers GHCR, avec signature (cosign) et SBOM.
  2. Job agrégé `ci-ok` qui dépend de tous les jobs.
  3. Rendre `ci-ok` obligatoire dans la protection de branche (réglage GitHub à faire par l'utilisateur).
- **Critères d'acceptation** :
  - Un merge publie une image par service, tagguée par SHA et signée.
  - Une PR dont un job service échoue ne peut pas être fusionnée.
- **Tests** : PR de test avec un échec volontaire.
- **Risques** : permissions du dépôt.

### PORO-012 — Sauvegardes et restauration testée

- **Domaine** : Infra. **Priorité** : P0. **Estimation** : 3 j. **Dépend de** : PORO-010.
- **Objectif** : un RPO de 24 h au plus (au mieux 15 min avec WAL), un RTO documenté.
- **Constat** : aucun dispositif de sauvegarde dans le dépôt.
- **Sous-tâches** :
  1. Sauvegarde Postgres de chaque base (pgBackRest ou sauvegarde managée).
  2. Réplication ou versioning du stockage objet.
  3. Restauration mensuelle automatisée et runbook.
- **Critères d'acceptation** : une restauration en staging depuis la dernière sauvegarde est réussie et chronométrée.
- **Tests** : job de restauration.
- **Risques** : coût du stockage.

### PORO-013 — Android : socle de l'application et connexion à l'API auth

- **Domaine** : Android. **Priorité** : P0. **Estimation** : 15 j.
- **Constat** : `apps/mobile-android` est le gabarit Android Studio (`com.example.myporo`). Il ne contient ni code Kotlin dans `src/main`, ni activité, ni permission INTERNET, et son historique se limite au commit `b869e8f`.
- **Sous-tâches** :
  1. Renommer le package.
  2. Architecture : Compose, navigation, injection de dépendances.
  3. Client HTTP vers la gateway.
  4. Stockage chiffré des jetons.
  5. Écrans d'inscription et de connexion (email et OTP), refresh automatique, déconnexion.
  6. Flavors dev, staging et prod.
- **Critères d'acceptation** :
  - L'application s'installe, crée un compte sur l'API locale via la gateway, se reconnecte après le redémarrage de l'application et se déconnecte.
  - Des tests instrumentés couvrent le flux.
- **Tests** : unitaires (ViewModel, client HTTP avec MockWebServer) et instrumentés.
- **Risques** : sujet hors du périmètre de cet audit (aucune modification d'Android, conformément à la consigne). Le travail est à planifier explicitement avec l'utilisateur.

### PORO-014 — Build Android réel en CI

- **Domaine** : Android / CI. **Priorité** : P0. **Estimation** : 2 j. **Dépend de** : PORO-013.
- **Constat** :
  - Le job `mobile-build` n'exécute qu'un `echo` (`.github/workflows/ci.yml:653` et suivantes).
  - `allowBackup=true`, aucune minification, aucune signature.
- **Critères d'acceptation** :
  - La CI lance `./gradlew lint test assembleRelease` et produit un AAB signé avec un secret CI.
  - `allowBackup` est désactivé ou des règles d'exclusion sont définies.
- **Risques** : gestion du keystore.

### PORO-015 — Service payment avec Wave et webhooks signés

- **Domaine** : Paiements. **Priorité** : P0 pour le lancement du paiement en ligne ; le paiement à la livraison fonctionne déjà. **Estimation** : 12 j. **Dépend de** : PORO-016.
- **Constat** :
  - `services/payment` n'existe pas.
  - order consomme déjà `poro.payment.payment.succeeded`, `payment.failed` et `refund.succeeded` (`services/order/src/orders/saga.consumer.ts:22-28`), mais personne ne les produit.
- **Sous-tâches** :
  1. Service payment.
  2. Création de session Wave Checkout.
  3. Webhook avec vérification de signature, idempotence et rapprochement.
  4. Remboursements.
  5. Outbox, contrats et ADR.
- **Critères d'acceptation** :
  - Une commande Wave passe à `paid` dans le bac à sable Wave.
  - Un webhook rejoué ne produit aucun effet de bord.
  - Un webhook non signé reçoit 401.
- **Tests** : unitaires, E2E avec un fournisseur simulé, test de signature.
- **Risques** : contrat marchand Wave, KYC, délais d'accès au bac à sable.

### PORO-016 — Restreindre qui peut produire sur les topics sensibles

- **Domaine** : Événements. **Priorité** : P0. **Estimation** : 3 j. **Dépend de** : PORO-010.
- **Objectif** : seul le service payment peut produire `poro.payment.*`, et chaque service ne produit que sur ses propres topics.
- **Constat** : aucune ACL ni authentification SASL n'est configurée sur Redpanda dans le dépôt.
- **Critères d'acceptation** : un client authentifié comme `order` qui tente de produire sur `poro.payment.payment.succeeded` est refusé.
- **Risques** : configuration de Redpanda et des clients ; ne modifier les listeners locaux qu'avec l'accord de l'utilisateur.

### PORO-017 — Outbox : écarter les messages toxiques et alerter

- **Domaine** : Événements. **Priorité** : P1. **Estimation** : 2 j.
- **Constat** : en cas d'échec de publication, le relais incrémente `attempts` et retente le lot indéfiniment (`packages/shared-go/outbox/relay.go:171-182`). Un message rejeté en permanence bloque toute l'outbox de sa base.
- **Critères d'acceptation** :
  - Au-delà de N tentatives, le message est mis à l'écart, une métrique augmente et les messages suivants sont publiés.
  - L'ordre par clé reste garanti, ou la limite de cette garantie est documentée.
- **Tests** : `outbox_test.go` avec un éditeur qui rejette un seul message.

### PORO-018 — Outil de rejeu des DLQ

- **Domaine** : Événements. **Priorité** : P1. **Estimation** : 2 j.
- **Constat** : il n'existe qu'une procédure manuelle avec `rpk` (`docs/runbooks/outbox-relay.md`).
- **Critères d'acceptation** : une commande rejoue tout ou partie d'une DLQ vers son topic d'origine, avec un mode simulation (dry-run), et laisse une trace.

### PORO-019 — Purger `otp_codes`, `refresh_tokens` et `processed_events`

- **Domaine** : Base de données. **Priorité** : P1. **Estimation** : 1,5 j.
- **Constat** : `otpRepository.DeleteExpired` et `refreshTokenRepository.DeleteExpired` existent mais ne sont jamais appelés. Seuls les codes email sont purgés (`services/auth/internal/service/account_purge.go:61`). Rien ne purge `processed_events`.
- **Critères d'acceptation** : la tâche de purge les appelle, et un test vérifie que les lignes expirées disparaissent.

### PORO-020 — Vidéo : réessayer les erreurs S3 transitoires et nettoyer les objets partiels

- **Domaine** : Vidéo. **Priorité** : P1. **Estimation** : 2 j.
- **Constat** :
  - Toute erreur de téléchargement est classée `invalid_media` et toute erreur d'envoi `transcode_failed`, sans nouvelle tentative (`services/video/internal/worker/handler.go:94-97, 110-112`).
  - Les objets HLS déjà envoyés restent après un échec.
- **Critères d'acceptation** : une erreur S3 transitoire déclenche un nouvel essai par le consommateur, et un échec définitif supprime les objets partiels.

### PORO-021 — Bloquer les actions d'une boutique suspendue sur ses commandes

- **Domaine** : Commandes. **Priorité** : P1. **Estimation** : 0,5 j.
- **Constat** : `OrdersService.act` ne vérifie pas le statut de la boutique.
- **Critères d'acceptation** : une boutique suspendue reçoit 403 sur l'expédition et l'annulation vendeur, avec un test E2E. La règle sur les commandes en cours est à décider avec l'utilisateur.
- **État (2026-10-22)** : **terminé** (sous réserve de la CI), voir [ADR-0019](../architecture/adr/0019-suspension-des-comptes-et-des-boutiques.md).
  - Order : 403 `shop_suspended` sur l'expédition et l'annulation vendeur ; l'acheteur garde ses actions. Test E2E `refuses seller actions while the shop is suspended`.
  - Étendu aux comptes : `POST /api/v1/auth/admin/users/{id}/suspend` et `/reinstate` (ADMIN, ou MODERATOR sur un compte qui n'est pas du personnel). La suspension révoque toutes les sessions dans le registre partagé : tous les services refusent le compte immédiatement.
  - Annulation automatique des commandes non expédiées d'une boutique suspendue, avec le motif `shop_suspended` et un message à l'acheteur (ADR-0019, point 6).

### PORO-022 — Redemander l'accord de l'acheteur si le prix a changé

- **Domaine** : Commandes. **Priorité** : P1. **Estimation** : 2 j.
- **Constat** : `stockReserved` fixe le total depuis les prix de shop et confirme la commande sans le comparer au `subtotal_seen` (`services/order/src/orders/saga.service.ts:44-71`).
- **Critères d'acceptation** : un prix en hausse entre l'aperçu et la réservation met la commande en attente d'accord, ou l'annule, selon la règle choisie, avec un test.

### PORO-023 — Ne montrer le contact au vendeur qu'après la réservation

- **Domaine** : Commandes / protection des données. **Priorité** : P1. **Estimation** : 1 j.
- **Critères d'acceptation** : un vendeur ne voit pas le contact d'une commande `pending`, avec un test.

### PORO-024 — Administration des rôles avec piste d'audit

- **Domaine** : Rôles. **Priorité** : P1. **Estimation** : 3 j.
- **Constat** :
  - ADMIN, MODERATOR et SUPPORT ne s'attribuent que par SQL.
  - Aucun retrait de rôle n'est possible.
  - `jwtauth.RequireRole` n'est utilisé nulle part et SUPPORT n'est contrôlé nulle part.
- **Critères d'acceptation** :
  - Des endpoints réservés à ADMIN attribuent et retirent les rôles.
  - Chaque changement est tracé (auteur, date, motif) et un événement est publié.
  - Le retrait prend effet au plus tard au refresh suivant.

### PORO-025 — Modérer les produits et les boutiques

- **Domaine** : Modération. **Priorité** : P1. **Estimation** : 5 j.
- **Constat** : `target_type` n'accepte que `video`, `comment` et `user` (`services/moderation/src/moderation/api/schemas.py:61`).
- **Critères d'acceptation** :
  - Les produits et les boutiques peuvent être signalés et passent par le filtre automatique.
  - Un retrait masque le produit côté shop et côté recherche.
  - Un test E2E Kafka le vérifie.

### PORO-026 — Indexer les produits et les boutiques dans la recherche

- **Domaine** : Recherche. **Priorité** : P1. **Estimation** : 4 j.
- **Critères d'acceptation** : `GET /api/v1/search?type=products` trouve un produit actif et ne trouve pas un produit archivé ou une boutique suspendue.
- **État (2026-10-22)** : **terminé** (sous réserve de la CI). Le service search indexe `poro.shop.shop.updated`, `poro.shop.product.updated` et `poro.shop.product.deleted` (migration `000003_catalog`). Types `products` et `shops`, filtres `country`, `currency`, `min_price`, `max_price`. Un produit supprimé ne revient jamais ; l'effacement d'un compte retire son catalogue. Dans l'app : onglets Produits et Boutiques de la recherche, ouverts depuis l'onglet Boutique. Limite : un produit inchangé depuis plus longtemps que la rétention de Redpanda n'est indexé qu'à sa prochaine modification.

### PORO-027 — Commissions et journal des montants dus

- **Domaine** : Marketplace. **Priorité** : P1. **Estimation** : 6 j. **Dépend de** : PORO-015.
- **Constat** : aucun code ne correspond, alors que l'ADR-0012 §7 prévoit ce journal.
- **Critères d'acceptation** : une commande `completed` produit une écriture de commission et de montant dû vendeur, idempotente et rapprochable.

### PORO-028 — Reversements aux vendeurs

- **Domaine** : Paiements. **Priorité** : P1. **Estimation** : 6 j. **Dépend de** : PORO-027.

### PORO-029 — CDN Bunny devant HLS

- **Domaine** : Vidéo. **Priorité** : P1. **Estimation** : 3 j. **Dépend de** : PORO-001.
- **Constat** : le CDN n'est que documenté (ADR-0007 l.53).
- **Critères d'acceptation** : les URL HLS passent par le CDN, et un retrait de modération purge le cache.

### PORO-030 — Healthchecks des services applicatifs, `depends_on` et digest Traefik

- **Domaine** : Infra. **Priorité** : P1. **Estimation** : 1,5 j.
- **Constat** :
  - `docker-compose.yml` ne compte que 7 healthchecks, aucun sur les services applicatifs.
  - La gateway n'a pas de `depends_on`.
  - Traefik n'est pas épinglé par digest, et v3.7.14 porte 4 vulnérabilités HIGH déjà corrigées en amont.
- **Critères d'acceptation** :
  - `docker compose up --wait` attend que tous les services soient `healthy`.
  - Traefik est épinglé par digest dans une version sans vulnérabilité HIGH.

### PORO-031 — Couverture et vulnérabilités Go en CI

- **Domaine** : CI. **Priorité** : P1. **Estimation** : 1 j.
- **Constat** :
  - Seuls social, feed et search vérifient un seuil de couverture total en CI.
  - video tourne sans `-race` (`.github/workflows/ci.yml:503`).
  - auth, video, shared-go et outbox-relay n'ont aucun seuil.
- **Critères d'acceptation** : un seuil de 70 % est imposé sur tous les modules Go, `-race` est actif partout et govulncheck bloque sur les vulnérabilités atteignables.

### PORO-032 — Documentation à jour

- **Domaine** : Documentation. **Priorité** : P1. **Estimation** : 1,5 j.
- **Constat** :
  - Le README ne cite que auth, user et outbox-relay.
  - `docs/architecture/00-etat-des-lieux.md` est périmé : il présente packages et infra comme vides, la CI comme un `echo`, et le marketplace comme à faire.
  - ADR-0003 est remplacé, mais le découpage commerce n'est pas tracé.
  - Les scripts `init-project.*` sont obsolètes, le Makefile est vide et `.cursor/rules` cite MinIO.
- **Critères d'acceptation** : l'état des lieux renvoie vers ce rapport d'audit, le README liste les 11 services et la gateway, et les éléments obsolètes sont supprimés ou marqués comme tels.

### PORO-033 — Tests de charge

- **Domaine** : Qualité. **Priorité** : P1. **Estimation** : 4 j. **Dépend de** : PORO-010.
- **Critères d'acceptation** : des scénarios k6 (feed, upload, checkout) passent en staging avec des objectifs p95 définis, et les résultats sont archivés.

### PORO-034 — Gestion des secrets et rotation des clés JWT

- **Domaine** : Sécurité. **Priorité** : P1. **Estimation** : 3 j. **Dépend de** : PORO-010.
- **Critères d'acceptation** : les secrets sont stockés dans un coffre ou un gestionnaire de secrets, et une rotation de clé JWT sans coupure est démontrée en staging (le JWKS publie l'ancienne et la nouvelle clé).

### PORO-035 à PORO-045 — Backlog produit

| ID | Objectif | Critère d'acceptation principal | Risques |
|---|---|---|---|
| PORO-035 | GeniusPay comme second fournisseur, derrière l'interface `PaymentProvider` | Une commande GeniusPay passe à `paid` dans le bac à sable du fournisseur | Contrat avec le fournisseur |
| PORO-036 | Litiges, retours et remboursements | Une demande de retour suit son cycle complet, avec un test E2E | Règles métier à définir |
| PORO-037 | Email et SMS transactionnels pour les commandes | Un acheteur sans push reçoit la confirmation par email | Coût des SMS |
| PORO-038 | Édition du titre et de la description, visibilité privée ou non listée | `PATCH /videos/:id` fonctionne ; une vidéo privée est absente du feed et de la recherche | Propagation aux projections |
| PORO-039 | Hashtags tendance sur une fenêtre glissante | `/search/suggest` peut trier par tendance | — |
| PORO-040 | Chat privé et présence (WebSocket) | Deux comptes échangent des messages en temps réel ; les messages sont persistés et modérables | Charge et modération |
| PORO-041 | Analytics : événements vers ClickHouse, tableaux de bord créateurs | Les vues et la rétention sont visibles pour un créateur | ClickHouse est présent dans compose mais n'est utilisé nulle part |
| PORO-042 | Réduire l'énumération de comptes | `forgot` envoie l'email de façon asynchrone avec un temps de réponse constant ; la réponse de `register` est revue | Expérience utilisateur |
| PORO-043 | Recommandation apprise avec embeddings Qdrant | Un test A/B montre un gain sur la rétention | Qdrant est présent dans compose mais n'est utilisé nulle part |
| PORO-044 | Transporteurs et calcul des frais de livraison | Le devis de livraison apparaît dans le récapitulatif de commande | Partenaires de livraison |
| PORO-045 | Lier un jeton FCM à la session qui l'a enregistré | Un jeton n'est réattribué qu'après une déconnexion de l'ancien compte | — |

### PORO-046 — Trier les branches en attente

- **Domaine** : Dépôt. **Priorité** : P1. **Estimation** : 1 j.
- **Constat** :
  - 25 branches Dependabot ne sont pas fusionnées : images Node 25, NestJS 12, actions, modules Go, uv.
  - `claude/infra-hardening` (commit `735bd68`, 29/09/2026) n'est pas fusionnée ; elle modifie `docker-compose.yml`, le Makefile, le README et `dev.ps1`, et est probablement obsolète.
- **Critères d'acceptation** : chaque branche est fusionnée, ou fermée avec un motif écrit. Une montée de version majeure (Node 25, NestJS 12) n'est fusionnée qu'avec une CI verte.
