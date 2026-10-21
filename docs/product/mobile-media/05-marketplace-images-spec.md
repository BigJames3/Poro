# Spec — images marketplace

La boutique Poro est **exclusivement** illustrée par des images. Cette règle est déjà vraie côté API et Android ; la spec fige l’UX manquante (aperçu avant envoi, erreurs distinctes) sans inventer de formats.

Écrans : `ProductEditorScreen`, `MyShopScreen` (logo), `ProductScreen`, `ShopScreen`, `CartScreen`.

## 1. Interdits (non négociables)

| Interdit | Comment le garantir |
|---|---|
| Bouton « Ajouter une vidéo » produit / logo | Aucun CTA de ce type |
| Picker `VideoOnly` ou `ImageAndVideo` dans `ui/shop/` | `ImageOnly` uniquement |
| `VideoPlayer` / Media3 dans les écrans boutique | `AsyncImage` seulement |
| MIME vidéo envoyé à shop | Contrôle local `IMAGE_TYPES` + enum serveur |
| Promettre HEIC, GIF, SVG, BMP | Hors liste jpeg/png/webp |
| Plus de 10 photos | CTA désactivé + `too_many_images` |
| Publier `active` sans photo | Serveur `image_required` ; CTA Publier après au moins une image confirmée |

## 2. Limites à afficher (source serveur)

| Champ | Valeur | Où le dire |
|---|---|---|
| Formats | JPEG, PNG, WebP | Hint sous « Ajouter une photo » / logo |
| Taille originale | 5 Mo | Même hint ; erreur dédiée si dépassé |
| Photos / produit | 10 | Compteur « n / 10 » |
| Logo publié | Carré 512 px (recadré par le serveur) | Hint : « L’image sera recadrée en carré. » |
| Photo publiée | Grand côté ≤ 1080 px, ratio conservé | Pas d’éditeur de recadrage in-app (absent) |
| TTL upload | 5 min | Invisible si l’envoi suit le pick de près |

Ne pas afficher 40 Mpx ni la qualité WebP 82 : détails d’implémentation serveur.

## 3. Photos produit — parcours cible

L’API exige un `productId` pour `…/images/upload-url`. **On ne simule pas** un brouillon d’images avant création.

```
1. Créer le produit (titre + variantes) → Save
2. Bloc Photos apparaît
3. « Ajouter une photo » → Photo Picker ImageOnly
4. Aperçu LOCAL (Coil sur content://) + actions
      [Retirer]  [Envoyer cette photo]
5. Envoi présigné + confirm → la photo rejoint la rangée serveur
6. Retirer une photo confirmée = DELETE (produit active : dernière photo refusée)
```

### États du bloc Photos

| État | UI |
|---|---|
| Produit pas encore créé | Bloc absent (déjà) |
| 0 photo | Hint formats + CTA Ajouter |
| Aperçu local (pending) | Vignette 96 dp, badge « Non envoyée », Retirer, Envoyer |
| Envoi | Vignette + barre ou spinner **sur cette vignette** ; CTA Ajouter off ; anti double envoi |
| Confirmée | Vignette `url` publique + Retirer |
| 10 confirmées | CTA Ajouter off ; `error_too_many_images` si course |
| `working` autre (save / publish) | Pickers et deletes off |

Remplacer = retirer (si autorisé) puis ajouter. Pas d’endpoint « replace ».

Ordre des photos : celui renvoyé par l’API. Pas de drag & drop (contrat absent).

### Validation locale (avant tout POST)

```
MIME ∉ {image/jpeg, image/png, image/webp} → error_image_format
size > 5 Mio                         → error_image_too_large
illisible / taille inconnue          → error_image_invalid
```

Aujourd’hui une seule string mélange format et taille. **Les séparer** (nouvelles clés). Le serveur peut encore renvoyer `image_invalid` / `image_too_large` / `image_not_uploaded` / `image_upload_invalid` : les mapper distinctement (`AuthErrorMessages`).

HEIC : tombe dans `error_image_format`. Copie : « JPEG, PNG ou WebP uniquement. Les photos HEIC des iPhone ne sont pas acceptées. »

## 4. Logo boutique

Même picker `ImageOnly`, mêmes MIME et 5 Mo.

Cible :

1. Tap « Changer le logo » → pick
2. Aperçu local circulaire 56–80 dp
3. Envoyer / Annuler
4. Succès : `logo_url` serveur (carré recadré — l’aperçu local peut différer du résultat)

`DELETE` logo : toujours P2 (API sans UI), inchangé.

Boutique `suspended` : pas d’upload (403 `shop_suspended`, message existant).

## 5. Affichage acheteur

| Surface | Règle |
|---|---|
| Vitrine `ProductCard` | 1:1, clip 8 dp, première `images[0].url` |
| PDP | Pager 1:1, « photo x sur y », `ContentScale.Crop` |
| Panier | Vignette 72 clip 8 |
| Commande | `image_url` ligne si présent |
| Logo vitrine | Cercle 56 |

Image absente : fond `surfaceVariant` + initiale / icône, pas de rectangle cassé. URL 404 Coil : placeholder, pas de crash.

**Pas** de bouton Enregistrer sur la PDP tant que Q-MED-01 n’est pas tranché. Si oui : voir [06](06-download-ux-spec.md) — action sur la photo courante du pager, jamais un lecteur.

## 6. Anti-abus et réseau

- `working` / pending unique : un seul POST image à la fois par écran.
- 429 logo (10/min) ou photo (30/min) : message throttle, pas de retry immédiat.
- Réseau pendant l’envoi : garder l’aperçu local, bouton **Réessayer** (même URI tant que le Photo Picker le permet).
- Formulaire présigné expiré (reprise > 5 min) : redemander `upload-url`, ne pas réutiliser `upload_key` étranger.

## 7. Mémoire

Lire au plus 5 Mio pour une image. Ne pas décoder un bitmap pleine résolution pour l’aperçu : Coil + taille d’affichage (96 dp / 80 dp). Le fichier original n’est pas recopié dans le cache disque de l’app au-delà de Coil.

## 8. Copies

| Clé (cible) | Texte |
|---|---|
| `product_photos` | Photos (existante) |
| `product_add_photo` | Ajouter une photo (existante) |
| `product_photo_rules` | JPEG, PNG ou WebP, 5 Mo au plus. 10 photos maximum. |
| `product_photo_pending` | Cette photo n’est pas encore envoyée. |
| `product_photo_send` | Envoyer cette photo |
| `shop_logo_rules` | JPEG, PNG ou WebP, 5 Mo. Elle sera recadrée en carré. |
| `error_image_format` | Format non pris en charge. Choisissez une image JPEG, PNG ou WebP. |
| `error_image_too_large` | Image trop lourde : 5 Mo au plus. |
| `error_image_invalid` | Image illisible. Choisissez une autre photo. |
| `error_too_many_images` | 10 photos au plus par produit. (existante) |
| `error_image_required` | Ajoutez au moins une photo avant de publier. (existante) |

Aucun verbe « Télécharger » / « Uploader » dans ces strings.
