# Runbook — relais outbox et DLQ

Le binaire `services/outbox-relay` lit `outbox_events` d'**une** base et publie
vers Redpanda. Compose démarre `outbox-relay-auth` (base `poro_auth`) et
`outbox-relay-user` (base `poro_user`).

## Santé

```powershell
docker compose logs -f outbox-relay-auth
# prêtes : GET http://<pod>:9100/health/ready (postgres + kafka)
# métriques : outbox_published_total, outbox_publish_failures_total, outbox_pending
```

Kafka n'est pas dans le ready d'auth/user : un courtier down n'arrête pas le login.
Les lignes s'accumulent dans `outbox_events` (`published_at IS NULL`).

## File d'attente trop longue

```sql
SELECT topic, count(*) FROM outbox_events WHERE published_at IS NULL GROUP BY 1;
SELECT id, topic, attempts, left(last_error, 200) FROM outbox_events
  WHERE published_at IS NULL ORDER BY attempts DESC LIMIT 20;
```

Causes fréquentes : topic absent (le relais n'auto-crée pas), Redpanda down,
payload rejeté. Créer le topic :

```powershell
docker compose exec redpanda rpk topic create poro.auth.user.created -p 3 --brokers redpanda:29092
```

Les topics et `.dlq` sont normalement créés par `redpanda-init`.

## Événements mis à l'écart (PORO-017)

Un événement que Kafka refuse **définitivement** (trop gros, invalide) n'est plus retenté en
boucle : le relais le met à l'écart (`last_error` commence par `rejected: `), métrique
`outbox_parked`, alerte `OutboxParked`. Les événements **suivants de la même entité** (même
topic et même clé) attendent derrière lui, pour que les consommateurs les reçoivent toujours dans
l'ordre ; toutes les autres entités continuent d'être publiées. Une panne passagère (broker
injoignable, leader en élection, topic absent) ne met jamais rien à l'écart : le lot est
retenté.

Limite de la garantie d'ordre : dans un même lot déjà envoyé, un événement plus récent de la même
clé peut être accepté par Kafka alors que l'ancien est refusé. Les lots suivants, eux, attendent.

```sql
SELECT id, topic, event_key, attempts, left(last_error, 300) FROM outbox_events
  WHERE published_at IS NULL AND starts_with(last_error, 'rejected: ');
```

Corriger la cause (taille maximale du topic, producteur qui écrit un payload invalide), puis
remettre en file :

```sql
UPDATE outbox_events SET last_error = NULL
  WHERE published_at IS NULL AND starts_with(last_error, 'rejected: ') AND id = '<uuid>';
```

Si l'événement ne doit jamais partir, le marquer publié **après décision humaine** (les
consommateurs ne le verront pas) : `UPDATE outbox_events SET published_at = now() WHERE id = '<uuid>';`

## Rejouer une DLQ

Les records DLQ gardent la **même clé** et le **même JSON**. En-têtes :
`x-poro-error`, `x-poro-original-topic`, `x-poro-original-partition`,
`x-poro-original-offset`, `x-poro-consumer-group`.

1. Lire l'erreur : `rpk topic consume poro.auth.user.created.dlq -n 10 --brokers localhost:9092`
2. Corriger la cause (schéma, bug consommateur, données).
3. Republier vers le topic d'origine avec `dlq-replay` (PORO-018). Il garde la clé, le JSON et
   donc l'`id` de chaque événement : l'inbox des consommateurs ignore les vrais doublons, et un
   rejeu fait deux fois est sans effet. Toujours commencer **sans `-apply`** (simulation : rien
   n'est publié) et garder la sortie, qui est la trace de l'opération :

   ```bash
   # Simulation : ce qui serait rejoué, une ligne JSON par événement, puis un résumé
   docker compose run --rm --entrypoint /app/dlq-replay outbox-relay-video \
     -dlq poro.video.ready.dlq -group poro-feed-projector | tee replay-dry.jsonl
   # Rejeu effectif, éventuellement limité à quelques événements
   docker compose run --rm --entrypoint /app/dlq-replay outbox-relay-video \
     -dlq poro.video.ready.dlq -group poro-feed-projector -event-id <uuid>,<uuid> -apply \
     | tee "replay-$(date -u +%Y%m%dT%H%M%SZ).jsonl"
   ```

   Filtres : `-group` (groupe qui a mis l'événement en DLQ), `-event-id` (liste), `-limit`.
   L'outil lit la DLQ du début jusqu'à sa fin au moment du lancement, sans groupe de
   consommateurs : il ne modifie ni ne vide la DLQ. Chaque événement rejoué porte l'en-tête
   `x-poro-replayed-from: <dlq>/<partition>/<offset>`.
4. Ne pas vider la DLQ avant d'avoir un export (rétention 30 jours en local).

Un replay aveugle d'un poison pill (JSON illisible) le renverra en DLQ.

## Rétention outbox

Les lignes **publiées** de plus de 72 h sont effacées par le relais. Les
pending ne sont jamais effacés automatiquement.

## Démarrage refusé : `APP_ENV is required`

`APP_ENV` n'a pas de valeur par défaut, pas plus que le mot de passe Postgres ou les clés S3 :
un oubli arrête le service au lieu de le lancer avec les raccourcis du mode dev (TLS facultatif,
Redis facultatif, secrets de développement). Fixer `APP_ENV=dev`, `staging` ou `prod` et fournir
les secrets (compose, `.env` copié de `.env.example`, ou gestionnaire de secrets).
