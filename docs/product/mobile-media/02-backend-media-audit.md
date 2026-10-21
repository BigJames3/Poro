# Audit backend — médias

Faits tirés de `docs/api/*.openapi.yaml`, ADR-0007, ADR-0015, et du code des services `video`, `shop`, `user`. Aucune limite n’est inventée.

## 1. Ce que le backend sait faire

| Domaine | Médias | Rôle |
|---|---|---|
| Vidéo | Clip court → HLS + miniature JPEG | Publication sociale, lecture |
| Social / feed / search | Références `video_id` uniquement | Like, commentaire, share, fil |
| User | Avatar JPEG/PNG/WebP → WebP 512×512 | Profil |
| Shop | Logo + photos produit JPEG/PNG/WebP → WebP | Marketplace |
| Order | `image_url` en lecture (copie catalogue) | Ligne de commande |

Il n’existe **aucun** type de publication sociale « image seule », ni de champ vidéo sur un produit.

## 2. Publication vidéo (import social)

| Étape | Route | Auth | Effet |
|---|---|---|---|
| Init | `POST /api/v1/videos/uploads` | Bearer | Statut `uploading` ; URLs PUT présignées (parties 8 Mio) |
| Parties | `PUT` sur chaque URL | Signature S3 | Garder l’`ETag` |
| Fin | `POST /videos/uploads/{id}/complete` | Bearer | Magic bytes ; `processing` ; événement `poro.video.uploaded` |
| Abandon | `DELETE /videos/uploads/{id}` | Bearer | 204 si encore `uploading` |
| Suivi | `GET /videos/{id}` | Public si `ready`+`approved` | `hls_url`, `thumbnail_url` |
| Liste proprio | `GET /videos` | Bearer | Tous statuts |
| Suppression | `DELETE /videos/{id}` | Propriétaire | Soft-delete ; médias publics → quarantaine |

### Limites serveur (code `services/video/internal/model`)

| Contrainte | Valeur | Refus |
|---|---|---|
| MIME déclaré | `video/mp4`, `video/quicktime`, `video/webm` | `content_type_invalid` |
| Taille | 1 … 268 435 456 octets (256 Mio) | `size_invalid` / `payload_too_large` |
| Durée | ≤ 180 s (probe FFmpeg, pas le client) | `too_long` après traitement |
| Conteneur réel | MP4/MOV (`ftyp`) ou WebM (EBML) | `invalid_media` au complete |
| Codecs vidéo | h264, hevc, mpeg4, vp8, vp9, av1 | `invalid_media` |
| Codecs audio | aac, mp3, opus, vorbis | `invalid_media` |
| Dimensions | 2 … 4096 px par côté | `invalid_media` |
| Titre | 1–100 runes | `title_invalid` |
| Description | ≤ 500 runes | `description_invalid` (OpenAPI dit 1000 : écart) |
| Rate init | 10 / min / compte | `429` |
| Rate complete | 20 / min / compte | `429` |
| Rate général | 60 / min / compte | `429` |

Toute personne `PERSONAL` authentifiée peut uploader. `CREATOR` n’est pas exigé.

Statuts publics d’échec : `too_long`, `invalid_media`, `transcode_failed`. stderr FFmpeg jamais renvoyé.

## 3. Lecture et URLs vidéo

Schéma `Video` : `hls_url`, `thumbnail_url`, `renditions[]` (nom, bandwidth, width, height). Pas d’URL MP4, pas de `download_url`, pas de `Content-Disposition: attachment`.

| Statut | Qui voit les URLs | Comportement |
|---|---|---|
| `ready` + `approved` | Tout le monde (GET public) | HLS + miniature |
| `uploading` / `processing` / `failed` | Propriétaire seulement | Sinon 404 |
| `moderation_status: removed` | Propriétaire : sans URL média | Tiers : 404 |

HLS : `videos/{user_id}/{video_id}/hls/master.m3u8` dans `poro-videos` (lecture anonyme GET/HEAD). Échelle 360p toujours ; 720p si la source a ≥ 720 lignes. Miniature JPEG.

**L’original** (`source`) est dans `poro-video-sources` (**privé**). Sa clé se déduit des identifiants mais un GET anonyme est refusé (ADR-0015). Un client mobile **ne peut pas** télécharger le fichier que l’auteur a envoyé.

Enregistrer « la vidéo » impliquerait de récupérer tous les segments HLS puis de les remuxer. Ce n’est pas un téléchargement HTTP unique. Aucun endpoint ne le fait. **Non vérifié côté Android.**

## 4. Images boutique

| Action | Route | Auth | Limite dédiée |
|---|---|---|---|
| URL logo | `POST /shops/me/logo/upload-url` | Bearer | 10 / min |
| Confirmer logo | `PUT /shops/me/logo` | Bearer | 10 / min |
| Retirer logo | `DELETE /shops/me/logo` | Bearer | API **sans UI** Android |
| URL photo | `POST /shops/me/products/{id}/images/upload-url` | Bearer | 30 / min |
| Confirmer photo | `PUT /shops/me/products/{id}/images` | Bearer | 30 / min |
| Retirer photo | `DELETE .../images/{imageId}` | Bearer | Produit `active` : au moins une |

Rate global shop : 120 / min / compte (ou IP si anonyme).

### Limites serveur (`services/shop/src/media/media.service.ts`, `product.dto.ts`)

| Contrainte | Valeur | Code |
|---|---|---|
| MIME | `image/jpeg`, `image/png`, `image/webp` | 422 si autre à l’init |
| Taille originale | 1 … 5 Mio (`IMAGE_MAX_BYTES`) | `image_too_large` |
| TTL formulaire | 300 s | POST S3 expiré |
| Formats réels (sharp) | jpeg, png, webp | `image_invalid` (GIF, SVG, HTML refusés) |
| Pixels d’entrée | ≤ 40 000 000 | `image_invalid` |
| Logo publié | WebP 512×512, recadrage `cover`, EXIF retiré | — |
| Photo publiée | WebP, grand côté ≤ 1080, ratio conservé, sans agrandissement | — |
| Photos / produit | 10 (`MAX_IMAGES`) | `too_many_images` |
| Publication `active` | ≥ 1 photo | `image_required` |

Le client envoie l’original en **POST multipart présigné** (champs puis `file`) vers `poro-shop-uploads` (privé). Après confirm, l’objet public est un **nouveau** WebP dans `poro-shop`. L’original est supprimé.

Champs publics : `logo_url`, `images[].image_id` + `url`. Aucun `video_url`, `hls_url` ni `content_type` vidéo.

## 5. Avatar

| Action | Route | Auth | Limite |
|---|---|---|---|
| URL | `POST /users/me/avatar/upload-url` | Bearer | 10 / min |
| Confirmer | `PUT /users/me/avatar` | Bearer | 10 / min |
| Retirer | `DELETE /users/me/avatar` | Bearer | — |

Même trio MIME, 5 Mio, 300 s. Ré-encodage 512×512 WebP, EXIF/GPS retirés. Codes : `avatar_upload_invalid`, `avatar_not_uploaded`, `avatar_too_large`, `avatar_invalid`.

Ce n’est **pas** une publication sociale.

## 6. Stockage (ADR-0015)

| Bucket | Visibilité anonyme | Contenu |
|---|---|---|
| `poro-videos` | GET/HEAD objet | HLS + miniatures |
| `poro-video-sources` | Aucune | Originaux vidéo |
| `poro-quarantine` | Aucune | Médias retirés / supprimés |
| `poro-avatars` | GET/HEAD | Avatars WebP |
| `poro-avatar-uploads` | Aucune | Originaux avatar (éphémères) |
| `poro-shop` | GET/HEAD | Logos et photos WebP |
| `poro-shop-uploads` | Aucune | Originaux image (éphémères) |

Lecture publique ≠ liste, écriture, ni création de bucket. Bunny CDN **absent** (PORO-029). Les URLs pointent vers SeaweedFS (dev) ou l’endpoint S3 public.

## 7. Ce qui n’existe pas

| Capacité | Statut |
|---|---|
| `GET /videos/{id}/download` | Absent |
| Rendition MP4 progressive | Absent (HLS seulement) |
| Flag `allow_download` / DRM | Absent |
| Post image social | Absent |
| Vidéo sur produit / boutique | Absent |
| Upload image vers le service vidéo | Absent |
| Lecture hors ligne côté API | Absent |
| Partage de fichier (octet stream) | Absent — share = compteur + texte |

## 8. Conséquences pour l’UI

1. Importer une **vidéo** sociale : contrat complet, déjà utilisé.
2. Importer / afficher une **image** sociale : **cible produit**, **bloqué** (pas de route, B-IMG-POST).
3. Importer une image boutique / avatar : contrat complet, déjà utilisé.
4. Enregistrer une **vidéo** : pas de fichier unique public ; original privé → **bloqué**.
5. Enregistrer une **image déjà publique** (WebP / JPEG miniature) : GET anonyme possible. Pas d’autorisation métier distincte : c’est la même chose qu’ouvrir l’URL dans un navigateur.
6. Ne jamais afficher, journaliser ni persister `upload_url`, `fields`, `upload_key`, `upload_id` ni les URLs présignées.
