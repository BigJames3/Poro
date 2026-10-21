# Bloquants et décisions — médias

## 1. Bloquants techniques (ne pas simuler)

| ID | Capacité demandée | Pourquoi c’est bloqué | Que faire |
|---|---|---|---|
| B-VID-SAVE | Enregistrer une **vidéo** sociale | Pas d’URL de fichier unique. `hls_url` = playlist. L’original est dans `poro-video-sources` (privé, ADR-0015). Pas de remux HLS vérifié dans l’app. | Cacher l’action. Pour débloquer : rendition MP4 publique **ou** spike Media3 Transformer + ADR. |
| B-IMG-POST | Afficher / publier une **image** dans le fil | Cible produit = vidéo **ou** image. Contrat actuel = `video_id` + HLS seulement. Like/commentaire/share/report/search sont scopés vidéo. | Pas de picker photo, pas de carte fake. Débloquer : OpenAPI + ADR (voir [04](04-social-media-spec.md) §3), puis lot M5. |
| B-OFFLINE | Lecture hors ligne | Pas de manifeste de cache, pas d’API offline, ExoPlayer sans cache utilisateur spécifié. | Ne pas afficher « Disponible hors ligne ». |
| B-BG | Téléchargement / envoi après kill | Pas de WorkManager. `UploadManager` le documente déjà. | Garder « Gardez l’application ouverte ». Lot 5 phase 2 inchangé. |
| B-FILE-SHARE | Partager le fichier média | Share = `text/plain` (+ URL si `SHARE_BASE_URL`). | Ne pas ajouter `video/*` / `image/*` à l’Intent. |
| B-AND | Tout Kotlin | Autorisation explicite absente pour **cette** phase | Spec seulement |
| B-LEGAL | Activer Enregistrer une photo (Q-MED-01) | TOS / droits d’auteur non validés (Q-03) | Défaut : ne pas livrer M3 |

## 2. Décisions à trancher

| ID | Question | Impact | Reco spec |
|---|---|---|---|
| Q-MED-01 | Permettre d’enregistrer une **photo produit** publique dans la galerie ? | Lot M3 | **Non** tant que Q-03 / Q-MED-02 ouverts. Techniquement oui (GET WebP). |
| Q-MED-02 | L’enregistrement d’un média public est-il autorisé par les TOS ? | M3, copies | Juridique. L’API n’a pas de flag ; le bucket public le permet déjà via navigateur. |
| Q-MED-03 | Faut-il un MP4 public pour les clips ? | M4, bande passante, stockage | **Non pour le MVP social.** HLS suffit à la lecture. Un MP4 = nouvel objet, politique de cache, peut-être CDN (PORO-029). |
| Q-MED-04 | Accepter HEIC un jour ? | Photos iPhone | **Non** sans sharp/heif côté shop **et** enum OpenAPI. Aujourd’hui : message format. |
| Q-MED-05 | Caméra in-app pour Publier / produit ? | Permissions CAMERA / MICRO | **Non.** Photo Picker / galerie seulement. |
| Q-MED-06 | Backend : post image social (`kind`, upload, actions) ? | Lot M5, B-IMG-POST | **Oui, requis** pour honorer « afficher vidéo ou image ». Hors phase conception Android. |

## 3. Non bloquant (déjà là ou simple alignement)

| Sujet | État |
|---|---|
| Importer une vidéo compatible | Livré |
| États d’envoi vidéo (%, succès, échec, un-à-la-fois) | Livré |
| Picker produit / logo / avatar `ImageOnly` | Livré |
| Plafond 10 photos, 5 Mo, jpeg/png/webp | Livré (copies format/taille à séparer : M2) |
| Aperçu **serveur** des photos | Livré |
| Aperçu **local** avant envoi | Manquant, **pas** bloqué par l’API (M2) |
| Player boutique | Absent — correct |

## 4. Informations manquantes (ne pas inventer)

- Décision juridique Q-MED-02 / textes TOS (Q-03).
- `SHARE_BASE_URL` public (partage lien, pas fichier).
- CDN devant `poro-videos` (PORO-029) : change les hôtes, pas le modèle HLS.
- Spike remux HLS : **non réalisé**. Tant qu’il n’existe pas, B-VID-SAVE tient même si Q-MED-03 = « on verra côté client ».

## 5. Contrôle de cette phase

| Contrôle | Résultat |
|---|---|
| Fichiers créés | `docs/product/mobile-media/*.md` + entrée EXECUTION_STATUS |
| `apps/mobile-android` | **Non modifié** |
| `services/*`, `docs/api/*.yaml` | **Non modifiés** |
| Audit / phase 2 / design | **Préservés** |
| Branche / commit / deploy | **Non effectués** |
| Fonctionnalités livrées | **Aucune** — conception seulement |

## 6. Ordre des décisions humaines

1. Valider : fil = **vidéo ou image** (cible) ; marketplace = images seules.
2. Trancher Q-MED-06 (contrat post image) — sans ça, le social reste vidéo à l’écran.
3. Trancher Q-MED-01 / Q-MED-02 avant tout lot M3.
4. Autoriser M1 et/ou M2 dans `apps/mobile-android` **explicitement**.
5. Ne pas autoriser M4 / M5 tant que B-VID-SAVE / B-IMG-POST tiennent.
