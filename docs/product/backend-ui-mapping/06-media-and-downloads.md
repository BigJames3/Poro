# Médias et enregistrement (mapping)

Normatif : [mobile-media](../mobile-media/01-scope-and-decisions.md). Ici : lien capacités ↔ UI.

## 1. Trois verbes

| Verbe | Sens | Social | Marketplace |
|---|---|---|---|
| Importer | Galerie → app | `VideoOnly` (cible + image si contrat) | `ImageOnly` |
| Publier / Envoyer | App → Poro | Multipart vidéo | POST présigné image |
| Enregistrer | Poro → galerie | Vidéo **bloquée** (HLS, original privé) | Photo publique : Q-MED-01 |

Jamais « Télécharger » dans l’UI.

## 2. Social

| Capacité | Contrat | UI | Écart |
|---|---|---|---|
| Publier vidéo | ADR-0007, E-V01–03 | `UploadScreen` %, un à la fois, « gardez ouvert » | Desc 500 runes |
| Lire HLS | E-V05, E-V07 | `VideoPlayer` | Mute / HLS fail P1 |
| Post **image** | Aucun | Interdit de simuler | B-IMG-POST |
| Enregistrer clip | Pas de MP4 | Pas de bouton | B-VID-SAVE |
| Miniature / avatar | URLs publiques | Coil affichage | Pas « Enregistrer la miniature » |

WorkManager / offline / share fichier : **non promis**.

## 3. Marketplace

| Règle | Preuve | UI |
|---|---|---|
| Images only jpeg/png/webp 5 Mio | `media.service.ts` | Picker `ImageOnly` |
| 10 photos | `MAX_IMAGES` | `MAX_PRODUCT_IMAGES` |
| Logo 512 / photo ≤1080 WebP | sharp | Après confirm |
| Pas de player | DTO `images[].url` | `AsyncImage` seulement |
| Aperçu local avant POST | — | Cible M2 |
| HEIC | Hors enum | Message format |

## 4. Permissions

Photo Picker : aucune. Enregistrer photo API 24–28 : `WRITE_EXTERNAL_STORAGE` maxSdk 28 **au tap**. Pas de CAMERA. Manifest actuel : `INTERNET` seul.

## 5. Mémoire / sécu

Vidéo par morceaux 8 Mio. Image ≤5 Mio. Jamais logger les URLs présignées. Originaux privés (ADR-0015).
