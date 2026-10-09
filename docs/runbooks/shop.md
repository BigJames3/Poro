# Runbook — service shop

Port 8090, base `poro_shop`, Redis DB 8 (limites de débit), bucket `poro-shop`,
groupe Kafka `poro-shop-stock`, relais `outbox-relay-shop`.

## Sondes

`/health/ready` vérifie Postgres et Redis. Kafka n'en fait pas partie : les
boutiques et produits restent servis pendant une panne du broker ; les
commandes attendent dans Kafka et sont traitées au retour.

## Une commande reste sans réponse de stock

1. Réponse déjà donnée ?
   `SELECT status, reason, created_at FROM reservations WHERE order_id = '<id>';`
   - `held` : `stock.reserved` est dans l'outbox ; voir le relais.
   - `rejected` : `stock.rejected` est dans l'outbox, avec la raison.
   - `cancelled` : l'annulation est arrivée avant la commande ; rien n'est retenu.
2. Événement publié ?
   `SELECT topic, published_at, attempts, last_error FROM outbox_events WHERE event_key = '<id>';`
   Un `published_at` vide avec des erreurs : voir le runbook outbox-relay.
3. Aucune ligne : lag `rpk group describe poro-shop-stock --brokers localhost:9092`
   et DLQ `rpk topic consume poro.order.order.created.dlq` (en-tête `x-poro-error`).

## Stock réservé bloqué

Le stock réservé (`variants.stock_reserved`) n'est libéré que par
`poro.order.order.cancelled` ou consommé par `poro.order.order.completed`.
Lister les réservations anciennes :

```sql
SELECT order_id, shop_id, created_at FROM reservations
WHERE status = 'held' AND created_at < now() - interval '2 days'
ORDER BY created_at;
```

Vérifier l'état de la commande dans le service order avant toute action. Ne
jamais modifier `stock_reserved` à la main : republier l'annulation depuis
order pour garder les deux services cohérents.

## `poro.order.order.completed` en DLQ

« completed without held stock » : la commande n'a pas de réservation `held`
(rejetée, libérée ou inconnue). C'est une incohérence entre order et shop à
investiguer ; le stock n'a pas été touché.

## Suspendre une boutique

`POST /api/v1/admin/shops/<shop_id>/suspend` avec un jeton ADMIN ou MODERATOR.
La boutique disparaît du public, refuse les nouvelles réservations, et le
vendeur ne peut plus modifier ses produits. Les commandes déjà réservées
suivent leur cours. `.../reinstate` rétablit la boutique.

## Photos

Les uploads bruts sont sous `uploads/<owner_id>/` et supprimés après
ré-encodage ; un upload jamais confirmé reste orphelin. Prévoir une règle de
cycle de vie du bucket (expiration à 1 jour sur `uploads/`).
