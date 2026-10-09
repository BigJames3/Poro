# Order Service

Panier, passage en caisse et cycle de vie des commandes PORO. Le service mène
la saga de commande avec shop (stock) et payment (paiement)
([ADR-0012](../../docs/architecture/adr/0012-marketplace-vague-3.md)).

- Contrat HTTP : [`docs/api/order.openapi.yaml`](../../docs/api/order.openapi.yaml)
- Exploitation : [`docs/runbooks/order.md`](../../docs/runbooks/order.md)
- Contrats d'événements : [`packages/contracts/events`](../../packages/contracts/events)

## Parcours

1. Le panier (un par compte) contient des variantes du catalogue, copié
   localement depuis les événements de shop : aucun appel à shop.
2. `GET /api/v1/checkout/prefill` propose les coordonnées : l'adresse par
   défaut du carnet, sinon le nom (user) et le téléphone (auth) du compte, lus
   avec le jeton de l'acheteur, sans bloquer si ces services ne répondent pas.
3. `POST /api/v1/checkout/preview` valide le panier et la livraison (nom,
   téléphone, ville, et position GPS ou adresse ; indications facultatives)
   et renvoie le récapitulatif : articles, quantités, totaux, coordonnées,
   adresse, « Paiement à la livraison » et la mention des frais de livraison
   payés au livreur. Rien n'est commandé.
4. `POST /api/v1/checkout/confirm` (`confirmed: true`) crée une commande par
   boutique (`pending`), publie `poro.order.order.created` et vide le panier,
   seulement si le panier et les prix n'ont pas changé depuis le
   récapitulatif (sinon 409 `preview_outdated`). Confirmer deux fois renvoie
   les mêmes commandes ; l'adresse peut être enregistrée dans le carnet.
5. Shop répond. `stock.reserved` fixe les prix qui font foi : paiement à la
   livraison → `confirmed`, sinon `awaiting_payment` pendant 30 minutes ;
   `order.placed` est publié. `stock.rejected` → `cancelled`.
6. `payment.succeeded` → `paid`. Sans paiement à l'échéance → `cancelled`
   (`payment_timeout`).
7. Le vendeur expédie (`shipped`, suivi en texte libre) ; l'acheteur confirme
   la réception, sinon la commande se clôt 7 jours après l'expédition
   (`completed`, publié pour shop et payment).
8. Acheteur ou vendeur peuvent annuler tant que rien n'est expédié ;
   `order.cancelled` porte `paid` pour que payment rembourse.

Un paiement arrivé après l'annulation marque la commande payée et republie
`order.cancelled` avec `paid: true` : payment rembourse.

## Événements

| Sens | Topic | Groupe |
|---|---|---|
| produit | `poro.order.order.created`, `.placed`, `.cancelled`, `.shipped`, `.completed` | outbox, relais `outbox-relay-order` |
| consommé | `poro.shop.shop.updated`, `poro.shop.product.updated`, `poro.shop.product.deleted` | `poro-order-catalog` |
| consommé | `poro.shop.stock.reserved`, `.rejected`, `poro.payment.payment.succeeded`, `.failed`, `poro.payment.refund.succeeded` | `poro-order-saga` |

Chaque événement est appliqué une fois (inbox), les messages illisibles ou
incohérents (commande inconnue, montant différent) partent en DLQ. Le
catalogue garde l'instantané au `updated_at` le plus récent ; une suppression
de produit n'est jamais annulée par un instantané en retard.

## API

| Méthode | Chemin |
|---|---|
| `GET`, `DELETE` | `/api/v1/cart` |
| `PUT`, `DELETE` | `/api/v1/cart/items/:variantId` |
| `GET` | `/api/v1/checkout/prefill` |
| `POST` | `/api/v1/checkout/preview`, `/api/v1/checkout/confirm` |
| `GET`, `POST` | `/api/v1/addresses` (10 adresses au plus) |
| `PUT`, `DELETE` | `/api/v1/addresses/:addressId` |
| `POST` | `/api/v1/addresses/:addressId/default` |
| `GET` | `/api/v1/orders`, `/api/v1/orders/:orderId` |
| `POST` | `/api/v1/orders/:orderId/cancel`, `/api/v1/orders/:orderId/confirm-receipt` |
| `GET` | `/api/v1/seller/orders?status=`, `/api/v1/seller/orders/:orderId` |
| `POST` | `/api/v1/seller/orders/:orderId/ship`, `/api/v1/seller/orders/:orderId/cancel` |
| `GET` | `/health/live`, `/health/ready`, `/metrics` |

Toutes les routes métier exigent un jeton. Le vendeur est le compte
propriétaire de la boutique (`seller_id`). Personne ne peut acheter dans sa
propre boutique.

## Données personnelles

Les coordonnées de livraison (nom, téléphone E.164, ville, adresse,
indications, position GPS) ne sont visibles que par l'acheteur et le vendeur
de la commande et ne partent dans aucun événement. Elles sont effacées
`CONTACT_RETENTION_DAYS` (90) jours après la fin de la commande ; les
récapitulatifs, qui les contiennent aussi, sont supprimés un jour après leur
expiration. Le carnet d'adresses est conservé jusqu'à ce que l'acheteur
supprime ses adresses. La position GPS n'est envoyée que si l'acheteur
l'autorise dans l'app.

## Variables d'environnement

Voir [`.env.example`](.env.example). `PORT` 8091, base `poro_order`, Redis DB 9.

| Variable | Défaut | Rôle |
|---|---|---|
| `PAYMENT_METHODS` | `cash_on_delivery` | Moyens proposés ; `wave` quand payment existe ; `simulated` en dev seulement |
| `PAYMENT_TIMEOUT_MINUTES` | `30` | Délai de paiement |
| `AUTO_COMPLETE_DAYS` | `7` | Clôture automatique après expédition |
| `CONTACT_RETENTION_DAYS` | `90` | Effacement des coordonnées |
| `SCHEDULER_ENABLED`, `SCHEDULER_INTERVAL_MS` | `true`, `60000` | Tâches planifiées (`FOR UPDATE SKIP LOCKED`, sûres sur plusieurs réplicas) |
| `AUTH_URL`, `USER_URL` | `http://localhost:8081`, `:8082` | Lecture du compte pour pré-remplir les coordonnées |
| `ACCOUNT_LOOKUP_TIMEOUT_MS` | `2000` | Au-delà, le champ reste vide |
| `PREVIEW_TTL_MINUTES` | `15` | Validité d'un récapitulatif |

## Tests

```powershell
npm run lint
npm run typecheck
npm run test:cov
```

Postgres et Redpanda via testcontainers. Couverture minimale 70 %.

## Limites

- Frais de livraison non calculés : le récapitulatif indique seulement
  qu'ils sont payés au livreur. Pas de transporteur : suivi en texte libre.
- Pas d'annulation après expédition, ni de litige ou de retour.
- Les prix du panier sont indicatifs : le prix réservé fait foi et peut
  différer si le vendeur l'a changé entre-temps.
- Sans service payment, seules les commandes payées à la livraison aboutissent.
