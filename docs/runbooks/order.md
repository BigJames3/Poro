# Runbook — service order

Port 8091, base `poro_order`, Redis DB 9, groupes Kafka `poro-order-catalog`
et `poro-order-saga`, relais `outbox-relay-order`.

## Sondes

`/health/ready` vérifie Postgres et Redis. Kafka n'en fait pas partie : paniers
et commandes restent consultables pendant une panne du broker ; les réponses
de shop et payment attendent dans Kafka.

## Une commande reste `pending`

Shop n'a pas répondu à `order.created`.

1. `SELECT topic, published_at, last_error FROM outbox_events WHERE event_key = '<order_id>';`
   `published_at` vide : voir le relais (runbook outbox-relay).
2. Publié : côté shop, voir le runbook shop (« Une commande reste sans réponse de stock »).
3. Réponse publiée par shop mais pas appliquée : lag
   `rpk group describe poro-order-saga --brokers localhost:9092` et DLQ
   `rpk topic consume poro.shop.stock.reserved.dlq` (en-tête `x-poro-error`).

## Produit ou boutique absent du panier

Le catalogue local suit `shop.updated` et `product.updated`. Vérifier
`SELECT * FROM catalog_products WHERE id = '<product_id>';` et le lag de
`poro-order-catalog`. Une ligne `deleted_at` non nulle est définitive.

## Tâches planifiées

Toutes les `SCHEDULER_INTERVAL_MS` : expiration des commandes non payées,
clôture des commandes expédiées depuis `AUTO_COMPLETE_DAYS`, effacement des
coordonnées après `CONTACT_RETENTION_DAYS`. Logs `scheduled task done` /
`scheduled task failed`. Commandes en retard :

```sql
SELECT id, status, expires_at, shipped_at FROM orders
WHERE (status = 'awaiting_payment' AND expires_at < now() - interval '5 minutes')
   OR (status = 'shipped' AND shipped_at < now() - interval '8 days');
```

## DLQ `poro-order-saga`

- « unknown order » : un événement vise une commande absente de cette base.
- « does not match its total » : montant ou devise payés différents du total ;
  ne pas rejouer sans comprendre, vérifier côté payment.
- « cannot be paid while … » : paiement d'une commande payée à la livraison ou
  déjà expédiée.

## Paiement reçu après annulation

La commande passe `paid = true` et `order.cancelled` est republié avec
`paid: true` : payment doit rembourser. Suivre `refunded_at`.
