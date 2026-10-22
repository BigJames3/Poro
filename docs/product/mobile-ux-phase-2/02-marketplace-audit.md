# Axe A — Diagnostic et parcours de la marketplace

Référence code : `apps/mobile-android/app/src/main/java/com/poro/app/` (abrégé `app/`), PR #58 @ `237075f`.
Référence contrats : `docs/api/shop.openapi.yaml`, `docs/api/order.openapi.yaml` (`origin/main`).

Légende des gravités : **P0** bloque un parcours ou affiche une information fausse ;
**P1** risque d'erreur ou de confusion ; **P2** confort.

## 1. Carte des parcours

```
Onglet Boutique (ShopHomeRoute)
 ├─ « Trouver une boutique » @handle ──► ShopRoute(handle) ──► ProductRoute(id) ──► [Ajouter au panier]
 ├─ Panier (CartRoute) ──► CheckoutRoute : Livraison ──► Récapitulatif ──► [Confirmer]
 │                                                                   └─► OrdersRoute() (pile vidée jusqu'à l'onglet)
 ├─ Mes commandes (OrdersRoute(asSeller=false)) ──► OrderRoute(id, false) : annuler / confirmer la réception
 └─ Ma boutique (MyShopRoute)
       ├─ pas de boutique : formulaire d'ouverture
       └─ boutique : logo, nom, description, fermer/rouvrir
             ├─ Commandes reçues (OrdersRoute(asSeller=true)) ──► OrderRoute(id, true) : expédier / annuler
             └─ Produits ──► ProductEditorRoute(id?, devise) : déclinaisons, stock, photos, publier, supprimer
Notification de commande ──► OrderRoute(id, asSeller)
```

## 2. Inventaire par parcours

### 2.1 Découvrir les boutiques

| Élément | Constat |
|---|---|
| Écran | `ui/shop/ShopHomeScreen.kt`, `ShopHomeViewModel.kt` |
| API | `GET /api/v1/shops/{handle}` (public) ; aucune route de liste ou de recherche |
| États | recherche en cours (bouton), erreur `shop_not_found` ou 404 → « Aucune boutique avec cet identifiant », réseau |
| Validation | identifiant ≥ 3 caractères après retrait de `@` ; mis en minuscules côté dépôt |
| Écart | La « découverte » se limite à un identifiant connu. **Normal** : PORO-026 non livré. Ne pas ajouter de liste factice |
| Recommandation | P2 : texte d'aide expliquant que l'identifiant est partagé par le vendeur ; lien de partage de boutique quand `SHARE_BASE_URL` existera |

### 2.2 Ouvrir une vitrine

| Élément | Constat |
|---|---|
| Écran | `ShopScreen.kt`, `ShopViewModel.kt` |
| API | `GET /api/v1/shops/{handle}`, `GET /api/v1/shops/{handle}/products?cursor&limit=20` (publics) |
| États | `LoadableContent` (chargement / erreur + Réessayer), liste vide « aucun produit », pagination infinie |
| Écart P1 | Une boutique `closed` ou `suspended` : comportement serveur à confirmer (404 attendu côté public). L'app affiche le message générique d'erreur, sans état dédié |
| Écart P2 | La grille n'affiche pas l'état « rupture » sur la vignette |

### 2.3 Fiche produit

| Élément | Constat |
|---|---|
| Écran | `ProductScreen.kt`, `ProductViewModel.kt` |
| API | `GET /api/v1/products/{productId}` (public) ; ajout : `PUT /api/v1/cart/items/{variantId}` `{quantity}` |
| Validation | quantité 1..99 (contrat : `minimum 1, maximum 99`) ✅ ; déclinaison en stock présélectionnée |
| Écart **P0** | `PUT` **remplace** la quantité de la ligne (« Set the quantity ») : ajouter 1 article déjà présent 2 fois dans le panier ramène la ligne à 1. Le message « Ajouté au panier » est alors trompeur |
| Correction | Lire la quantité existante (`GET /cart`) et envoyer `existante + choisie` (plafonnée à 99), ou libeller l'action « Mettre N au panier ». Recommandé : additionner, plafonner, et afficher « Quantité au panier : N » |
| Écart P1 | Après l'ajout, aucun accès direct au panier (snackbar sans action) |
| Écart P1 | `own_shop` (acheter dans sa propre boutique) : message mappé, mais le bouton reste actif sur ses propres produits |

### 2.4 Panier

| Élément | Constat |
|---|---|
| Écran | `CartScreen.kt`, `CartViewModel.kt` |
| API | `GET /api/v1/cart`, `PUT /cart/items/{variantId}`, `DELETE /cart/items/{variantId}` (chaque réponse renvoie le panier à jour) |
| États | chargement, erreur + Réessayer, vide, groupes par boutique, lignes `unavailable` séparées |
| Double soumission | Bouton de la ligne désactivé pendant la requête (`busy`) ✅ |
| Conservation | Le panier vit côté serveur ✅ ; rechargé à chaque affichage (un seul appel depuis `237075f`) |
| Écart P1 | Le badge du panier de l'onglet n'est relu qu'au retour sur l'onglet Boutique |
| Écart P2 | `DELETE /api/v1/cart` (vider) existe et n'est pas proposé |

### 2.5 Checkout, récapitulatif, confirmation (paiement à la livraison)

| Étape | Écran / API | Constat |
|---|---|---|
| Préremplissage | `GET /checkout/prefill`, `GET /addresses` | ✅ Champs restaurés après mort du processus (`SavedStateHandle`) |
| Validation locale | `DeliveryForm.isValid` | ⚠️ **Écart P1** : le serveur exige adresse **et** point de repère d'au moins **3 caractères** (`cleanText(…, 300, 3, …)`, `services/order/src/delivery/delivery.ts`) ; l'app accepte 1 caractère → 422 `delivery_invalid` évitable |
| Récapitulatif | `POST /checkout/preview` (valide 15 min) | ✅ Affiche lignes, sous-totaux, totaux par devise, livraison, `payment_method_label`, `delivery_fee_notice` |
| Confirmation | `POST /checkout/confirm` `{preview_id, confirmed:true}` | ✅ Idempotente côté serveur ; bouton désactivé pendant l'envoi ; récapitulatif périmé (`preview_outdated/expired/not_found`) → retour à la relecture |
| Après confirmation | Message « Commande passée. Le vendeur va la préparer. » | ⚠️ **Écart P0 (message trompeur)** : chaque commande démarre `pending` jusqu'à la réservation du stock, puis passe `confirmed` (COD) **ou** `cancelled` (`stock_rejected`). Le message doit dire « Commande envoyée. La boutique confirme la disponibilité. » |
| Codes non gérés | `confirmation_required`, `address_book_full`, `address_label_invalid`, `address_not_found` | P1 : tombent dans le message générique |

### 2.6 Commandes acheteur

| Élément | Constat |
|---|---|
| Écrans | `OrderScreens.kt` (`OrdersScreen`, `OrderScreen`), `OrderViewModels.kt` |
| API | `GET /orders`, `GET /orders/{id}`, `POST /orders/{id}/cancel`, `POST /orders/{id}/confirm-receipt` |
| Statuts | `pending`, `awaiting_payment`, `confirmed`, `paid`, `shipped`, `completed`, `cancelled` → libellés français ✅ |
| « Payé » | ✅ Seul le statut `paid` affiche « Payée » ; une commande COD reste `confirmed` jusqu'à l'expédition. Le champ `paid`/`paid_at` n'est pas utilisé. **Règle à conserver** : ne jamais déduire « payé » de `completed` |
| Écart **P0** | `total` est `null` tant que le stock n'est pas réservé ; l'app affiche alors `subtotal_seen` sous le libellé « Total : » → montant présenté comme définitif à tort. Afficher « Total estimé » quand `total == null` |
| Écart P1 | `cancel_reason` affiché brut (« Motif : stock_rejected ») ; à traduire : `stock_rejected`, `payment_timeout`, `buyer_cancelled`, `seller_cancelled` |
| Écart P1 | `unit_price` (définitif) ≠ `unit_price_seen` : l'écart de prix n'est pas signalé à l'acheteur |
| Double soumission | `working` désactive les actions ✅ ; annulation confirmée par une boîte de dialogue ✅ |
| `409 order_status_conflict` | Message mappé ; **P1** : recharger la commande après ce code (statut changé entre-temps) |

### 2.7 Commandes vendeur

| Élément | Constat |
|---|---|
| API | `GET /seller/orders`, `GET /seller/orders/{id}`, `POST …/ship {tracking?}`, `POST …/cancel` |
| Écart P1 | La liste vendeur affiche le **nom de sa propre boutique** sur chaque ligne (champ `shop.name`) : information inutile. Afficher le nom du destinataire (`delivery.full_name`) et la ville |
| Expédition | Proposée pour `confirmed` et `paid` ✅ (contrat : « Mark a confirmed or paid order shipped ») ; suivi facultatif ≤ 200 caractères ✅ |
| Écart P2 | Pas de filtre par statut (« à expédier ») ; le contrat n'en propose pas : à trier côté client seulement sur la page chargée, ou demander un filtre serveur |

### 2.8 Ma boutique et produits

| Élément | Constat |
|---|---|
| API | `POST /shops`, `GET/PATCH /shops/me`, `close`, `reopen`, logo (`upload-url`, `PUT`, `DELETE`), produits (`POST`, `GET`, `PATCH`, `DELETE`), stock, images |
| Ouverture | identifiant `^[A-Za-z0-9._]{3,30}$` et pays (fixe la devise) ✅ |
| Écart **P0 (constaté)** | Envoi des photos en échec sur l'émulateur ; conséquence : publication refusée `image_required`. Diagnostic en cours (`PoroMedia`) — ne pas modifier le code avant le résultat |
| Écart P1 | Après un échec d'envoi de photo, le message est générique ; distinguer « réseau » (réessayer) de « fichier refusé » |
| Écart P2 | `DELETE /shops/me/logo` non proposé |
| Double soumission | `working` ✅ ; confirmation après enregistrement ajoutée dans `237075f` |

## 3. Matrice écran → action → API → résultat → erreurs → acceptation

| Écran | Action | API | Résultat attendu | Erreurs à gérer | Critère d'acceptation |
|---|---|---|---|---|---|
| ShopHome | Trouver `@handle` | `GET /shops/{handle}` | Ouvre la vitrine | 404/`shop_not_found`, réseau, 429 | Un identifiant inconnu affiche « Aucune boutique… » sans quitter l'écran |
| Shop | Défiler | `GET /shops/{h}/products?cursor` | Page suivante ajoutée sans doublon | `invalid_cursor`, réseau | 45 produits → 3 pages, aucune vignette dupliquée |
| Product | Ajouter N | `GET /cart` puis `PUT /cart/items/{v}` `{min(existante+N, 99)}` | Ligne = existante + N | `variant_unavailable`, `own_shop`, `cart_full`, `too_many_lines` | Ajouter 2 puis 1 donne 3 au panier |
| Cart | Changer la quantité | `PUT /cart/items/{v}` | Panier à jour (prix catalogue) | `variant_unavailable`, réseau | Boutons de la ligne inactifs pendant l'appel |
| Cart | Retirer | `DELETE /cart/items/{v}` | Ligne retirée | réseau | Une ligne `unavailable` peut toujours être retirée |
| Checkout | Continuer | `POST /checkout/preview` | Récapitulatif affiché | `delivery_invalid`, `delivery_location_required`, `cart_empty`, `cart_unavailable`, `payment_method_unavailable` | Adresse et repère < 3 caractères refusés localement, sans appel |
| Checkout | Confirmer | `POST /checkout/confirm` | Commandes créées (`pending`) | `preview_outdated/expired/not_found` → relecture ; `confirmation_required` | Double appui : une seule série de commandes ; message « confirmation du stock en cours » |
| Order (acheteur) | Annuler | `POST /orders/{id}/cancel` | `cancelled`, `buyer_cancelled` | `order_status_conflict` → recharger | Bouton absent après `shipped` |
| Order (acheteur) | Confirmer la réception | `POST /orders/{id}/confirm-receipt` | `completed` | `order_status_conflict` | Visible seulement en `shipped` |
| Order (vendeur) | Expédier | `POST /seller/orders/{id}/ship` | `shipped`, suivi enregistré | `tracking_invalid`, `order_status_conflict` | Visible seulement en `confirmed`/`paid` |
| Order (tous) | Afficher le total | — | « Total » si `total` ≠ null, sinon « Total estimé » | — | Une commande `pending` n'affiche jamais « Total : » seul |
| MyShop | Enregistrer | `PATCH /shops/me` | « Boutique enregistrée » | `shop_name_invalid`, réseau | Un seul appel par appui ; message affiché |
| ProductEditor | Publier | `PATCH …/products/{id}` `{status:active}` | « En vente » | `image_required` | Message explicite invitant à ajouter une photo |

## 4. Synthèse des corrections du Lot 1

| ID | Gravité | Correction | Fichiers probables |
|---|---|---|---|
| M1 | P0 | Ajout au panier additif (lecture + somme plafonnée) | `ProductViewModel.kt`, test associé |
| M2 | P0 | « Total estimé » quand `total == null` | `OrderScreens.kt`, `strings.xml` |
| M3 | P0 | Message post-confirmation fidèle au statut `pending` | `strings.xml` (`checkout_placed`) |
| M4 | P0 | Diagnostic puis correction de l'envoi de photos | selon `PoroMedia` (`PresignedUpload.kt`, environnement) |
| M5 | P1 | Validation adresse/repère ≥ 3 caractères | `CheckoutViewModel.kt` (`DeliveryForm`) |
| M6 | P1 | Libellés des `cancel_reason` | `OrderScreens.kt`, `strings.xml` |
| M7 | P1 | Liste vendeur : destinataire + ville au lieu du nom de boutique | `OrderScreens.kt` |
| M8 | P1 | Codes `confirmation_required`, `address_*` mappés | `ApiResult.kt`, `AuthErrorMessages.kt`, `strings.xml` |
| M9 | P1 | Recharger la commande après `order_status_conflict` | `OrderViewModels.kt` |
| M10 | P1 | Accès au panier depuis la confirmation d'ajout | `ShopGraph.kt` (snackbar avec action) |
| M11 | P1 | Désactiver l'achat sur ses propres produits | `ProductScreen.kt` (comparer `shop.shop_id` à sa boutique) |
| M12 | P2 | Vider le panier, retirer le logo, filtre « à expédier » local | à arbitrer |
