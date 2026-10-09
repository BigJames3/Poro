# Shop Service

Boutiques PORO : ouverture d'une boutique par compte, produits, variantes,
photos et stock. Le service tient le côté stock de la saga de commande
([ADR-0012](../../docs/architecture/adr/0012-marketplace-vague-3.md)).

- Contrat HTTP : [`docs/api/shop.openapi.yaml`](../../docs/api/shop.openapi.yaml)
- Exploitation : [`docs/runbooks/shop.md`](../../docs/runbooks/shop.md)
- Contrats d'événements : [`packages/contracts/events`](../../packages/contracts/events)

## Boutique et compte

Une boutique appartient toujours à un compte auth : `owner_id` est le `sub` du
JWT de celui qui l'ouvre, et un compte possède au plus une boutique. Il n'y a
pas de compte boutique séparé. À l'ouverture, auth accorde le rôle BUSINESS
(consommateur `poro-auth-business-roles`) ; il apparaît au refresh suivant.

- Handle : 3 à 30 `[a-z0-9._]`, même règles que les usernames mais espace de
  noms séparé, mots réservés, jamais réattribué ni modifiable.
- Pays (CI, SN, CM, NG) choisi à l'ouverture et non modifiable : il fixe la
  devise (XOF, XAF, NGN) de tous les prix. Montants en unités mineures.
- Statuts : `active`, `closed` (par le vendeur, réversible), `suspended` (par
  un ADMIN ou MODERATOR). Seule une boutique active est visible et vend.

## Événements

| Sens | Topic | Quand |
|---|---|---|
| produit | `poro.shop.shop.created` | Ouverture (auth accorde BUSINESS) |
| produit | `poro.shop.shop.updated` | Ouverture et tout changement public (nom, description, logo, statut) |
| produit | `poro.shop.product.updated` | Tout changement d'un produit, de ses photos, de ses variantes, ou passage d'une variante en rupture et retour |
| produit | `poro.shop.product.deleted` | Suppression d'un produit |
| produit | `poro.shop.stock.reserved` | Stock retenu pour une commande ; prix et titres qui font foi |
| produit | `poro.shop.stock.rejected` | Commande refusée : `out_of_stock`, `product_unavailable`, `shop_unavailable`, `currency_mismatch` |
| consommé | `poro.order.order.created` | Réservation tout ou rien |
| consommé | `poro.order.order.cancelled` | Libération du stock retenu |
| consommé | `poro.order.order.completed` | Sortie définitive du stock |

Tous les événements produits passent par l'outbox, dans la transaction du
changement (relais `outbox-relay-shop`). Le consommateur `poro-shop-stock`
applique chaque événement une fois (inbox) et met les messages illisibles en
DLQ. Une commande reçoit une seule réponse, même si `order.created` est rejoué ;
une annulation arrivée avant la commande est mémorisée et la commande tardive
ne retient rien.

## API

| Méthode | Chemin | Auth |
|---|---|---|
| `POST` | `/api/v1/shops` | Bearer |
| `GET`, `PATCH` | `/api/v1/shops/me` | Bearer |
| `POST` | `/api/v1/shops/me/close`, `/api/v1/shops/me/reopen` | Bearer |
| `POST` / `PUT` / `DELETE` | `/api/v1/shops/me/logo/upload-url`, `/api/v1/shops/me/logo` | Bearer |
| `POST`, `GET` | `/api/v1/shops/me/products` | Bearer |
| `GET`, `PATCH`, `DELETE` | `/api/v1/shops/me/products/:productId` | Bearer |
| `PUT` | `/api/v1/shops/me/products/:productId/variants/:variantId/stock` | Bearer |
| `POST` / `PUT` | `/api/v1/shops/me/products/:productId/images/upload-url`, `.../images` | Bearer |
| `DELETE` | `/api/v1/shops/me/products/:productId/images/:imageId` | Bearer |
| `GET` | `/api/v1/shops/:handle`, `/api/v1/shops/:handle/products` | — |
| `GET` | `/api/v1/products/:productId` | — |
| `POST` | `/api/v1/admin/shops/:shopId/suspend`, `.../reinstate` | ADMIN ou MODERATOR |
| `GET` | `/health/live`, `/health/ready`, `/metrics` | — |

Règles produit :

- Un produit naît en `draft`. Le passer en `active` exige au moins une photo ;
  la dernière photo d'un produit actif ne peut pas être retirée.
- `PATCH` avec `variants` remplace la liste : ids gardés et mis à jour, entrées
  sans id créées, absentes supprimées. Une variante ou un produit dont une
  commande retient du stock ne peut pas être supprimé (409).
- Le stock d'une variante existante change par sa route `stock` ; il ne
  descend jamais sous le stock réservé (409 `stock_below_reserved`).
- Photos : upload présigné (1 à 5 Mio, jpeg/png/webp), ré-encodées en WebP
  (logo 512×512, produit 1080 px max), EXIF retiré.
- Limites : 10 photos, 50 variantes, prix 1 à 10¹², stock 0 à 1 000 000.

## Variables d'environnement

Voir [`.env.example`](.env.example). `PORT` 8090, base `poro_shop`, Redis DB 8
(limites de débit), bucket `poro-shop`, `MEDIA_PUBLIC_BASE_URL` pour les URLs
publiques (CDN en production). Hors `dev` : TLS Postgres, `REDIS_URL`, HTTPS
sur `S3_PUBLIC_ENDPOINT`, CORS en origines https exactes, clés S3 obligatoires.

## Démarrage local

```powershell
docker compose up -d shop-migrate shop outbox-relay-shop
```

## Tests

```powershell
npm run lint
npm run typecheck
npm run test:cov
```

Postgres et Redpanda via testcontainers, S3 simulé. Couverture minimale 70 %.

## Limites

- Pas de modération des produits : prévue dans une petite PR sur moderation
  (cible `product`).
- Pas de vérification d'identité du vendeur (KYC) : elle viendra avec le
  paiement.
- Pas de réordonnancement des photos : elles s'ajoutent à la fin.
- Une boutique fermée garde ses produits ; ils redeviennent visibles à la
  réouverture.
- Les produits ne sont pas encore cherchables : search ne les indexe pas.
