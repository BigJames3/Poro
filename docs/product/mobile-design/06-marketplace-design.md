# Poro — Spécification visuelle marketplace

Périmètre : parcours **déjà codés** + correctifs de vérité (phase 2). Pas de catalogue global, pas de Mobile Money.

Décision : onglet Boutique **visible** (phase 2). Paiement : `cash_on_delivery` seulement.

## 1. Promesse commerciale

L’acheteur doit toujours pouvoir répondre à :

1. Chez qui j’achète ? (`@handle`, nom)
2. Combien ça coûte ? (`Formats.money`, devise de la **boutique**)
3. Est-ce disponible ?
4. Comment je paie ? → **à la livraison**
5. Où en est ma commande ? → statut serveur, jamais « Payée » après un COD

## 2. Accueil boutique — Δ majeur

### Objectif

Trouver une boutique par identifiant, accéder au panier, aux commandes, à sa vitrine vendeur.

### Structure cible

```
TopBar « Boutique »                          [🛒 n]
─────────────────────────────────────────────
Carte surface
  Trouver une boutique                       heading
  Saisissez l’identifiant que le vendeur
  vous a communiqué.
  [@                    ]
  [ Ouvrir la boutique  ]   primary
  erreur éventuelle

  [ Mes commandes ]   [ Ma boutique / Ouvrir ]
     outlined              outlined
```

### Pourquoi

Aujourd’hui quatre `OutlinedButton` empilés donnent un formulaire d’admin, pas un commerce. Une carte de recherche + deux tuiles sépare **acheter** et **vendre** sans inventer un feed.

### Composants

`TopAppBar`, `BadgedBox` panier, `OutlinedTextField` prefix `@`, `SubmitButton`, deux tuiles `OutlinedButton` 48 dp (ou `Card` cliquables 48+).

### Actions / API

| Action | API | Résultat |
|---|---|---|
| Find | `GET /shops/{handle}` | `ShopRoute` ou `shop_not_found` |
| Refresh | `GET /cart`, `GET /shops/me` | badge ; `hasShop` |
| Cart | — | `CartRoute` |
| Orders | — | `OrdersRoute(false)` |
| Sell | — | `MyShopRoute` |

`hasShop == null` : label « Ma boutique », pas « Ouvrir une boutique » (éviter le mensonge réseau).

### États

Idle, finding (CTA spinner), handle < 3 (CTA off), 404, réseau.

### Accès

Connecté. Lecture handle publique côté API ; Android session requise.

### Petit écran

Tuiles en **colonne** sous 360 dp, en rangée au-delà.

### AC

Handle actif ≥ 3 ouvre la grille. Pas de liste « boutiques populaires ». Icône onglet ≠ obligation d’un badge panier sur la tab (le badge reste dans l’écran — moins de bruit).

## 3. Vitrine

### Structure

`BackTopBar` (nom boutique). Header : logo 56 clip cercle, nom, @handle, description. Grille 2 col, gap 12, `ProductCard`.

`ProductCard` : image 1:1 clip 8 dp, titre 2 lignes, prix « À partir de » `titleMedium` bold.

### États

Loadable shop, empty `shop_no_products`, paging, 404.

### API

`GET /shops/{handle}`, `GET /shops/{handle}/products?cursor&limit`.

Closed / suspended : 404 public — **conserver** (pas d’énumération).

### Petit écran

2 colonnes minimum 140 dp. Police XXL : 1 colonne acceptable.

## 4. Fiche produit

### Hiérarchie

1. Galerie 1:1 (pager, « photo x sur y »)
2. **Prix** `titleLarge`
3. Titre
4. Vendu par `@handle` (lien)
5. Variantes `FilterChip`
6. Stepper 1–99
7. CTA Ajouter au panier
8. Description

### États

Loading, 404, rupture (`canAdd=false`), adding, `own_shop`, `variant_unavailable`, `cart_full`.

### API

`GET /products/{id}` ; `PUT /cart/items/{variantId}` `{quantity}` **absolu**.

**Correctif** : relire le panier, additionner, cap 99. Deux « +1 » → 2.

### Accès

Lecture publique API ; add = auth.

### AC

Prix toujours formaté. CTA loading. Snackbar `product_added`. Pas de bouton « Payer ».

## 5. Panier

### Structure

`BackTopBar`. Groupes par boutique (nom cliquable). Ligne : thumb 72 **clip 8**, titre, variante, `PriceText`, `QuantityStepper`, Retirer. Bandeau `unavailable` si besoin. Notice COD. Sous-total. CTA Commander.

### Règles

- `canCheckout` = ≥1 boutique et `unavailable` vide et aucune ligne `busy`.
- Qty < 1 → DELETE.
- Retour écran → `GET /cart` (prix live).

### API

`GET /cart`, `PUT/DELETE /cart/items/{id}`.

`DELETE /cart` (vider) : API sans UI — P2.

## 6. Checkout — deux écrans visuels, une route

Ne pas fusionner. Ne jamais écrire « Payé » / « Payée » comme succès COD.

### Étape 1 — Livraison

Titre `checkout_delivery_title`. Adresses chips. Champs nom, tél, ville, adresse. Checkbox enregistrer. CTA Continuer.

Validation : nom 1–80, tél `+\d`, ville 1–80, adresse 1–300.

### Étape 2 — Récap

Titre `checkout_review_title`. Lignes, **Total `titleLarge`**, bloc

```
Paiement
Paiement à la livraison
Le vendeur confirmera. Vous réglez à la réception.
```

Pas de logos Wave / Orange Money / MTN. CTA Confirmer la commande. Lien Modifier la livraison.

### API

`GET /checkout/prefill`, `GET /addresses`, `POST /checkout/preview` (aucune commande), `POST /checkout/confirm` `{preview_id, confirmed:true}` → `orders[]` typiquement `confirmed`, `paid: false`.

TTL preview 15 min. Stale → retour livraison, `checkout_preview_outdated`.

Réseau après confirm : « Vérification… » + retry idempotent ou `GET /orders`.

Back récap = `edit()`, pas pop.

### GPS

Non demandé. Adresse texte seulement.

## 7. Commandes — vérité des statuts

| Statut API | Label | COD v1 | Couleur d’appoint |
|---|---|---|---|
| `pending` | En attente | Rare (réservation) | Neutre |
| `confirmed` | Confirmée | **Oui après confirm** | Secondary |
| `awaiting_payment` | En attente de paiement | Non produit | Neutre — n’afficher que si reçu |
| `paid` | Payée | Non produit | Secondary — **jamais forcé** |
| `shipped` | Expédiée | Oui après ship vendeur | Ambre / tertiary |
| `completed` | Terminée | Reçu ou J+7 serveur | Neutre |
| `cancelled` | Annulée | Oui | Error |

`order.paid` booléen : l’UI ne badge « Payée » que si `status == paid`.

### Liste

`StatusChip` · date `dd/MM/yyyy` · total. Empty `orders_empty`.

### Détail acheteur

Lignes, paiement COD, adresse, annuler si `cancellable` (dialogue), confirmer réception si `shipped`.

### Détail vendeur

+ contact livraison (PII, pas en liste). Expédier (`tracking?` ≤200) si confirmed/paid. Annuler si non shipped.

### API

`GET /orders`, `GET /orders/{id}`, `POST .../cancel`, `POST .../confirm-receipt`  
`GET /seller/orders`, `GET /seller/orders/{id}`, `POST /seller/orders/{id}/ship`, cancel vendeur.

Pas de litige / retour (PORO-036).

## 8. Ma boutique et éditeur

### Ouverture

Handle définitif, pays = devise définitive (CI/SN → XOF, CM → XAF, NG → NGN). Hint clair, pas de « vous pourrez changer ».

### Dashboard

Logo, nom, @, `StatusChip` (Ouverte / Fermée / Suspendue), description, CTA commandes vendeur, liste produits, Nouveau produit. Fermer / rouvrir.

Suspendue : message, **pas** de bouton admin.

### Éditeur

Titre 1–120, desc ≤2000, variantes ≤50, photos **10** (aligner UI actuelle 8), stock. Publier `active` exige photo. Archive. Delete + dialogue (`has_reservations`).

### API

`POST /shops`, `GET/PATCH /shops/me`, close/reopen, logo upload, `GET /shops/me/products`, CRUD product, images, stock.

DELETE logo : API sans UI — P2.

## 9. Copies honnêtes (ne pas « designer »)

| String | Conserver |
|---|---|
| `cart_payment_notice` | Paiement à la livraison. Une commande par boutique. |
| `checkout_placed` | Commande passée. Le vendeur va la préparer. |
| `payment_cash_on_delivery` | Paiement à la livraison |
| `shop_status_suspended` | Suspendue par la modération |

Interdit : « Paiement réussi », « Commande payée », logos wallets, « Livraison estimée 24 h » (pas de contrat).

## 10. Accès

| Surface | Visiteur API | Android actuel |
|---|---|---|
| Handle, vitrine, produit GET | Public | Session requise |
| Panier, checkout, orders, me | Auth | Auth |
| Staff suspend | JWT staff | **Pas dans l’app** |

## 11. Petit écran

- Hub : tuiles empilées.
- PDP : galerie pleine largeur, CTA au-dessus du nav system (écran enfant = pas de bottom bar, padding insets).
- Checkout : un champ par ligne, Continuer visible.
- Grille : 2 → 1 si largeur utile < 300 dp.

## 12. AC marketplace visuelle

1. Shop home n’est plus une pile de 4 boutons identiques.
2. Prix hiérarchisé sur carte, PDP, panier, récap.
3. Images clipées.
4. Aucun contrôle Mobile Money.
5. Confirm COD → statut `confirmed` + `checkout_placed`.
6. `paid` jamais synthétisé.
7. `hasShop` réseau non trompeur.
8. Add-to-cart additionne.
9. Photos éditeur max 10.
10. Parcours social (feed) non régressé.
