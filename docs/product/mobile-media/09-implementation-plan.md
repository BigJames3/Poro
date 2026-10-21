# Plan d’implémentation — médias

Aucun lot M* n’est commencé dans `apps/mobile-android`. **Autorisation explicite distincte** requise, lot par lot.

Complète [phase-2/09](../mobile-ux-phase-2/09-implementation-plan.md) et [mobile-design/09](../mobile-design/09-implementation-plan.md) : lots **médias**, sans les fusionner. Marketplace COD, report, légal restent la source métier.

## 1. Principes

1. Un lot = un thème ; social et boutique non régressés (`NR-*`).
2. Pas de nouvel endpoint. Pas d’OpenAPI « pour faire joli ».
3. Pas de WorkManager, DownloadManager, caméra, `READ_MEDIA_*`.
4. Strings : Publier / Envoyer / Enregistrer — jamais « Télécharger ».
5. Si un AC est **Bloqué**, le lot documente le blocage ; il n’ajoute pas d’UI trompeuse.

## 2. Lots

### Lot M0 — Vocabulaire et revue (docs + strings si OK Android)

| | |
|---|---|
| But | Verrouiller le langage ; confirmer l’absence d’Enregistrer vidéo |
| Prérequis | Revue de ce dossier ; Q-MED-01/02 au moins **connus** (défaut : non) |
| Travaux | Grep strings « télécharg » ; aucune nouvelle surface |
| Interdit | Feature save, picker mixte |
| Terminé | AC-SOC-11, AC-SAV-01 ; EXECUTION_STATUS à jour |

Réalisable **sans** Kotlin si la revue suffit. Touchers `strings.xml` seulement après OK.

### Lot M1 — Import vidéo (alignements honnêtes)

| | |
|---|---|
| But | Description 500 runes ; copies stables ; pas de photo sur Publier |
| Prérequis | Autorisation Android ; M0 |
| Travaux | `UploadFormState.DESCRIPTION_MAX = 500` ; compteur ; tests |
| Fichiers | `UploadViewModel.kt`, `UploadScreen.kt`, tests, strings |
| Interdit | `ImageAndVideo` (B-IMG-POST), WorkManager, bouton Enregistrer |
| Tests | AC-SOC-01…06, 10, 12 ; NR-02 |
| Terminé | Une description trop longue est coupée / refusée **avant** l’init |

### Lot M2 — Images boutique (aperçu avant envoi)

| | |
|---|---|
| But | Pending local, erreurs format ≠ taille, compteur 10 |
| Prérequis | Autorisation ; lot 1 phase 2 (plafond 10 déjà) |
| Travaux | État `pendingImage` dans `ProductEditorViewModel` / logo ; Coil `content://` ; Envoyer / Retirer ; strings nouvelles |
| Fichiers | `ProductEditor*`, `MyShop*`, `AuthErrorMessages`, `strings.xml`, tests shop |
| Interdit | Vidéo, HEIC accepté, photos avant `productId`, player |
| Tests | AC-MX-IMG-* ; NR-04, 05 |
| Terminé | Pick → aperçu → POST ; HEIC message format ; 0 player shop |

### Lot M3 — Enregistrer une photo publique

| | |
|---|---|
| But | Galerie MediaStore pour **photo produit** si Q-MED-01 = oui |
| Prérequis | Q-MED-01 oui **et** Q-MED-02 (légal) non bloquant ; autorisation Android |
| Travaux | `SaveMedia*` ; permission maxSdk 28 au tap ; snackbars ; ⋮ PDP |
| Interdit | Save HLS, save original, WorkManager, notification, share fichier |
| Tests | AC-SAV-03…10 ; NR-01, 05 |
| Si Q-MED-01 = non | Lot **annulé**. Pas de bouton fantôme. |

### Lot M5 — Post image social (conditionné backend)

| | |
|---|---|
| But | Afficher et publier une **image** dans le fil, à côté des vidéos |
| Prérequis | **B-IMG-POST levé** : OpenAPI + ADR (`kind`, URL image, upload, social, search, report). Autorisation Android séparée. |
| Travaux | Renderer fil : `video` → player, `image` → `AsyncImage` plein cadre ; Publier `ImageAndVideo` ou deux CTA ; validation selon le contrat **publié** |
| Interdit | Envoyer une photo à `/videos/uploads` ; inventer jpeg/5 Mo si l’OpenAPI dit autre chose ; player sur une page image ; cartes fake |
| Tests | AC-SOC-07, 08 ; NR-01…03 |
| Si backend refusé | Lot **annulé**. Le fil reste vidéo seule. La cible produit reste écrite dans [04](04-social-media-spec.md). |

### Lot M4 — Futur enregistrement vidéo (conditionné)

| | |
|---|---|
| But | Enregistrer un **fichier unique** public |
| Prérequis | **Backend** : rendition MP4 progressive **ou** spike Android remux HLS **écrit et accepté** (ADR). Aujourd’hui : **B-VID-SAVE**. |
| Travaux | Alors seulement : bouton overlay, % , retry, éventuellement WorkManager |
| Interdit | Simuler avec `master.m3u8`, dump cache ExoPlayer, exposer `poro-video-sources` |
| Terminé | AC-SAV-11 devient un vrai AC — **pas avant** |

## 3. Ordre

```
Revue produit (M0, docs)
    → (OK Android) M1 import vidéo
    → M2 images boutique
    → stop Enregistrer vidéo
    → M3 seulement si Q-MED-01 + Q-MED-02
    → M4 jamais tant que B-VID-SAVE
    → M5 jamais tant que B-IMG-POST
```

M2 peut précéder M1 (fichiers disjoints). Ne pas ouvrir M2 et un lot shop phase 2 le même jour sur `ProductEditor*`.

## 4. Définition de terminé d’un lot M

- AC du lot verts (unitaires a minima).
- NR-01…06 non régressés.
- Aucune route staff.
- Aucun secret / URL présignée dans les logs de test.
- `apps/mobile-android` touché seulement après OK **de ce lot**.
- Blocages restants listés dans [10](10-blockers-and-decisions.md), pas contournés.

## 5. Ce que ce plan ne fait pas

- Commits, PR, deploy.
- Modification `services/*` ou `docs/api/*.yaml`.
- Mode invité, FCM, Wave, search catalogue.
- Caméra in-app.
