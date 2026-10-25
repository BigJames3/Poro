# ADR-0020 — Publications photo dans le service vidéo

- Statut : accepté (contrat validé le 2026-10-24) ; étapes 2b (service `video`), 2c (media-worker) et 2d (consommateurs) implémentées, publications photo ouvertes par défaut ; étapes 3 à 5 (Android) en cours
- Date : 2026-10-24
- Complète : [ADR-0007](0007-pipeline-video.md), [ADR-0009](0009-feed-v1.md),
  [ADR-0015](0015-controle-acces-stockage-objet.md), [ADR-0018](0018-effacement-du-compte-dans-les-services.md)
- Décisions du propriétaire (2026-10-24), validées avec le contrat :
  - capacité client `Poro-Capabilities: photo-posts` ;
  - réduction sur Android à 2048 px de côté long, JPEG qualité 92, avant l'envoi ;
  - option A (photos dans le service `video`) ;
  - 10 images au plus, 10 Mo au plus par image avant traitement ;
  - légende facultative ;
  - JPEG, PNG et WebP validés sur le contenu, HEIC refusé par le serveur ;
  - 40 Mpx au plus ;
  - EXIF corrigé, ré-encodage JPEG 2048 px qualité 85 sans GPS ;
  - aucune publication partielle ;
  - aucune photo pour un client qui ne déclare pas les afficher.

## Contexte (vérifié dans `main` à `76a062e`)

- **Service `video`** (Go, Fiber) :
  - table `videos` (`migrations/000001`, `000002`) : `title` NOT NULL, `description`, `status` (`uploading|processing|ready|failed`), `content_type`, `size_bytes > 0`, `source_key`, `s3_upload_id`, `duration_ms`, `width`, `height`, `hls_key`, `thumbnail_key`, `renditions`, `failure_code`, `moderation_status` (`approved|removed`), `deleted_at` ;
  - outbox `outbox_events` ;
  - routes `POST /api/v1/videos/uploads`, `POST …/uploads/{id}/complete`, `DELETE …/uploads/{id}`, `GET /api/v1/videos`, `GET|DELETE /api/v1/videos/{id}` (`docs/api/video.openapi.yaml`) ;
  - limites de débit `init` 10, `complete` 20, général 60 (`internal/middleware/rate_limit.go`).
- **Cycle d'un envoi vidéo** :
  - `init` crée la ligne en `uploading` et signe une URL PUT par partie de 8 Mio. Taille exacte signée : le stockage refuse tout autre corps (`Storage.PresignPart`).
  - `complete` assemble les parties, vérifie les octets magiques, passe en `processing` et publie `poro.video.uploaded` dans la même transaction.
  - Rejoué, `complete` répond **409 `conflict`** (vérifié sur la pile réelle le 2026-10-24) ; c'est sur quoi repose la reprise sans doublon d'Android (PR #74).
  - Un envoi resté `uploading` 24 h est balayé (`AbandonedUploadAge`, `SweepAbandonedUploads`).
- **media-worker** (même module, `cmd/worker`) :
  - consomme `poro.video.uploaded` et publie `poro.video.ready` ou `poro.video.failed` ;
  - conteneur borné : 2 Go de mémoire, 2 CPU, 128 processus, système de fichiers en lecture seule, tmpfs ;
  - FFmpeg isolé (PORO-007).
- **Stockage** :
  - trois seaux : sources privées, médias publics, quarantaine privée ;
  - la quarantaine de modération (`Hide`/`Reveal`), la suppression et l'effacement de compte travaillent **par préfixe** `videos/{user}/{video}/` (`model.MediaPrefix`).
  - Les sources vidéo sont **conservées** après un traitement réussi : elles ne sont supprimées qu'au refus de `complete`.
- **Événements** (`packages/shared-go/events/catalog.go`) : `VideoUploadedV1`, `VideoReadyV1`, `VideoFailedV1`, `VideoDeletedV1`. Le commentaire de `VideoReadyV1` impose déjà l'ajout compatible : « consumers must accept events without them ».
- **Consommateurs de `poro.video.ready` et `poro.video.deleted`** :

  | Service | Ce qu'il garde |
  |---|---|
  | feed | table `videos` : `thumbnail_key`, `hls_key`, `duration_ms`… ; DTO `Item` avec `hls_url` et `thumbnail_url` |
  | social | `videos_projection` |
  | search | titre, description, hashtags, `thumbnail_key` |
  | moderation | analyse le **texte** seul |
  | notification | `video_ready` |

- **Caches du fil** :
  - `following` page 1, 60 s par lecteur et `limit` (clé `following:{viewer}:{limit}`) ;
  - `trending` page 1, 60 s ;
  - sessions `for-you` de 200 vidéos, 30 min.
- **Traitement d'images existant** : `user` (avatars) et `shop` (photos) passent par `sharp` (Node) avec rotation EXIF, plafond de 40 Mpx et WebP. Le service `video` est en Go, sans `sharp`.
- **Android** :
  - `UploadManager`, `UploadScheduler` et WorkManager envoient **une** vidéo à la fois, avec `UploadSessionStore` (reprise sans doublon) ;
  - `PresignedUploader.putPart` envoie un corps en PUT présigné ;
  - le fil lit `hls_url` (`FeedScreen`, `VideoScreen`) ;
  - aucun en-tête de capacité n'est envoyé aujourd'hui ;
  - l'en-tête `Idempotency-Key` n'est autorisé qu'en CORS à la gateway, aucun service ne l'utilise.

## Décisions

### 1. Modèle : une publication typée, même agrégat

1. **Type de média explicite.** `videos.media_type VARCHAR(10) NOT NULL DEFAULT 'video'`, contrainte `CHECK (media_type IN ('video','photo'))`. Toutes les lignes existantes deviennent `video` par défaut, sans réécriture des données. Aucune image n'est déguisée en vidéo et aucun faux fichier vidéo n'est créé.
2. **Images d'une publication photo** : table `video_images`.

   | Colonne | Type | Rôle |
   |---|---|---|
   | `video_id` | UUID, FK `videos(id)` ON DELETE CASCADE | publication |
   | `position` | SMALLINT, 1..10 | ordre d'affichage, PK (`video_id`, `position`) |
   | `content_type` | VARCHAR(30) | `image/jpeg`, `image/png` ou `image/webp` (déclaré) |
   | `size_bytes` | BIGINT, 1..10 485 760 | taille déclarée, signée dans l'URL |
   | `source_key` | VARCHAR(500) | `videos/{user}/{id}/source-{position}`, seau privé |
   | `image_key` | VARCHAR(500) NULL | `videos/{user}/{id}/image-{position}.jpg`, seau public, rempli au traitement |
   | `thumbnail_key` | VARCHAR(500) NULL | `videos/{user}/{id}/thumb-{position}.jpg` |
   | `width`, `height` | INT NULL | dimensions **après** orientation et réduction |

3. **Colonnes vidéo NOT NULL d'une photo.** Valeurs agrégées, pour ne toucher aucune contrainte existante :
   - `content_type = 'image/*'` (la colonne fait 50 caractères) ;
   - `size_bytes` = somme des tailles déclarées ;
   - `source_key` = préfixe `videos/{user}/{id}/` ;
   - `s3_upload_id`, `hls_key`, `duration_ms` et `renditions` restent vides ;
   - `videos.thumbnail_key` = miniature de la position 1. Le fil et la recherche ont ainsi une vignette sans changement.
4. **Texte.** `title` reste NOT NULL : il vaut `''` pour une photo et reste obligatoire (1 à 100 runes) pour une vidéo. La **légende facultative** est `description`, 0 à 500 runes, avec les mêmes règles de caractères (`Cc`, `Cf` refusés sauf U+200D entre deux émojis). Les hashtags sont extraits de la légende comme pour une vidéo.
5. **Quarantaine, suppression et effacement.** Les clés d'images sont sous `model.MediaPrefix(user, id)` : `Hide`, `Reveal`, la suppression et l'effacement de compte les couvrent **sans changement de code**. Un test le prouvera (§ 7).

### 2. Cycle de vie d'une publication photo

```
POST /photo-uploads ──► uploading ──(PUT ×N vers le stockage)──► POST …/complete ──► processing
       │                    │                                          │ 413 / 422 (taille ou
       │                    ├─ DELETE /uploads/{id} : annulée, sources │  type ≠ contenu) : sources
       │                    │  supprimées, ligne supprimée             │  et ligne supprimées, comme
       │                    └─ 24 h sans complete : balayée            │  pour une vidéo (dropRow)
       ▼                                                               ▼
                                    media-worker : toutes les images valides ──► ready (poro.video.ready)
                                                   une image invalide       ──► failed (poro.video.failed,
                                                                                code invalid_media)
ready ──► DELETE /videos/{id} : soft delete, médias en quarantaine, poro.video.deleted
ready ──► modération : removed (quarantaine) / restored (Reveal)
```

- **Aucune publication partielle.**
  - `complete` vérifie **toutes** les images avant de passer en `processing`.
  - Le worker n'écrit les médias publics qu'après avoir traité **toutes** les images, et ne publie `ready` qu'ensuite.
  - Si une image échoue, les objets publics déjà écrits sont supprimés et la publication passe en `failed`.
- **« Publié » n'est affiché qu'à `ready`.** Pour une photo, `processing` dure quelques secondes. L'app montre « En traitement » jusqu'à relire `ready`, exactement comme pour une vidéo.
- **Sources après traitement.** Contrairement aux vidéos, les **originaux photo sont supprimés** dès `ready` ou `failed`, car ils contiennent souvent la position GPS. Cela respecte la décision « sans conservation des métadonnées GPS ». Conséquence : pas de retraitement possible, il faut republier.

### 3. Routes (détail dans `docs/api/video.openapi.yaml`)

| Méthode et route | Rôle | Succès | Erreurs |
|---|---|---|---|
| `POST /api/v1/videos/photo-uploads` | crée la publication en `uploading` et renvoie une URL PUT présignée par image (taille et type signés, 1 h) | 201 | 400 `invalid_request` ; 401 ; 413 `payload_too_large` (une image déclarée au-delà de 10 Mo, comme une vidéo au-delà de 256 Mio, ou un corps JSON au-delà de 64 Kio) ; 422 `image_count_invalid`, `content_type_invalid`, `size_invalid` (taille ≤ 0), `description_invalid` ; 429 `rate_limited` |
| `PUT <url présignée>` | envoi direct au stockage, sans jeton | 200 + `ETag` | 403 du stockage (taille ou type différent, URL expirée) |
| `POST /api/v1/videos/photo-uploads/{id}/complete` | vérifie chaque objet (présence, taille déclarée, octets magiques), passe en `processing`, publie `poro.video.uploaded` | 200 `Video` (`processing`) | 400 `upload_incomplete` (image absente) ; 401 ; 404 `not_found` (autre propriétaire, inexistante, ou vidéo) ; **409 `conflict`** (n'est plus `uploading`, y compris un `complete` rejoué) ; 413 `payload_too_large` (objet stocké au-delà de 10 Mo) ; 422 `invalid_media` (contenu non JPEG/PNG/WebP, HEIC compris, ou objet vide). Sur 413 ou 422, sources et ligne sont supprimées, comme pour une vidéo (`dropRow`) : un `GET` répond ensuite 404 ; 429 |
| `DELETE /api/v1/videos/uploads/{id}` (existante) | annule une publication **vidéo ou photo** encore `uploading` ; pour une photo, supprime ses sources | 204 | 401 ; 404 ; 409 `conflict` |
| `GET /api/v1/videos/{id}` (existante) | ajoute `media_type` et `images[]` | 200 | 404 (non prête, retirée, ou photo demandée sans capacité, § 5) |
| `GET /api/v1/videos` (existante, ses propres publications) | ajoute `media_type` et `images[]` ; photos seulement avec la capacité (§ 5) | 200 | 401 |
| `DELETE /api/v1/videos/{id}` (existante) | inchangée pour une photo | 204 | 401 ; 404 |

- **Autorisations** : celles des routes vidéo.
  - Jeton d'accès, et tout compte PERSONAL peut publier.
  - Propriété vérifiée sur `user_id` : 404 et non 403 pour une publication d'un autre compte, comme aujourd'hui.
  - Compte suspendu : 401 par le registre de révocation (ADR-0019).
- **Débit** : `photo-uploads` compte dans la limite `init` (10/min), `…/complete` dans `complete` (20/min). Une publication photo coûte donc autant qu'une vidéo.
- **Validation à la création** :
  - 1 à 10 éléments dans `images` ;
  - chaque `content_type` dans {`image/jpeg`, `image/png`, `image/webp`} ;
  - chaque `size_bytes` entre 1 et 10 485 760 ;
  - `description` facultative (0 à 500 runes, caractères interdits refusés) ;
  - champs inconnus refusés (`additionalProperties: false`), comme pour la vidéo.
  - Un `content_type` HEIC ou HEIF déclaré est refusé dès la création (`content_type_invalid`). Un HEIC renommé en `.jpg` est refusé à `complete` sur ses octets magiques (`invalid_media`).

### 4. Envoi, reprises et idempotence

- **Une URL PUT par image**, sur le modèle de `PresignPart` : longueur et `Content-Type` signés. Le stockage refuse tout autre corps (403), donc un client ne peut pas stocker plus de 10 Mo par image ni changer de type.
- **Vérifié sur SeaweedFS 4.45** (`TestS3PresignedPutAgainstSeaweedFS`, joué par `infra/storage/tests/run.sh`) : un corps plus long ou plus court que signé, un autre `Content-Type` ou un `Content-Type` absent sont refusés en 403 et ne stockent rien ; le PUT conforme est accepté et peut être rejoué ; un objet jamais envoyé est lu comme `storage.ErrMissing`, distinct d'une panne du stockage.
- Nouvelle opération de stockage : `PresignPut(key, contentType, size)`. Les autres sont réutilisées : `HeadSource`, `GetSource`, `DeleteSource`, `PutMedia`, `Hide`, `Reveal`.
- **Réessai d'une image** : une même URL peut être rejouée tant qu'elle n'a pas expiré (1 h). Le PUT est idempotent : même clé, même contenu.
- **Idempotence de la publication**, sans nouvel en-tête : même mécanisme que la vidéo, déjà éprouvé par la PR #74.
  - Android mémorise `{video_id, empreinte}` avant tout envoi (`UploadSessionStore`).
  - Si la réponse de `complete` est perdue, réseau, 5xx ou 409 laissent la session en place et rien n'est annulé.
  - L'essai suivant relit `GET /videos/{id}` :
    - `processing` ou `ready` avec la même empreinte : succès, sans republier ;
    - `uploading` : annulation puis nouvel envoi.
  - L'empreinte d'une photo couvre : les URI et tailles de toutes les images dans l'ordre, et la légende.
- **URL expirées** (envoi de plus d'une heure) : 403 du stockage. Le client annule (`DELETE /uploads/{id}`) puis recommence. Il n'y a pas de route de re-signature, comme pour la vidéo.
- **Envois incomplets** : le balayage existant (24 h) est étendu aux photos. Il supprime leurs sources (`DeleteSource` par position) au lieu d'un `AbortMultipart`. Les orphelins sont donc bornés à 24 h et 100 Mo par publication.

### 5. Négociation de capacités : aucune photo pour un client qui ne les affiche pas

**Règle** : sans déclaration explicite, le serveur ne renvoie **que des vidéos**. C'est le comportement d'aujourd'hui, et donc celui de toute version déjà installée.

1. **Déclaration par le client.** En-tête `Poro-Capabilities`, liste de jetons séparés par des virgules, insensible à la casse. Les jetons inconnus sont ignorés. Premier jeton : `photo-posts`.
   - Android l'ajoute dans **un intercepteur OkHttp unique** des clients API (`NetworkModule`), et non appel par appel : aucun écran ne peut l'oublier.
   - Valeur portée par une constante, testée.
   - Pourquoi un en-tête plutôt qu'un paramètre `media_types` : il s'applique à toutes les routes sans changer leurs signatures, et ne se perd pas en recopiant un `next_cursor`.
2. **Filtrage par le serveur, jamais par le client** :

   | Route | Sans `photo-posts` | Avec `photo-posts` |
   |---|---|---|
   | `GET /api/v1/feed/for-you` | session construite avec les vidéos seules | vidéos et photos |
   | `GET /api/v1/feed/following` | vidéos seules | vidéos et photos |
   | `GET /api/v1/feed/trending`, `…/public/trending` | vidéos seules | vidéos et photos |
   | `GET /api/v1/videos` (ses publications) | vidéos seules | vidéos et photos |
   | `GET /api/v1/videos/{id}` | 404 pour une photo | photo |
   | `GET /api/v1/users/{userId}/likes` (social) | vidéos seules | vidéos et photos |
   | `GET /api/v1/search?type=videos` | vidéos seules | vidéos et photos |
   | `GET /api/v1/search?type=hashtags` | compte des vidéos seules | compte des vidéos et photos |

   - Social et search apprennent le `media_type` par `poro.video.ready`, comme les autres champs.
   - Notifications : `video_ready` d'une photo et les notifications de j'aime ou commentaire d'une photo restent **stockées**. `GET /api/v1/notifications` les omet pour un client sans `photo-posts`, et le compteur `unread-count` est calculé sur la même règle.
3. **Caches et sessions.**
   - L'ensemble des capacités entre dans **chaque clé de cache** et dans la clé de session `for-you`, par exemple `following:{viewer}:{limit}:v` ou `:vp`.
   - Les curseurs portent le même ensemble : un curseur obtenu avec photos et rejoué sans l'en-tête est refusé (400 `invalid_cursor`). Il n'est jamais servi.
4. **Pourquoi une ancienne version ne peut pas contourner le filtre** :
   - le filtre est appliqué par le serveur, et la valeur par défaut est « vidéos seules » ;
   - une ancienne version n'envoie pas l'en-tête et ne peut pas l'envoyer par erreur, puisqu'elle ne le connaît pas ;
   - les caches et curseurs sont cloisonnés par capacité (point 3).
5. **Gateway.** Traefik transmet les en-têtes de requête tels quels ; rien à changer, sauf ajouter `Poro-Capabilities` à `accessControlAllowHeaders` pour les navigateurs.

### 6. Traitement des images (media-worker)

- **Décodeurs Go, sans FFmpeg** :
  - bibliothèque standard `image/jpeg` et `image/png` ;
  - `golang.org/x/image/webp` pour le décodage ;
  - `golang.org/x/image/draw` (CatmullRom) pour la réduction.
- Pourquoi :
  - décodage sûr en mémoire de fichiers hostiles ;
  - comportement indépendant de la version de FFmpeg pour l'EXIF ;
  - testable sans binaire.
- C'est une nouvelle dépendance du module `video` : `golang.org/x/image`, maintenue par l'équipe Go.
- **Étapes par image**, toutes les images d'abord, puis l'écriture :
  1. Lire au plus `size_bytes` déclarés depuis la source ; au-delà, `invalid_media`.
  2. Détecter le format sur les **octets magiques**. JPEG, PNG et WebP sont acceptés. HEIC/HEIF (`ftypheic`, `ftypmif1`…), GIF, BMP, SVG et tout le reste sont refusés : `invalid_media`.
  3. `image.DecodeConfig` : refus si largeur × hauteur dépasse 40 000 000 ou si une dimension vaut 0, **avant** tout décodage complet (protection contre les bombes de décompression).
  4. Décodage complet, puis orientation EXIF (balise 0x0112, valeurs 1 à 8) lue par un petit lecteur EXIF du module (JPEG APP1, et chunk `eXIf` des PNG et WebP).
  5. Réduction au côté long de 2048 px au plus, sans agrandissement. Miniature au côté long de 540 px.
  6. Ré-encodage JPEG qualité 85. L'image est reconstruite pixel par pixel : **aucune métadonnée n'est recopiée**, ni EXIF, ni GPS, ni XMP, ni ICC. La transparence PNG ou WebP est aplatie sur fond blanc.
- **Mémoire du worker** : un décodage de 40 Mpx en RGBA occupe environ 160 Mo. Les images sont traitées **une par une**, ce qui reste loin de la limite de 2 Go. Un délai de 2 minutes par publication s'ajoute.
- `poro.video.failed` reprend le code existant `invalid_media` (`events.VideoFailInvalid`) : aucun nouveau code. Pour une photo, `failure_code` ne nomme jamais l'image fautive dans l'événement ; `last_error` la garde pour l'exploitation.

### 7. Événements : ajouts compatibles

- `VideoUploadedV1` reçoit `media_type` (`video` par défaut si absent).
- `VideoReadyV1` reçoit `media_type` et `images` : liste de `{image_key, thumbnail_key, width, height}` dans l'ordre. Pour une photo :
  - `thumbnail_key` = miniature 1 ;
  - `hls_key` vide, `duration_ms` 0, `renditions` vide ;
  - `width` et `height` = image 1.
- `VideoFailedV1` ne change pas : une image invalide utilise le code existant `invalid_media`. `VideoDeletedV1` ne change pas.
- **Tous les consommateurs traitent l'absence de `media_type` comme `video`**. Les événements déjà dans Redpanda et les DLQ restent lisibles.

| Consommateur | Changement |
|---|---|
| feed | colonnes `media_type` et `images JSONB` sur sa table `videos` ; DTO `Item` + `media_type`, `images[]` (`url`, `thumbnail_url`, `width`, `height`) ; `hls_url` vide pour une photo ; filtrage § 5 |
| social | `videos_projection.media_type`, pour filtrer `GET /users/{userId}/likes` ; j'aime, commentaires et partages inchangés (même identifiant) |
| search | colonne `media_type`, renvoyée dans les résultats `videos` ; filtrage § 5 ; recherche sur la légende et les hashtags (le titre est vide) |
| notification | libellé `video_ready` propre aux photos (« Votre publication photo est en ligne ») ; filtrage § 5 |
| moderation | aucun : le texte (légende, hashtags) est analysé. **Les images ne sont pas analysées automatiquement**, comme les images des vidéos aujourd'hui. Le signalement (`target_type: video`, même identifiant) et la quarantaine par préfixe couvrent les photos. |

### 8. Migrations envisagées (non exécutées)

- `services/video/migrations/000003_photo_posts.up.sql` :
  - `ADD COLUMN media_type` avec sa contrainte CHECK ;
  - `CREATE TABLE video_images` ;
  - pas d'index supplémentaire nécessaire, la PK (`video_id`, `position`) suffit.
  - Le `.down.sql` supprime la table et la colonne. **Il refuse de s'exécuter tant qu'une publication photo existe**, même supprimée : la contrainte `rollback_000003_needs_no_photo_post` ne peut pas être posée, et rien n'est modifié. golang-migrate marque alors la base « dirty » : l'opérateur purge les publications photo (lignes et objets), force la version (`migrate force 3`), puis relance le retour arrière. Testé par `TestPhotoPostsMigration` sur des vidéos de chaque état.
- `services/feed/migrations/000003_media_type.up.sql` : `media_type` sur `videos` (défaut `video`) et `images JSONB NOT NULL DEFAULT '[]'`. Les index existants filtrent sur `deleted_at` et `moderation_status`. Le filtre `media_type = 'video'` s'ajoute aux requêtes, et un index partiel `WHERE media_type = 'video'` est à mesurer avant d'être ajouté.
- `services/social/migrations/000004_media_type.up.sql` : `videos_projection.media_type`.
- `services/search/migrations/000004_media_type.up.sql` : `media_type` sur sa table `videos`.
- Toutes ces migrations sont additives avec une valeur par défaut. Le déploiement suit cet ordre :
  1. consommateurs et migrations ;
  2. service `video` ;
  3. Android.

  Un événement photo n'est donc jamais lu par un consommateur qui l'ignorerait.

### 9. Implications des règles

| Sujet | Effet |
|---|---|
| **Mémoire Android** | Aucun décodage complet n'est nécessaire pour envoyer : lecture de l'image par `ContentResolver`, 10 Mo au plus en mémoire, une image à la fois (`putPart(url, ByteArray)`). Les aperçus passent par Coil à taille d'affichage, jamais en pleine résolution : 40 Mpx en ARGB occuperaient 160 Mo. La conversion HEIC passe par `ImageDecoder` (Android 9+) avec une **taille cible de 2048 px** sur le côté long, soit environ 12 Mo de pixels, puis JPEG qualité 90. |
| **HEIC sur Android** | `ImageDecoder` décode le HEIC d'Android 9 (API 28) et plus, quand le codec HEVC est présent. Si le décodage échoue, ou sous Android 9 (minSdk 24), l'image est refusée avec un message explicite : « Format HEIC non pris en charge : choisissez une autre photo ou convertissez-la en JPEG. » Aucune conversion n'est simulée : l'image convertie est relue (type et taille réels) avant l'envoi. |
| **Bande passante** | Au pire, 10 × 10 Mo = 100 Mo par publication. Cas courant : une photo de téléphone de 12 Mpx fait 3 à 5 Mo en JPEG, soit 30 à 50 Mo pour 10 images. **Décidé** : Android réduit à 2048 px (JPEG qualité 92) les images plus grandes avant l'envoi, pour descendre à 0,6 à 1,2 Mo par image. Il n'y a aucune perte visible, puisque le serveur réduit de toute façon à 2048 px. Le serveur ne s'y fie pas : il revérifie et ré-encode tout. |
| **Stockage serveur** | Sortie d'environ 0,4 à 1 Mo par image (2048 px, qualité 85), plus une miniature d'environ 40 Ko : au plus environ 10 Mo par publication de 10 images. Les originaux (jusqu'à 100 Mo) sont supprimés après traitement (§ 2), et la quarantaine garde les seuls médias traités. |
| **Worker** | Environ 160 Mo de pointe par image de 40 Mpx, traitement séquentiel. Il partage le conteneur et la file avec le transcodage vidéo : une rafale de photos peut retarder des vidéos. Une file dédiée est à envisager si la charge le justifie. |

### 10. Compatibilité et limites

- Aucune version d'Android n'est publiée sur une boutique : la release est BLOQUÉE faute de keystore. La négociation (§ 5) protège quand même les versions de développement installées.
- Une ancienne version ne verra jamais une photo d'un autre compte. Si son propriétaire publie une photo depuis une nouvelle version, l'ancienne version ne la voit pas non plus dans « Mes vidéos » : c'est voulu, car elle ne saurait pas l'afficher.
- Un lien partagé vers une photo (`SHARE_BASE_URL`) ouvert par une ancienne version donne 404 (§ 5).
- Les images ne sont pas analysées par la modération automatique (§ 7).

## État d'avancement

### Étape 2b (service `video`, PR #75)

- Migration `000003_photo_posts`, routes `POST /api/v1/videos/photo-uploads` et `…/{id}/complete`, PUT présigné, annulation, balayage, effacement de compte et filtre de capacité sur `GET /api/v1/videos` et `GET /api/v1/videos/{id}`.
- **Routes photo fermées par défaut à cette étape** : `PHOTO_POSTS_ENABLED` (service `video`, défaut `false` jusqu'à l'étape 2d, puis `true`).
  - Fermées, `POST /api/v1/videos/photo-uploads` et `…/{id}/complete` répondent 404 `photo_posts_disabled` avant toute authentification ou limite de débit, et n'acceptent rien.
  - Les routes vidéo n'en dépendent pas.
  - L'annulation (`DELETE /uploads/{id}`), la lecture et la suppression restent ouvertes, pour qu'aucun objet déjà stocké ne reste bloqué.
  - Une publication laissée `uploading` quand la route se ferme est balayée après 24 h avec ses sources.

### Étape 2c (media-worker)

- Le worker traite les photos (paquet `internal/imaging`, décodeurs Go et `golang.org/x/image`). Le garde-fou de l'étape 2b (événement envoyé en DLQ) est retiré. Un événement qui annonce une photo pour une ligne vidéo reste refusé (DLQ) : c'est la ligne qui fait foi.
- **Revalidation au traitement**, exigée car l'URL PUT reste valide 1 h. Chaque source est relue, au plus 10 Mo + 1 octet. Puis, **avant tout décodage**, le worker revérifie :
  - la taille déclarée ;
  - les octets magiques ;
  - le type déclaré ;
  - les 40 Mpx, lus dans l'en-tête.

  Une image remplacée après `complete` fait échouer toute la publication.
- **Tout ou rien** :
  - les images sont toutes traitées en mémoire avant la première écriture publique ;
  - le passage à `ready`, l'enregistrement des images et `poro.video.ready` (avec `media_type` et `images`) sont dans la transaction de la réclamation de l'événement ;
  - une panne du stockage pendant l'écriture fait échouer la publication et met en quarantaine ce qui était déjà écrit ;
  - une publication supprimée pendant le traitement ne devient jamais `ready`.
- **Sources supprimées** après `ready`, après `failed` et après un retrait. Un échec de suppression est journalisé : l'objet reste privé, et l'effacement du compte le supprime.
- **Sortie** : JPEG au côté long de 2048 px au plus (arrondi au pixel), qualité 85, et miniature de 540 px, qualité 80. Transparence aplatie sur blanc. Aucune métadonnée conservée : ni EXIF, ni GPS, ni XMP, ni ICC.
- **Événements photo envoyés en DLQ pendant l'étape 2b.** C'est seulement possible là où le drapeau aurait été ouvert. Ils sont dans `poro.video.uploaded.dlq` (rétention 30 jours en Compose, à fixer en production avec PORO-010). Aucun outil de rejeu n'existe (PORO-018) ; inspection : `rpk topic consume poro.video.uploaded.dlq -o start`. Conditions de rejeu, jamais en aveugle :
  1. ce worker est déployé ;
  2. seuls les événements dont `data.media_type` vaut `photo` sont republiés, avec leur **identifiant d'origine**, vers `poro.video.uploaded` ;
  3. la publication est encore `processing` et non supprimée : sinon le worker l'ignore.

  Le rejeu est idempotent, car chaque identifiant d'événement n'est traité qu'une fois, et la revalidation s'applique comme pour un premier traitement.
- Le drapeau est resté fermé jusqu'à l'étape 2d : une photo prête publie `poro.video.ready`, et les consommateurs ne savaient pas encore réserver les photos aux clients `photo-posts`.

### Étape 2d (feed, search, social, notification)

- **Analyse de l'en-tête partagée** : `packages/shared-go/capability` (Go) et `services/notification/src/common/capabilities.ts`. Jetons séparés par des virgules, insensibles à la casse ; jeton inconnu ignoré ; sans en-tête, vidéos seules.
- **feed** (migration `000003_photo_posts`) :
  - colonnes `media_type` (défaut `video`) et `images JSONB` (défaut `[]`) ; un événement sans `media_type` est une vidéo, une photo sans images ou un type inconnu part en DLQ ;
  - les six requêtes (Abonnements, Tendances, hydratation de « Pour toi » et ses trois sources de candidats) filtrent `media_type = ANY(...)` ;
  - clés de cache de page 1 et clés de session « Pour toi » suffixées par l'ensemble de capacités (`v` ou `vp`) ;
  - curseurs : ceux des clients `photo-posts` portent un type distinct (`tp`, `sp`, `fp`) et sont refusés sans l'en-tête (400 `invalid_cursor`), et inversement ; ceux des vidéos seules gardent leur forme, donc aucun curseur en cours n'est invalidé au déploiement ;
  - DTO `Item` : `media_type`, `images[]` (`url`, `thumbnail_url`, `width`, `height`, toujours un tableau), `hls_url` vide pour une photo.
  - une photo n'a pas de titre : sa légende (200 caractères au plus, coupée par « … ») est projetée à la place, car l'élément du fil ne porte pas de description.
- **social** (migration `000004_photo_posts`) : `videos_projection.media_type` ; `GET /users/{userId}/likes` filtre par capacité, curseur préfixé `p.` avec photos. J'aime, commentaires et partages d'une photo utilisent les mêmes routes.
- **search** (migration `000004_photo_posts`) : `videos.media_type` et `hashtags.photos_count`, tenu à jour à chaque changement de visibilité comme `videos_count`. Sans l'en-tête : résultats `videos` sans photo, hashtags comptés sur les vidéos seules, tag porté uniquement par des photos absent (recherche et suggestions). Curseurs `videos` et `hashtags` distincts par capacité. La légende d'une photo est cherchée comme une description.
- **notification** (migration `20261025120000_photo_posts`) :
  - table `photo_posts`, remplie par `poro.video.ready` d'une photo, et colonne `notifications.photo_post` ;
  - liste et `unread-count` filtrés par capacité ; les notifications restent stockées ;
  - libellés : « Ta publication photo est en ligne », « … a aimé ta publication », « … a commenté ta publication » ; `data.media_type: photo` pour le routage côté app ;
  - course entre sujets Kafka : un j'aime traité avant le `ready` de sa photo est rattrapé au `ready` (drapeau, `media_type`, libellé). Les curseurs de notification sont des positions de clé sur la requête déjà filtrée : rejoués sans l'en-tête, ils ne renvoient aucune photo.
- **moderation** : aucun changement, le texte analysé comprend déjà la description (légende) et les hashtags.
- **Gateway** : `Poro-Capabilities` ajouté aux en-têtes CORS autorisés.
- **`PHOTO_POSTS_ENABLED` passe à `true` par défaut** (décision du propriétaire du 2026-10-25) : config, `docker-compose.yml` et `.env.example`. `false` referme les deux routes de création (404 `photo_posts_disabled`) sans toucher aux vidéos.
- **Tests** : Go sur Postgres réel (testcontainers) pour feed, social et search, dont un test par service vérifiant qu'aucune photo n'atteint un client sans l'en-tête, en base comme en cache, et le refus des curseurs croisés ; e2e Jest pour notification (j'aime avant `ready` compris) ; parcours e2e « photo posts across feed, search, social, notifications » sur la pile complète.

## Tests contractuels et critères d'acceptation

| Étape | Tests | Critère d'acceptation |
|---|---|---|
| **2b** : service `video` (migration, routes, PUT présigné, `complete`, annulation, balayage, filtre de capacité sur `GET /videos` et `/videos/{id}`) | Go, `model` : 0 et 11 images ; 10 485 760 et 10 485 761 octets ; HEIC déclaré ; légende de 501 runes ; caractères `Cf`. Go, testcontainers + gofakes3 : création → URL signées (taille et type) ; `complete` sans objet → 400 `upload_incomplete` ; objet de 10 Mo + 1 → 413 `payload_too_large` ; PNG déclaré JPEG ou HEIC renommé → 422 `invalid_media`, sources et ligne supprimées puis `GET` → 404 ; taille déclarée 0 → 422 `size_invalid`, 10 Mo + 1 → 413 ; `complete` rejoué → 409 ; `DELETE /uploads/{id}` supprime toutes les sources ; balayage à 24 h ; `GET /videos` sans en-tête ne renvoie aucune photo ; `GET /videos/{id}` d'une photo sans en-tête → 404 ; publication d'un autre compte → 404. **Les tests vidéo existants passent sans modification.** | `go test ./...` vert, couverture au moins égale au seuil du module ; contrat OpenAPI identique au comportement (test de contrat sur les codes) |
| **2c** : media-worker | Fixtures : JPEG en orientations 1 à 8 (dimensions et coin témoin vérifiés après rotation) ; JPEG avec GPS (APP1 absent en sortie, aucun octet `GPS`) ; PNG transparent ; WebP ; HEIC ; faux JPEG ; JPEG tronqué ; 8000 × 6000 (48 Mpx, refusé avant décodage) ; 2049 px → 2048 ; 800 px non agrandi ; 3 images valides + 1 invalide → `failed`, aucun objet public restant | `poro.video.ready` émis seulement quand toutes les images sont traitées ; sources supprimées après `ready` et `failed` |
| **2d** : consommateurs | feed : événement sans `media_type` → vidéo ; événement photo → `images` ; pour chacune des 4 routes du fil, **sans en-tête aucune photo**, avec en-tête photos présentes ; clés de cache et de session distinctes ; curseur avec photos rejoué sans en-tête → 400. Social : likes filtrés. Search : `type=videos` filtré. Notification : liste et compteur filtrés, libellé photo. Moderation : un événement photo est analysé sur sa légende. | Test « fuite » commun : sur une base contenant une photo, **toutes** les routes listées au § 5, appelées sans en-tête, ne renvoient aucun `media_type: photo` |
| **3** : Android, mode Photo | JVM : règles (10 images, 10 Mo, types) ; HEIC converti (fake décodeur) ou refusé avec le message ; ajout et retrait ; légende vide acceptée ; double appui = un envoi ; réessai sans doublon (réponse de `complete` perdue) ; perte d'accès à une image → re-sélection de cette image seule, légende gardée ; intercepteur `Poro-Capabilities` présent sur les appels API et absent des URL présignées. Instrumenté : mode Photo, aperçus, retrait. | « Publié » seulement à `ready` relu du serveur |
| **4** : Android, affichage | Instrumenté : carrousel (glissement, indicateur « 2/5 », description d'accessibilité par image) dans le fil, le profil et le détail ; j'aime, commentaire et signalement sur une photo | Photo visible après redémarrage de l'app, relue depuis le serveur |
| **5** : e2e et téléphone | `tests/e2e` : publier 2 JPEG (orientation 6 + GPS) → `ready` ; télécharger l'image 1 : largeur et hauteur échangées, aucun EXIF ; elle apparaît dans `for-you`, `following`, `trending` **avec** l'en-tête et dans **aucune** de ces routes sans l'en-tête ; j'aime, commentaire, signalement ; suppression → images plus servies (404 ou 403 du stockage) ; refus 11 images, 10 Mo + 1, HEIC, PNG déclaré JPEG ; double `complete` → 409 ; la vidéo reste couverte comme aujourd'hui | Tous les jobs CI verts ; procédure sur téléphone réel jouée (photo publiée, relue après redémarrage, coupure réseau sans doublon) |

## Conséquences

- Le parcours vidéo ne change ni de route, ni de colonne, ni d'événement existant : seuls des champs facultatifs et des filtres s'ajoutent.
- Un client doit déclarer `photo-posts` pour voir des photos ; c'est la seule obligation nouvelle pour Android.
- Les originaux photo ne sont pas conservés : pas de retraitement a posteriori.
- Les photos ne sont pas modérées automatiquement. Il faudra une analyse d'images dédiée avant une ouverture large.
