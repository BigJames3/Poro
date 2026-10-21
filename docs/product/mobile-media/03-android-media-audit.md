# Audit Android — médias

Lecture seule de `apps/mobile-android` au 21 octobre 2026. `applicationId` `com.poro.app`, `minSdk` 24, `targetSdk` 37.

## 1. Permissions

`AndroidManifest.xml` déclare **uniquement** `INTERNET`.

Absents : `READ_EXTERNAL_STORAGE`, `WRITE_EXTERNAL_STORAGE`, `READ_MEDIA_IMAGES`, `READ_MEDIA_VIDEO`, `READ_MEDIA_VISUAL_USER_SELECTED`, `CAMERA`, `RECORD_AUDIO`, `POST_NOTIFICATIONS`, `FOREGROUND_SERVICE*`.

Les sélections passent par **Photo Picker** (`ActivityResultContracts.PickVisualMedia`) : aucune permission de stockage n’est requise, y compris sur API 24.

## 2. Où l’on choisit un fichier

| Écran | Picker | Filtre | Destination |
|---|---|---|---|
| `UploadScreen` | `PickVisualMedia` | **`VideoOnly`** | `POST /videos/uploads` |
| `ProductEditorScreen` | `PickVisualMedia` | **`ImageOnly`** | photos produit |
| `MyShopScreen` | `PickVisualMedia` | **`ImageOnly`** | logo |
| `EditProfileScreen` | `PickVisualMedia` | **`ImageOnly`** | avatar |

Aucun `GetContent`, `OpenDocument`, `TakePicture` ou `CaptureVideo`. Pas de caméra in-app.

`ImageAndVideo` n’est utilisé nulle part. Les écrans boutique n’embarquent pas `VideoPlayer` / Media3 (recherche `ui/shop/` : `AsyncImage` seulement).

## 3. Contrôles locaux avant envoi

`LocalMediaReader` / `ContentResolverMediaReader` : MIME + taille via `OpenableColumns` ; durée vidéo via `MediaMetadataRetriever`. Lecture par **morceaux** (vidéos 256 Mio jamais chargées en entier).

| Surface | MIME acceptés | Taille | Autre | String d’erreur |
|---|---|---|---|---|
| Vidéo | `video/mp4`, `video/quicktime`, `video/webm` | 256 Mio | durée ≤ 3 min | `upload_error_format` / `_size` / `_duration` |
| Produit / logo | `image/jpeg`, `image/png`, `image/webp` | 5 Mio | — | `error_image_invalid` (format **et** taille mêlés) |
| Avatar | mêmes images | 5 Mio | — | `error_avatar_invalid` |

HEIC/HEIF, GIF, BMP, SVG, AVI : refusés **avant** l’upload si le MIME système n’est pas dans la liste. Un iPhone qui expose `image/heic` échoue avec le message image générique. Ne pas « corriger » en acceptant HEIC : le serveur le refuserait au sharp.

Images boutique et avatar : `media.read(uri, 0, size)` charge **tout le fichier** (plafond 5 Mio). Acceptable. Interdit pour une vidéo.

## 4. Publication vidéo — déjà en place

`UploadManager` (portée application) :

| État | UI actuelle |
|---|---|
| `Idle` | Formulaire |
| `Uploading` | % + barre + « Gardez l’application ouverte jusqu’à la fin de l’envoi. » |
| `Processing` | Attente transcodage (poll croissant ~30 min) |
| `Finished` ready | « Votre vidéo est en ligne ! » + Voir |
| `Finished` failed | `video_failed` |
| `Failed` | `error.code` + possibilité de republier |

Une seule publication à la fois (`isBusy` → `start` retourne false). L’utilisateur peut quitter l’écran. **L’envoi meurt avec le processus** (pas de WorkManager). C’est déjà écrit dans le code et le lot 5 phase 2 : cette spec ne le promet pas.

Onglet libellé **Publier** (`tab_upload`), titre **Publier une vidéo**. Vocabulaire déjà correct (pas « Télécharger »).

## 5. Images boutique / avatar — écarts UX

| Besoin exigence | État réel |
|---|---|
| Picker images seulement | OK |
| Plafond 10 photos | OK (`MAX_PRODUCT_IMAGES = 10`) |
| Contrôle type + taille avant envoi | OK, mais une seule string pour les deux causes |
| Aperçu des images **déjà** confirmées | OK (`AsyncImage` 96 dp éditeur ; pager 1:1 PDP) |
| Aperçu **local** avant envoi | **Absent** : pick → upload immédiat |
| Retirer avant envoi | **Absent** (on retire une photo **déjà** publiée) |
| Remplacer | Absent (supprimer + ajouter) |
| Progression d’envoi image | Spinner `working` global, pas de % |
| Photos avant création du produit | Impossible : `productId` requis par l’API |
| Lecteur vidéo boutique | Absent — à **conserver** |
| Enregistrer une image dans la galerie | **Absent** |

`DELETE /shops/me/logo` : API sans bouton (déjà noté phase 2, P2).

## 6. Lecture sociale

`VideoPlayer` : Media3 / ExoPlayer, HLS `hls_url`, miniature Coil jusqu’à la première frame, boucle, tap pause. Une vidéo décodée à la fois.

Absents : mute (P1), barre de seek, hors ligne, bouton Enregistrer, cache utilisateur.

Overlay (`VideoActions`) : like, commentaires, partage texte, ⋮ Signaler. **Pas d’entrée Enregistrer.**

Partage : `Intent.ACTION_SEND` `text/plain`. Sans `SHARE_BASE_URL`, pas de lien. Jamais le fichier média.

## 7. Enregistrement local

Aucune occurrence de `DownloadManager`, `MediaStore` insert, `ContentValues`, `WRITE_EXTERNAL`, `saveTo` dans le module app.

Pas de service de premier plan, pas de WorkManager.

## 8. Images dans le social aujourd’hui

Ce que l’utilisateur **voit** (affichage Coil) :

- Miniatures vidéo (`thumbnail_url`, JPEG public)
- Avatars (WebP public)

Ce qu’il **ne peut pas** faire :

- Publier une image dans le fil
- Ouvrir une « photo sociale » distincte de la vidéo
- Enregistrer miniature ou avatar

Le profil public n’a **pas** de grille de médias (API absente).

## 9. Implications minSdk 24 pour un futur Enregistrer

Si Q-MED-01 active l’enregistrement d’images publiques :

| API | Écrire dans la galerie (MediaStore Images) |
|---|---|
| 24–28 | `WRITE_EXTERNAL_STORAGE` (demander au tap, `maxSdkVersion=28`) |
| 29–32 | MediaStore insert **sans** permission (stockage scopé) |
| 33+ | Idem ; `READ_MEDIA_*` seulement pour **lire** la galerie d’autrui, inutile ici |

Ne pas ajouter `READ_MEDIA_*` « au cas où ». Ne pas demander la permission au lancement.

Photo Picker reste sans permission, y compris après ajout d’un Enregistrer.
