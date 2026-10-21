# Médias, import et enregistrement — périmètre et décisions

- **Date** : 21 octobre 2026.
- **Nature** : conception et spécification. **Aucun fichier de `apps/mobile-android`, des services ou des OpenAPI n’est modifié.**
- **Base** : OpenAPI `docs/api/*.yaml`, ADR-0007 / ADR-0012 / ADR-0015, code Kotlin et services en vigueur.
- **Complète** : [mobile-ux-audit](../mobile-ux-audit/01-product-overview.md), [phase 2](../mobile-ux-phase-2/01-phase-2-scope.md), [mobile-design](../mobile-design/01-visual-direction.md).

## 1. Vocabulaire obligatoire

Le mot « télécharger » est interdit dans l’UI et dans les strings : il mélange deux actions.

| Action réelle | Terme UI (FR) | Sens | Exemples d’écrans |
|---|---|---|---|
| Envoyer un fichier de l’appareil vers Poro | **Importer** (choix) / **Publier** ou **Envoyer** (soumission) | Upload présigné | Onglet Publier, photos produit, logo, avatar |
| Écrire un média Poro dans la galerie de l’appareil | **Enregistrer** | Sauvegarde locale | Menu ⋮ d’un média **autorisé** |

Interdit : « Télécharger », « Download », « Save & share file », « Disponible hors ligne ».

Le partage actuel (`ShareSheet`) envoie du **texte** (`text/plain`), éventuellement une URL si `SHARE_BASE_URL` est configuré. Ce n’est pas un partage de fichier.

## 2. Décisions produit (cette phase)

| # | Décision | Conséquence |
|---|---|---|
| 1 | Le fil social affiche une **vidéo ou une image** (un média par page). | Cible produit. **Aujourd’hui le contrat ne porte que des vidéos** (B-IMG-POST). Pas de picker photo tant que l’API n’existe pas. |
| 2 | La marketplace n’accepte que des **images** (produits et logo). | Aucun bouton, picker ou lecteur vidéo dans `ui/shop/`. |
| 3 | Les formats, tailles et plafonds viennent du **backend** (code + OpenAPI). | Ne pas inventer de HEIC, GIF, 4K produit, 20 photos, etc. |
| 4 | « Enregistrer une vidéo » n’est **pas livrable** tant qu’il n’existe pas un fichier unique public. | L’original est privé ; la lecture est HLS segmenté. Voir [10](10-blockers-and-decisions.md) B-VID-SAVE. |
| 5 | « Enregistrer une image publique » est **techniquement possible** côté client, sans nouvel endpoint. | Décision d’activation : Q-MED-01. Pas de flag `downloadable` dans l’API. |
| 6 | Pas de promesse hors vérification. | Pas de téléchargement en arrière-plan, lecture hors ligne, ni partage de fichier. |
| 7 | Permissions Android **au moment utile**. | Photo Picker : aucune. Enregistrement galerie : seulement si Q-MED-01 et API 24–28. |
| 8 | Conception seulement. | Implémentation Kotlin : autorisation explicite distincte. |

## 3. Périmètre inclus

1. Audit des capacités réelles (upload, stockage, diffusion, lecture).
2. Spec sociale : afficher et publier une **vidéo ou une image** ; écart contrat actuel documenté.
3. Spec marketplace : images uniquement, prévisualisation, retrait, erreurs format/taille.
4. Spec d’enregistrement local : états, copies, sécurité ; livrable vs bloqué.
5. Architecture, permissions, critères d’acceptation, plan par lots, blocages.

## 4. Exclusions explicites

| Exclu | Preuve |
|---|---|
| Livrer un post **image** sans nouvel OpenAPI | Feed/social/search = `video_id` seulement (B-IMG-POST). La cible produit reste vidéo **ou** image. |
| Vidéo produit ou vitrine | Shop : `content_type` ∈ jpeg/png/webp ; DTO `images[].url` |
| Endpoint `GET …/download` ou MP4 progressif public | Absent de `video.openapi.yaml` |
| Original utilisateur (`source.mp4` / EXIF) | Bucket privé `poro-video-sources` / `*-uploads` (ADR-0015) |
| WorkManager, DownloadManager, cache hors ligne | Absents de l’app ; lot 5 phase 2 inchangé |
| Firebase / FCM | Absent (`google-services.json`) |
| Modification backend, OpenAPI, Kotlin | Cette phase documentation |
| Commit, PR, déploiement | Interdit sans demande |

## 5. Contradiction à ne pas « corriger » en silence

| Source | Affirme | Fait |
|---|---|---|
| Exigence produit « vidéo **ou** image » dans le social | Posts image + fil mixte | Aucun `kind` / `image_url` de post dans feed/social/video. **Cible retenue**, livraison bloquée. |
| Audit `03` MOD-001 | « Aucune UI Android » signalement | Lots 1–3 ont livré le report (non commité) |
| OpenAPI vidéo description `maxLength: 1000` | 1000 | Code `MaxDescriptionRunes = 500` |
| OpenAPI titre `maxLength: 200` | 200 | Code `MaxTitleRunes = 100` (Android déjà 100) |

La spec s’aligne sur le **code serveur** pour les limites d’acceptation, et sur l’OpenAPI pour les routes.

## 6. Livrables

| Fichier | Contenu |
|---|---|
| [02-backend-media-audit.md](02-backend-media-audit.md) | Contrats, buckets, limites |
| [03-android-media-audit.md](03-android-media-audit.md) | Pickers, upload, player, permissions |
| [04-social-media-spec.md](04-social-media-spec.md) | Fil et Publier : vidéo **ou** image ; contrat manquant |
| [05-marketplace-images-spec.md](05-marketplace-images-spec.md) | Images produit / logo |
| [06-download-ux-spec.md](06-download-ux-spec.md) | Enregistrer : états et blocage vidéo |
| [07-security-and-architecture.md](07-security-and-architecture.md) | URLs, mémoire, cycle de vie |
| [08-acceptance-tests.md](08-acceptance-tests.md) | AC-MED-* |
| [09-implementation-plan.md](09-implementation-plan.md) | Lots M0–M4 |
| [10-blockers-and-decisions.md](10-blockers-and-decisions.md) | B-* et Q-MED-* |

Rien de ce dossier n’est **implémenté**. C’est une spec prête pour revue.
