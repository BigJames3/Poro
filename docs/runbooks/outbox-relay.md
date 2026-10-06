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

## Rejouer une DLQ

Les records DLQ gardent la **même clé** et le **même JSON**. En-têtes :
`x-poro-error`, `x-poro-original-topic`, `x-poro-original-partition`,
`x-poro-original-offset`, `x-poro-consumer-group`.

1. Lire l'erreur : `rpk topic consume poro.auth.user.created.dlq -n 10 --brokers localhost:9092`
2. Corriger la cause (schéma, bug consommateur, données).
3. Republier vers le topic d'origine **sans changer l'`id`** : l'inbox ignore
   les vrais doublons. Ne pas générer un nouvel `id`.
4. Ne pas vider la DLQ avant d'avoir un export (rétention 30 jours en local).

Un replay aveugle d'un poison pill (JSON illisible) le renverra en DLQ.

## Rétention outbox

Les lignes **publiées** de plus de 72 h sont effacées par le relais. Les
pending ne sont jamais effacés automatiquement.
