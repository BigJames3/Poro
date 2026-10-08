# Runbook — service search

Port 8088, base `poro_search`, Redis DB 7, groupe Kafka `poro-search-indexer`.

## Sondes

`/health/ready` (aussi `/api/v1/health`) vérifie Postgres et Redis. Kafka n'en
fait pas partie : la recherche sert le dernier état indexé pendant une panne
du broker.

## Une vidéo n'est pas trouvée

1. `SELECT published_at, deleted_at, moderation_status FROM videos WHERE video_id = '<id>';`
   Elle doit avoir `published_at`, pas de `deleted_at` et `approved`.
2. Absente : lag `rpk group describe poro-search-indexer --brokers localhost:9092`,
   DLQ `rpk topic consume poro.video.ready.dlq` (en-tête `x-poro-error`).
3. Présente : tester la requête en SQL :
   `SELECT document @@ websearch_to_tsquery('search_fr', '<q>') FROM videos WHERE video_id = '<id>';`

## Un hashtag a un mauvais compte

`hashtags.videos_count` suit chaque changement de visibilité. Recalcul complet :

```sql
UPDATE hashtags h SET videos_count = coalesce((
  SELECT count(*) FROM videos v
  WHERE h.tag = ANY (v.hashtags) AND v.published_at IS NOT NULL
    AND v.deleted_at IS NULL AND v.moderation_status = 'approved'), 0);
```

## Lenteur

Voir le seuil de bascule vers OpenSearch dans
[ADR-0011](../architecture/adr/0011-search-postgres-v1.md) (p95 > 200 ms).
Vérifier d'abord les plans : `EXPLAIN ANALYZE` doit utiliser
`videos_document_idx` ou `videos_title_trgm_idx`.
