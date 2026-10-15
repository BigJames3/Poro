# État d'exécution — socle essentiel

Suivi du mandat « finaliser le socle essentiel ». Mis à jour à chaque étape cohérente.
Statuts : À FAIRE · EN COURS · BLOQUÉE · TERMINÉE.

Branche d'intégration : `feat/socle-essentiel`, partie de `main` (`a949cea`). Elle fusionne
(commits de fusion, aucun historique réécrit) les branches livrées mais non fusionnées, puis
porte les tâches suivantes en commits séparés. Aucune PR n'est ouverte sans demande explicite.

## Ordre d'exécution

| # | Tâche | Statut | Justification de l'ordre |
|---|---|---|---|
| 0 | Intégration des branches en attente | TERMINÉE | les tâches suivantes touchent les mêmes fichiers (compose, CI, configs) |
| 1 | PORO-007 isoler FFmpeg | TERMINÉE | surface d'attaque la plus exposée (fichiers d'inconnus) |
| 2 | PORO-002 propager la suppression de compte | TERMINÉE (durée de conservation des commandes : validation juridique) | obligation de protection des données, réutilise l'outbox existante |
| 3 | PORO-011 CI et images | TERMINÉE (protection de branche : action humaine) | prérequis de toute livraison (staging, CD) |
| 4 | PORO-009 observabilité | TERMINÉE (routage des alertes : action humaine) | prérequis d'une exploitation de staging |
| 5 | PORO-010/012 staging, CD, sauvegardes | TERMINÉE côté code ; mise en service BLOQUÉE (hébergeur, secrets) | dépend de 3 et 4 ; accès hébergeur absents |
| 6 | PORO-013 Android et connexion API | BLOQUÉE côté Android (périmètre interdit) ; côté backend TERMINÉE | code Android hors périmètre autorisé (voir plus bas) |

## 0. Intégration des branches en attente — TERMINÉE

- **Branches fusionnées** : `docs/audit-etat-avancement`, `fix/storage-access-control`
  (PORO-001), `fix/session-revocation` (PORO-003), `fix/explicit-app-env` (PORO-004),
  `feat/auth-abuse-limits` (PORO-005/006).
- **Conflits résolus** :
  - `services/video/README.md` ;
  - `README.md` ;
  - configs `user` et `shop` : bucket d'uploads conservé, secrets sans valeur par défaut ;
  - auth `NewEmailService` : reçoit à la fois `SessionRevoker` et `LoginGuard`. Une
    réinitialisation de mot de passe révoque les sessions **et** lève le verrou de login ;
    un test refuse chacun des deux absents ;
  - test de config auth.
- **Conflit sémantique corrigé** : `storage-migrate` exigeait `POSTGRES_PASSWORD` depuis
  PORO-004.
  - Plutôt que lui donner un secret inutile, il valide désormais seulement le stockage
    (`config.LoadStorage`).
  - Compose et `infra/storage/tests/run.sh` ne lui passent plus JWKS ni Kafka.
  - Les specs e2e SeaweedFS fixent `APP_ENV`.
- **Tests et résultats** :
  - Go : auth, video, social, feed, search, outbox-relay, shared-go. Build, vet,
    `go test` et golangci-lint verts ; auth avec `-race`.
  - Node : user 128, shop 159, order 126, notification 156. Lint, typecheck et Jest verts.
  - moderation : ruff, format, mypy (`src`) et 95 tests pytest verts.
  - `infra/storage/tests/run.sh` contre un vrai SeaweedFS : matrice d'accès, migration, e2e
    user/shop, tous verts.
- **Risque** : `mypy .` sur `services/moderation/tests` signale 5 erreurs. Elles existent déjà
  sur `main` et sont hors du périmètre de la CI (`uv run mypy` cible `src`).

## 1. PORO-007 — Isoler FFmpeg — TERMINÉE

- **Fichiers** :
  - `services/video/internal/transcode/transcode.go` ;
  - `services/video/internal/media/media.go` ;
  - `services/video/internal/worker/handler.go` ;
  - tests `transcode/security_test.go`, `transcode_test.go`, `media_test.go`, `handler_test.go` ;
  - `docker-compose.yml` (`media-worker`) ;
  - `infra/media-worker/check_sandbox.sh` ;
  - CI (`-race` pour la vidéo) ;
  - ADR-0007 (complément), README vidéo, runbook vidéo.
- **Changements** :
  - démuxeur imposé d'après les octets magiques ;
  - listes blanches codecs, dimensions et flux ; `-map` explicite ;
  - `-protocol_whitelist file`, `-enable_drefs 0`, `-nostdin`, `-threads 2`, `-max_alloc` ;
  - environnement réduit à `PATH` ; délais (probe 30 s, 10 min, `WaitDelay`) ;
  - reprises bornées du stockage, puis `failed` ;
  - conteneur durci : UID 10001, `cap_drop ALL`, `no-new-privileges`, racine en lecture
    seule, tmpfs `/tmp` noexec, 2 Gio sans swap, 2 CPU, 128 PID, `nofile`, pas de core.
- **Bug existant corrigé** : une source non 4:2:0 (HEVC 10 bits des iPhone, 4:4:4) faisait
  échouer l'encodage baseline. La sortie est maintenant forcée en `yuv420p`.
- **Tests et résultats** :
  - **Avec le vrai ffmpeg 6.1.1** :
    - une playlist pointant vers un serveur local n'est jamais suivie (0 connexion), qu'elle
      soit envoyée telle quelle, déguisée derrière une signature MP4, ou ouverte avec le
      mauvais démuxeur ;
    - un témoin, avec http autorisé, prouve que le serveur détecte bien les connexions ;
    - codec, résolution, nombre de flux, codec audio et fichier tronqué sont refusés ;
    - un fichier Matroska valide passe.
  - **Avec de faux binaires** :
    - aucun secret dans l'environnement de ffprobe ;
    - un ffprobe bloqué est tué ;
    - un chemin contenant `$(…)` n'est jamais interprété.
  - **Mutation** : chacune des 7 protections retirée une à une fait échouer au moins un test.
  - **Couverture** : `transcode` 87,5 %, `worker` 77,2 %, `media` 100 %. Le module vidéo est
    à 64,4 % au total, une dette existante (handlers HTTP 19 %).
  - **Bac à sable** : `check_sandbox.sh` passe sur un conteneur lancé avec les options du
    compose. Sur un conteneur par défaut, il signale 12 écarts.
- **Écarts** :
  - Le plafond de 4 flux ne compte que l'audio et la vidéo, pour ne pas refuser les `.mov`
    d'iPhone.
  - L'image worker n'a pas pu être construite ici : `apk` est inaccessible depuis cet
    environnement. Le script a donc été validé sur un conteneur témoin, pas sur
    `poro-media-worker`.
- **Risques résiduels** :
  - Le worker garde un accès réseau à Postgres, Kafka et S3. Une exécution de code dans un
    décodeur l'atteindrait. Étape suivante possible : FFmpeg dans un bac à sable sans réseau
    (gVisor ou nsjail).
  - Il faut reconstruire l'image régulièrement pour les CVE de FFmpeg.
- **Action humaine** : après `docker compose up -d --build media-worker`, lancer
  `infra/media-worker/check_sandbox.sh`.

## 2. PORO-002 — Propager la suppression de compte — TERMINÉE (une validation juridique)

- **Spécification** : [ADR-0018](architecture/adr/0018-effacement-du-compte-dans-les-services.md).
  Ce qui est effacé et gardé, service par service, avec la règle des commandes en cours.
- **Mécanisme** :
  - un consommateur `poro-<service>-account-erasure` dans les 9 services, sur l'outbox, Kafka,
    l'inbox et la DLQ existants ;
  - un message invalide part directement en DLQ ;
  - les objets S3 sont supprimés après la validation de la base ; l'inbox n'est réclamée qu'une
    fois le stockage traité ;
  - les tombstones `erased_users` empêchent un événement tardif de recréer les données ;
  - les événements de domaine existants sont réutilisés en aval : `video.deleted`,
    `like/follow/comment.deleted`, `order.cancelled`, `product.updated` et `shop.updated`.
- **Règles clés** :
  - **profil** anonymisé, l'ancien pseudo renvoie 404 et redevient libre ;
  - **vidéos** supprimées : médias en quarantaine, sources supprimées ;
  - **social** : likes, abonnements, partages et commentaires retirés, compteurs justes ;
  - **boutique** fermée quel que soit son statut, produits archivés, médias supprimés ;
  - **commandes** non expédiées annulées (`account_deleted`, remboursement si payée) ;
    expédiées terminées normalement ; contacts et carnet d'adresses effacés ;
  - **notification** : jetons et historique supprimés ;
  - **modération** : signaleur pseudonymisé par un UUID aléatoire, journal conservé.
- **Contrat** : nouveau motif `account_deleted` dans `poro.order.order.cancelled`. Notification le
  gère : sans ce changement, ces annulations seraient parties en DLQ.
- **Tests et résultats** (relancés après la fusion du travail des 4 implémenteurs) :
  - **Go** :
    - video, social, feed et search : gofmt, vet, golangci-lint (0) et `go test -race` verts ;
    - nouveaux tests d'effacement, de rejeu, de second événement, de panne de stockage et
      d'événements invalides ;
    - un cas Redpanda par service ;
    - migrations up et down testées.
  - **Node** : format, lint et typecheck verts. Tests et couverture :

    | Service | Tests | Couverture |
    |---|---|---|
    | user | 134 | 94,7 % |
    | shop | 168 | 95,1 % |
    | order | 133 | 96,1 % |
    | notification | 163 | 96,6 % |

  - **moderation** : ruff, format et mypy verts, 106 tests.
  - **Stockage réel** (`infra/storage/tests/run.sh`) : vert.
- **Risques résiduels** :
  - **Une transaction par compte et par service** : convient au volume du MVP. Passer à un
    traitement par lots au-delà de 10 000 éléments.
  - **Partages** : les compteurs dans feed et search ne baissent pas (aucun événement de
    suppression).
  - **Notification** : une course théorique entre l'effacement et une notification simultanée
    reste possible (READ COMMITTED). Les rejeux et les événements tardifs sont couverts.
- **Décision à valider (juridique)** : durée de conservation des commandes (ids, montants,
  lignes) et base légale de cette conservation.

## 3. PORO-011 — CI et images — TERMINÉE (une action humaine)

- **Fichiers** : `.github/workflows/ci.yml`, `.github/workflows/images.yml` (nouveau),
  `SECURITY.md`.
- **Changements** :
  - **`ci-ok`** : un job qui agrège les 18 autres et échoue si l'un a échoué ou a été annulé.
    Les jobs écartés par le filtre de chemins comptent comme réussis. C'est le seul contrôle à
    exiger. Jusqu'ici, la protection documentée n'exigeait que `detect-changes` et `security` :
    une PR aux tests rouges pouvait être fusionnée.
  - **`repo-checks`** (à chaque changement) : actionlint sur les workflows, `docker compose
    config` avec `.env.example`, shellcheck des scripts du worker.
  - **`mobile-build`** : il exécutait `echo` et passait toujours. Il assemble maintenant
    réellement l'APK debug (`gradlew assembleDebug`, SDK Android installé). Aucun fichier Android
    n'est modifié.
  - **Vidéo** : tests en `-race`, comme les autres modules Go.
  - **`images.yml`** : publication sur GHCR après une CI verte sur un push de `main`, jamais
    depuis une PR.
    - Étiquette immuable `sha-<commit>` : jamais `latest`, et une étiquette déjà publiée n'est
      jamais réécrite.
    - Scan Trivy bloquant avant la publication, labels OCI (source, révision), attestation de
      provenance signée.
    - Droits limités : `packages`, `id-token` et `attestations` en écriture, au niveau du job
      seulement.
    - Actions épinglées par SHA, vérifiés sur les dépôts officiels.
- **Tests et résultats** :
  - actionlint 1.7.7 (avec shellcheck) : 0 erreur sur les deux workflows.
  - Logique de `ci-ok` exécutée localement : `success skipped` → 0, `failure` → 1,
    `cancelled` → 1.
  - `docker compose config` valide.
- **Non vérifiable ici** :
  - Les workflows ne tournent que sur GitHub. La première exécution de `images.yml` aura lieu au
    premier push sur `main` après la fusion.
  - Le build Android n'a pas été lancé localement, pour ne rien générer dans
    `apps/mobile-android`. S'il échoue en CI, c'est l'état réel du projet Android, à traiter par
    l'équipe mobile (propriétaire du dossier, voir `.github/dependabot.yml`).
- **Bloqué (droits)** : la protection de branche (`ci-ok` et `security` requis, pas de
  contournement, pas de force push) demande un administrateur du dépôt. Les outils disponibles ici
  n'ont pas cette API. La procédure est dans `SECURITY.md`.

## 4. PORO-009 — Observabilité — TERMINÉE (une action humaine)

- **Fichiers** :
  - `infra/observability/` : Prometheus, règles et leurs tests, Alertmanager, Grafana ;
  - `docker-compose.yml` : profil `observability`, uniquement sur 127.0.0.1 ;
  - `packages/shared-go/kafka` et `packages/shared-go/logging` ;
  - les consommateurs Node et Python ;
  - `docs/runbooks/observability.md` ;
  - CI : job `observability`.
- **Changements** :
  - **Collecte** : 19 cibles (APIs, relais outbox, gateway, Redpanda) et un tableau de bord
    RED, outbox, consommateurs et SMS.
  - **Alertes** (8) : service tombé, taux de 5xx, latence p95, retard et échecs de l'outbox,
    DLQ, réessais en boucle, budget SMS.
  - **Consommateurs** : métrique `kafka_consumer_records_total`, identique en Go, Node et
    Python.
  - **Corrélation** : `request_id` côté HTTP ; `event_id` et `event_subject` dans tous les logs
    des consommateurs.
  - **Rédaction des logs** : masquage des secrets et données personnelles en Go (champs,
    messages, erreurs) et en Node (pino).
- **Tests et résultats** :
  - `promtool test rules` vert ;
  - la pile réelle (8 règles, Alertmanager, 19 cibles, tableau de bord, Grafana en lecture
    seule) : 7 contrôles verts ;
  - compteurs testés en Go (Redpanda réel), en Node (4 services) et en Python ;
  - rédaction testée en Go ;
  - suites Node : user 135, shop 169, order 134, notification 164 ; moderation : 107.
- **Limites** :
  - propagation du contexte de trace OpenTelemetry à travers Kafka non faite : il faudrait une
    colonne dans chaque outbox ; la corrélation passe par `event_subject` et `event_id` ;
  - media-worker sans endpoint de métriques ;
  - seuils d'alerte à ajuster au trafic réel.
- **Action humaine** : choisir le canal d'astreinte et créer le webhook (secret) pour
  Alertmanager.

## 5a. PORO-012 — Sauvegardes — TERMINÉE côté code (mise en service : action humaine)

- **Fichiers** : `infra/backup/backup.sh`, `infra/backup/restore_verify.sh`,
  `infra/backup/tests/run.sh`, `docs/runbooks/backup.md`, CI (job `backup`, inclus dans
  `ci-ok`).
- **Changements** :
  - un `pg_dump` par base `poro_*`, chiffré `age` avant écriture ; le refus est net sans clé
    publique ;
  - manifeste SHA-256 écrit et envoyé en dernier ;
  - aucune sauvegarde partielle laissée après un échec ;
  - copie distante par S3 SigV4 avec des identifiants en écriture seule, passés par stdin ;
  - rétention locale limitée aux répertoires du script ;
  - restauration vérifiée dans des bases jetables, toujours supprimées, avec un garde-fou contre
    la production.
- **Tests et résultats** : `infra/backup/tests/run.sh` passe ses 7 contrôles contre un vrai
  Postgres 16 et SeaweedFS ; shellcheck propre.
- **Décisions à prendre (métier ou juridique)** :
  - RPO 24 h, RTO 4 h et rétention de 35 jours sont des **propositions** ;
  - durée de conservation légale des commandes.
- **Bloqué (accès)** :
  - choisir le fournisseur du bucket de sauvegarde, sur un autre compte avec Object Lock ;
  - planifier la tâche ;
  - répliquer les objets S3 (médias) chez un second fournisseur.

## 5b. PORO-010 — Staging et CD — TERMINÉE côté code ; BLOQUÉE pour la mise en service (hébergeur, secrets)

- **Fichiers** :
  - `deploy/deploy.sh` ;
  - `deploy/staging/compose.staging.yml` ;
  - `deploy/tests/run.sh` ;
  - `.github/workflows/deploy-staging.yml` ;
  - CI (job `deploy`, inclus dans `ci-ok`) ;
  - `docs/runbooks/deploy.md`.
- **Changements** :
  - **Déploiement manuel de staging** :
    - approbation par l'environnement GitHub `staging` ;
    - vérification que le commit est sur `main` et que chaque image a une attestation de
      provenance de la CI ;
    - SSH avec empreinte d'hôte épinglée ;
    - test de fumée par la gateway.
  - **`deploy.sh`** :
    - images `sha-<commit>` uniquement, jamais de build local ;
    - migrations d'abord, puis attente du statut running ou healthy ;
    - **rollback automatique** vers la version précédente, avec historique et verrou.
  - **Migrations** : règle expand/contract documentée pour rester compatibles avec un rollback.
  - **Production** : aucun déploiement automatique, conformément au mandat.
- **Tests et résultats** :
  - `deploy/tests/run.sh`, contre un vrai projet Compose, passe ses 5 contrôles :
    - une version saine passe ;
    - une version malsaine revient à la précédente ;
    - une migration en échec revient aussi à la précédente ;
    - l'historique est tenu ;
    - un sha malformé et l'absence de secrets sont refusés.
  - La couche staging ne garde aucun service construit localement (0 sur 24) : vérifié en CI.
  - actionlint et shellcheck sont propres.
- **Bloqué (accès, coût, décision)** :
  - choix de l'hébergeur ;
  - couche d'environnement staging (Postgres TLS, S3, SMTP, SMS managés) ;
  - environnement GitHub `staging` et ses secrets.

  Tant que ces éléments manquent, les services refusent de démarrer en `APP_ENV=staging` et le
  workflow s'arrête en listant ce qui manque.

## 6. PORO-013 — Android et connexion API — BLOQUÉE côté Android ; TERMINÉE côté backend

- **Restriction respectée** : `apps/mobile-android/` est hors du périmètre autorisé. C'est une
  règle permanente du propriétaire, et `.github/dependabot.yml` attribue ce dossier à l'équipe
  mobile. Aucun fichier Android n'a été modifié ni généré (pas de build local).
- **Fait côté backend** :
  - `docs/api/ANDROID-INTEGRATION.md` :
    - URL de l'API pour l'émulateur et le téléphone (`adb reverse`) ;
    - enveloppe d'erreur, `X-Request-ID`, `Retry-After` ;
    - cycle des jetons avec rafraîchissement sérialisé, révocation immédiate ;
    - suppression de compte ;
    - upload multipart présigné et formats acceptés ;
    - enregistrement FCM.

    Les affirmations ont été vérifiées contre les contrats OpenAPI.
  - **CI** : `mobile-build` assemble réellement l'APK debug, au lieu d'un `echo` qui passait
    toujours.
- **Dépendances bloquantes (équipe mobile)** :
  - code Kotlin applicatif ;
  - permission `INTERNET` ;
  - *network security config* de dev ;
  - projet Firebase et `google-services.json` ;
  - identifiant d'application définitif (aujourd'hui `com.example.myporo`).
- **Vérification finale de la gateway** : contrat des 133 routes et e2e avec le vrai service auth,
  tous deux verts sur la branche intégrée.

## Interventions humaines requises (cumul)

- Lancer `infra/media-worker/check_sandbox.sh` sur le vrai conteneur après reconstruction.
- Administrateur GitHub : protection de `main` avec `ci-ok` et `security` requis (`SECURITY.md`).
- Sauvegardes : générer la clé `age` hors ligne, créer le bucket de sauvegarde (autre compte,
  Object Lock), planifier `backup.sh`, valider RPO/RTO (`docs/runbooks/backup.md`).
- Staging : choisir l'hébergeur, écrire la couche d'environnement staging, créer l'environnement
  GitHub `staging` (relecteurs, secrets SSH, variables) : `docs/runbooks/deploy.md`.
- Astreinte : canal et webhook Alertmanager pour staging et production.
- Juridique : durée de conservation des commandes d'un compte effacé (ADR-0018).
- Équipe mobile : développer l'application selon `docs/api/ANDROID-INTEGRATION.md` ; décider si
  le build Android réel doit bloquer la CI dès maintenant.
- Revue et fusion de `feat/socle-essentiel` dans `main` (aucune PR ouverte sans demande).
- Après la première publication : rendre les paquets GHCR privés ou publics selon la politique
  du projet (GHCR crée les paquets privés par défaut).
