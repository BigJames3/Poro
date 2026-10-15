# Runbook — staging, livraison et rollback (PORO-010)

## Chaîne de livraison

1. Une PR passe la CI (`ci-ok`, voir `SECURITY.md`) et est fusionnée dans `main`.
2. `images.yml` publie les 12 images sur GHCR, étiquetées `sha-<commit>` (immuables), scannées par
   Trivy et attestées (provenance signée).
3. Un humain lance **Deploy staging** (`deploy-staging.yml`, *Run workflow*) avec le sha complet.
   L'environnement GitHub `staging` exige l'approbation d'un relecteur. Le workflow vérifie que le
   commit est sur `main` et que chaque image est attestée par la CI de ce dépôt, puis exécute
   `deploy/deploy.sh <sha>` sur l'hôte et un test de fumée par la gateway.
4. **Production : aucun déploiement automatique.** Il n'existe pas de workflow de production.
   Prérequis avant d'en créer un : hébergement choisi, secrets de production dans un gestionnaire
   dédié, sauvegardes en service avec une restauration répétée (`docs/runbooks/backup.md`),
   alertes routées vers une astreinte, et protection de branche active.

## `deploy/deploy.sh`

Il s'exécute **sur l'hôte**, depuis le checkout du commit :

- tire les images `sha-<commit>` (jamais de build local : `deploy/staging/compose.staging.yml`) ;
- démarre la pile : les jobs de migration passent d'abord (dépendances Compose), puis chaque
  service doit être *running*, et *healthy* s'il a un `HEALTHCHECK`, dans `DEPLOY_WAIT_TIMEOUT`
  (300 s) ;
- en cas de succès, enregistre la version dans `.deploy/current` et `.deploy/history` ;
- sinon, **redémarre automatiquement la version précédente**, puis sort en erreur. Si le rollback
  échoue aussi : intervention manuelle (voir plus bas) ;
- un verrou empêche deux déploiements simultanés ;
- les secrets viennent de `.env.staging` sur l'hôte, jamais du dépôt.

Testé en CI (job `deploy`) contre un vrai projet Compose : une version saine passe, une version
malsaine et une migration en échec reviennent à la version précédente, l'historique est tenu, un
sha malformé ou l'absence du fichier de secrets sont refusés.

## Migrations compatibles avec le rollback

Le rollback remet l'ancien code **sur le schéma migré** : les migrations ne se défont jamais
automatiquement. Chaque migration doit donc rester compatible avec la version précédente :

- **expand / contract** : ajouter une colonne ou une table (nullable ou avec défaut) dans une
  version, la rendre obligatoire ou supprimer l'ancienne seulement dans une version suivante,
  une fois la précédente retirée ;
- ne jamais renommer une colonne en une étape (ajouter, copier, basculer, supprimer plus tard) ;
- les migrations destructives (`DROP`, perte de données) demandent une sauvegarde vérifiée juste
  avant et une validation humaine. Les migrations descendantes avec perte sont signalées dans
  les runbooks des services (ex. auth `000004`).

## Rollback manuel

```bash
cd /srv/poro                        # STAGING_APP_DIR
cat .deploy/history                 # versions déployées, échecs, rollbacks
prev=$(cat .deploy/previous)
git checkout --detach "$prev"
PORO_IMAGE_REGISTRY=ghcr.io/<owner> deploy/deploy.sh "$prev"
```

Si le schéma n'est plus compatible (migration non conforme à la règle ci-dessus), restaurer la
base depuis la dernière sauvegarde dans une nouvelle base (`docs/runbooks/backup.md`) : c'est une
décision humaine.

## Mettre en place staging (actions humaines)

1. **Choisir l'hébergement** (décision qui engage un coût) : VM avec Docker Compose, ou plateforme
   de conteneurs. Bases Postgres managées avec TLS et sauvegardes PITR de préférence.
2. **Écrire la couche d'environnement staging.** Elle doit pointer les services vers les
   ressources managées : Postgres TLS, Redis, broker, stockage S3, SMTP, fournisseur SMS. Le
   `docker-compose.yml` de dev les fixe en dur : avec `APP_ENV=staging`, les services **refusent
   de démarrer** tant que ce n'est pas fait, et `deploy.sh` revient en arrière. L'échec est donc
   sûr par défaut, mais bloquant.
3. Sur l'hôte : un utilisateur dédié sans sudo, membre du groupe docker, le checkout du dépôt,
   `.env.staging` (droits `600`) et `docker login ghcr.io` avec un jeton en lecture seule
   (`read:packages`).
4. Dans GitHub, **Settings → Environments → staging** :
   - relecteurs requis et branche `main` seulement ;
   - secrets `STAGING_SSH_HOST`, `STAGING_SSH_USER`, `STAGING_SSH_PRIVATE_KEY` (clé dédiée, limitée
     par `command=` si possible) et `STAGING_SSH_KNOWN_HOSTS` (empreinte de l'hôte, jamais
     acceptée au premier contact) ;
   - variables `STAGING_APP_DIR` et `STAGING_BASE_URL`.
5. Secrets **différents par environnement** : aucun secret de staging n'est réutilisé en
   production, et inversement. Les clés JWT, les secrets OTP et les identifiants S3 sont générés
   par environnement.

Sans ces éléments, le workflow s'arrête dès sa première étape avec la liste de ce qui manque.
