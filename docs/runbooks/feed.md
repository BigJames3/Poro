# Runbook — service feed

Port 8084, base `poro_feed`, Redis DB 4, groupe Kafka `poro-feed-projector`.

## Sondes

`/health/ready` vérifie Postgres et Redis. Kafka n'en fait pas partie : les flux
continuent de servir le dernier état projeté pendant une panne du broker.
Redis en panne : following et trending restent servis (cache contourné),
For You répond `503 unavailable`.

## Une vidéo n'apparaît pas

1. `SELECT video_id, deleted_at, moderation_status, published_at FROM videos WHERE video_id = '<id>';`
2. Absente → retard ou erreur du consommateur :
   `rpk group describe poro-feed-projector --brokers localhost:9092` (lag),
   DLQ `rpk topic consume poro.video.ready.dlq`.
3. Following : l'abonnement est-il projeté ?
   `SELECT * FROM follows WHERE follower_id = '<viewer>' AND following_id = '<auteur>';`
4. Page 1 en cache 60 s : attendre une minute ou supprimer
   `feed:cache:following:<viewer>:<limit>` en Redis DB 4.

## Tendances figées

Le ticker de 5 minutes journalise `trending recompute failed` en cas d'échec.
Recalcul manuel équivalent :

```sql
SELECT count(*) FROM video_stats WHERE trending_score > 0;
```

puis redémarrer le service (le ticker recalcule au démarrage).

## Compteurs différents de social

Le feed est une projection : quelques secondes de retard sont normales. Un
écart durable vient d'un événement en DLQ (`poro.social.*.dlq`). Social reste
la référence.

## Lenteur de /feed/following

Voir le seuil de bascule vers le fan-out à l'écriture dans
[ADR-0009](../architecture/adr/0009-feed-v1.md) (p95 > 150 ms).
