# Runbook — service video + media-worker

Upload multipart, transcodage HLS, miniatures. Port API **8083**.
Contrat : [`docs/api/video.openapi.yaml`](../api/video.openapi.yaml).
Décision : [ADR-0007](../architecture/adr/0007-pipeline-video.md).

## Santé

| Probe | Sens |
|---|---|
| `GET /health/live` | Processus HTTP up |
| `GET /health/ready` | Postgres. **Pas Kafka**, pas S3 |
| `GET /metrics` | Prometheus ; ne pas exposer sur l'ingress public |

Le worker n'a pas d'HTTP : un crash redémarre le conteneur. Un courtier down :
les completes s'accumulent en outbox ; le worker rattrape.

## Upload coincé en `uploading`

Le client n'a pas PUT toutes les parties (URLs valables 1 h) ou n'a pas appelé
`POST /uploads/:id/complete`. Relancer `POST /api/v1/videos/uploads`.
`DELETE /uploads/:id` abandonne le multipart.

## Complete → `invalid_media`

Les 32 premiers octets ne sont pas un MP4/MOV (`ftyp`) ni un WebM. Le fichier
est supprimé de S3. Demander un vrai clip.

## Statut `processing` qui ne passe pas à `ready`

1. Outbox video : `SELECT * FROM outbox_events WHERE event_key = '<video_id>';`
   dans `poro_video`. `published_at` null → relais video (runbook outbox).
2. Topic : `rpk topic consume poro.video.uploaded --brokers localhost:9092`.
3. Inbox worker : `SELECT * FROM processed_events WHERE consumer = 'poro-media-transcode';`
4. Logs `poro-media-worker`. `last_error` (colonne interne) n'est **jamais**
   dans l'API. Codes publics : `too_long` (plus de 180 s), `invalid_media`,
   `transcode_failed`.
5. DLQ : `poro.video.uploaded.dlq`.

## HLS 404 après `ready`

Bucket `poro-videos` créé ? (`seaweedfs-init`). Clé
`videos/<user_id>/<video_id>/hls/master.m3u8`. L'URL publique utilise
`S3_PUBLIC_ENDPOINT` (localhost:9000 en dev).

## Vidéo retirée par la modération ou supprimée

`SELECT moderation_status, moderated_at, deleted_at FROM videos WHERE id = '<id>';`
Les médias d'une vidéo `removed` ou supprimée sont dans le bucket privé
`poro-quarantine` (mêmes clés, préfixe `videos/<user_id>/<video_id>/`). Le
groupe `poro-video-moderation` les déplace ; une restauration
(`poro.moderation.content.restored`) les remet dans `poro-videos`, sauf pour une
vidéo supprimée entre-temps.

- Médias encore publics après un retrait : lag du groupe
  `rpk group describe poro-video-moderation --brokers localhost:9092`, puis la DLQ
  `rpk topic consume poro.moderation.content.removed.dlq` (en-tête `x-poro-error`).
  Un échec S3 est retenté (le statut est déjà enregistré, le déplacement est idempotent).
- Le CDN peut servir les segments déjà en cache jusqu'à l'expiration de leur TTL :
  purger le préfixe dans Bunny si l'urgence l'exige.

## FFmpeg absent

L'image worker installe `ffmpeg`. En local : `ffmpeg` et `ffprobe` sur le PATH,
ou `FFMPEG_BIN` / `FFPROBE_BIN`.
