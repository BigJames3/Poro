# ADR-0015 — Contrôle d'accès du stockage objet

- Statut : accepté
- Date : 2026-10-14
- Complète : ADR-0006 (avatars), ADR-0007 (pipeline vidéo), ADR-0012 (images de boutique)

## Contexte

L'audit du 14/10/2026 (PORO-001) a relevé deux failles, vérifiées sur l'image SeaweedFS 4.45
épinglée :

- **SeaweedFS ne contrôlait aucun accès.** Sans configuration IAM, un client anonyme peut créer
  un bucket, écrire, lister et supprimer n'importe quel objet. Les identifiants transmis aux
  services n'étaient jamais vérifiés.
- **Les originaux étaient dans les buckets publics.** La source d'une vidéo
  (`videos/<user>/<video>/source.<ext>`) était dans `poro-videos`, servi publiquement pour le
  HLS. Sa clé se déduit des identifiants renvoyés par l'API, et les vidéos en échec, retirées ou
  non publiées étaient concernées. Les originaux d'avatars et de photos de boutique, avec leurs
  métadonnées EXIF, étaient eux aussi dans les buckets publics jusqu'à leur ré-encodage.

SeaweedFS sait limiter un utilisateur à des buckets et ouvrir un bucket entier en lecture
anonyme, depuis `weed shell`. Il applique aussi des politiques de bucket par préfixe, mais
seulement via l'API S3 signée, et `fs.mv` refuse de déplacer un objet d'un bucket à un autre.

## Décisions

1. **Un bucket = une visibilité**, plutôt que des politiques par préfixe, plus fragiles :

   | Bucket | Visibilité | Contenu |
   |---|---|---|
   | `poro-videos` | lecture publique | HLS et miniatures |
   | `poro-video-sources` | privé | fichiers vidéo originaux, conservés après transcodage |
   | `poro-quarantine` | privé | médias retirés ou supprimés |
   | `poro-avatars` | lecture publique | avatars ré-encodés |
   | `poro-avatar-uploads` | privé | originaux d'avatars, supprimés après ré-encodage |
   | `poro-shop` | lecture publique | logos et photos ré-encodés |
   | `poro-shop-uploads` | privé | originaux des images de boutique |

   La lecture publique n'autorise que `GET` et `HEAD` sur un objet : jamais de liste, d'écriture
   ni de création de bucket.

2. **Un identifiant S3 par service**, limité à ses buckets, avec les droits `Read`, `Write` et
   `List`, sans `Admin` : un service ne peut ni toucher aux buckets d'un autre, ni supprimer ou
   créer un bucket. `video` et `media-worker` partagent l'identité `poro-video`.

3. **Configuration déclarative.** `infra/storage/init.sh` (service compose `seaweedfs-init`)
   crée les buckets puis remplace toute la configuration IAM par import (`s3.iam.import
   -apply`) à chaque démarrage. Une rotation de clé consiste à changer la variable puis à
   relancer compose. Les secrets viennent de l'environnement, transitent par un fichier
   temporaire privé et n'apparaissent jamais dans les logs.

4. **Démarrage ordonné.** `user`, `shop`, `video` et `media-worker` attendent la fin de
   `seaweedfs-init`, donc aucun ne démarre avant que l'accès soit restreint.

5. **Migration idempotente des données existantes.**
   - Le service ponctuel `video-storage-migrate` (`services/video/cmd/storage-migrate`) déplace
     par l'API S3 (copie puis suppression) les sources restées dans `poro-videos` et
     `poro-quarantine`.
   - `init.sh` supprime les originaux d'avatars et de boutique laissés dans les buckets publics.
     Ils ne vivent que quelques minutes ; un upload en cours pendant la mise à jour devra être
     refait.

6. **La quarantaine ne déplace que les médias publics** : une source n'est jamais publique,
   elle reste dans `poro-video-sources`.

7. **Le contrôle d'accès est testé contre un vrai SeaweedFS.** `infra/storage/tests/run.sh` (job
   CI `storage`) vérifie :
   - la matrice d'accès complète de chaque identité et de l'anonyme ;
   - la rotation des clés et le refus d'une clé partagée entre services ;
   - la migration ;
   - le code de stockage de video (Go), user et shop (JavaScript) avec signatures vérifiées.

## Conséquences

- `.env` doit contenir `S3_USER_*`, `S3_VIDEO_*` et `S3_SHOP_*` (identifiant et secret,
  distincts). Compose refuse de démarrer tant qu'ils manquent. `S3_ACCESS_KEY_ID` et
  `S3_SECRET_ACCESS_KEY` ne servent plus.
- Toute personne ou tout outil qui veut lire un bucket privé a besoin d'un identifiant :
  l'accès anonyme ne donne plus que les médias publics.
- Un identifiant ajouté à la main dans SeaweedFS disparaît au démarrage suivant : toute
  identité doit être déclarée dans `init.sh`.
- En production (Backblaze B2 prévu), la même séparation s'applique avec des clés
  d'application limitées par bucket. Elle sera reprise par P0-12.
- La purge des envois abandonnés et la limite de taille par partie sont traitées par PORO-008
  (voir `docs/runbooks/storage.md`, « Expiration des envois abandonnés »). Reste hors périmètre le CDN
  devant `poro-videos` (PORO-029).
