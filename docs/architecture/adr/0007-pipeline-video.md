# ADR-0007 — Pipeline vidéo (API + media-worker)

- Statut : accepté
- Date : 2026-09-30
- Décideurs : ingénierie

## Contexte

Le MVP social a besoin de clips courts (HLS) avant le feed (P0-7). ADR-0003
prévoit un déployable `content` (VIDEO + SOCIAL + FEED). Social et feed n'existent
pas encore ; le transcodage FFmpeg a un profil CPU incompatible avec une API
de lecture. `project.md` fixe le service vidéo sur le port 8083.

## Décision

1. **Deux binaires, un domaine, une base `poro_video`.**
   - `services/video` (Go/Fiber, port 8083) : init multipart présigné, complete,
     lecture, liste, suppression. Toute personne authentifiée (`PERSONAL`) peut
     uploader ; CREATOR n'est pas exigé.
   - `cmd/worker` (image `media-worker`, FFmpeg) : consomme `poro.video.uploaded`,
     transcode, écrit le statut et l'outbox `poro.video.ready` / `poro.video.failed`
     dans la **même** base. Pas d'HTTP public.
   - Fusion dans `content` abandonnée : voir [ADR-0008](0008-services-separes.md).

2. **Upload.** Multipart S3, parties de 8 MiB, plafond 256 MiB. Après complete
   le service relit l'objet (`Head` + 32 premiers octets) : MP4/MOV (`ftyp`) ou
   WebM (EBML). HTML/PDF rejetés. Durée max **3 minutes** (décidée au probe
   FFmpeg, pas sur la foi du client).

3. **HLS.** Échelle 360p toujours ; 720p seulement si la source a au moins
   720 lignes. Miniature JPEG. Clés déterministes
   `videos/{user_id}/{video_id}/…`. Le worker transcode **puis** claim l'inbox :
   un crash reprend ; un doublon Kafka n'écrit ready qu'une fois.

4. **Événements.** `poro.video.uploaded` et `poro.video.deleted` (API),
   `poro.video.ready` et `poro.video.failed` (worker). Codes publics d'échec :
   `too_long`, `invalid_media`, `transcode_failed`. stderr FFmpeg jamais renvoyé
   au client.
   - `poro.video.ready` porte aussi, depuis le 2026-10-07, `title`,
     `description`, `hashtags` (tags `#…` de la description, en minuscules,
     sans `#`, 20 au plus) et `published_at`. Ajout rétro-compatible : ces
     champs sont absents des événements plus anciens.
   - `poro.video.deleted` part dans la transaction du soft delete par le
     propriétaire, quel que soit le statut. L'abandon d'un upload (`DELETE
     /uploads/:id`) n'émet rien : la vidéo n'a jamais été visible.

5. **Visibilité.** Une vidéo `ready` est lisible sans JWT. Les autres statuts
   ne sont visibles que par le propriétaire (sinon 404, pas d'énumération).

## Conséquences

- Relais outbox `poro_video`, topics + DLQ créés par `redpanda-init`.
- Bunny CDN n'est pas encore devant HLS : les URLs pointent vers SeaweedFS
  (dev) ou l'endpoint S3 public.
- Les objets S3 ne sont pas effacés au soft-delete (dette P1).
