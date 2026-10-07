# Runbook — service social

Port 8085, base `poro_social`, Redis DB 3, groupe Kafka `poro-social-projections`.

## Sondes

- `GET /health/live` : processus vivant.
- `GET /health/ready` (alias `/health`, `/api/v1/health`) : Postgres et Redis.
  Kafka n'en fait pas partie : likes et commentaires continuent pendant une
  panne du broker, leurs événements attendent dans l'outbox.

## « video_not_found » sur une vidéo pourtant prête

1. La projection : `SELECT * FROM videos_projection WHERE video_id = '<id>';`
2. Absente → le consommateur a-t-il vu l'événement ?
   `rpk group describe poro-social-projections --brokers localhost:9092` (lag),
   puis `rpk topic consume poro.video.ready --brokers localhost:9092 -n 20`.
3. Statut `deleted` → la vidéo a été supprimée par son propriétaire ; c'est voulu.
4. Événement en DLQ : `rpk topic consume poro.video.ready.dlq` (en-tête
   `x-poro-error`).

## « user_not_found » sur un follow

Le compte n'est connu que par `poro.auth.user.created` ou par un appel
authentifié à social. `SELECT * FROM users_projection WHERE user_id = '<id>';`
Un compte plus ancien que la rétention du topic apparaît à son premier appel.

## Compteurs suspects

Les compteurs changent dans la transaction de l'action. Recalcul pour une vidéo :

```sql
SELECT (SELECT count(*) FROM likes WHERE video_id = '<id>') AS likes,
       (SELECT count(*) FROM comments WHERE video_id = '<id>' AND deleted_at IS NULL) AS comments,
       (SELECT count(*) FROM shares WHERE video_id = '<id>') AS shares;
SELECT * FROM video_counters WHERE video_id = '<id>';
```

Corriger un écart à la main dans `video_counters` si besoin et ouvrir un ticket :
un écart signifie un bug.

## Événements non publiés

`SELECT count(*) FROM outbox_events WHERE published_at IS NULL;` qui grossit →
relais `outbox-relay-social` (voir [outbox-relay](outbox-relay.md)).

## 429 en masse

Limites par utilisateur et par minute (likes 60, commentaires 10, follows 30,
partages 30, toutes routes 120). Clés Redis `social:rl:*` en DB 3. Derrière un
reverse proxy, vérifier `TRUSTED_PROXIES` : sinon tous les anonymes partagent
l'IP du proxy.
