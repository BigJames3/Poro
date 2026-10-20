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

## Complément 2026-10-15 — Isolation de FFmpeg (PORO-007)

FFmpeg décode des fichiers envoyés par n'importe quel utilisateur : c'est la surface d'attaque la
plus exposée du backend (failles de décodeurs, playlists et références externes qui font ouvrir
des URL ou des fichiers locaux, fichiers conçus pour épuiser mémoire ou CPU). Un conteneur Docker
par défaut ne suffit pas : il tourne avec des capacités Linux, un système de fichiers inscriptible,
sans limite de ressources, et le processus hérite de tous les secrets du worker.

**Décision : défense en profondeur, en quatre couches.**

1. **Avant FFmpeg (worker).** Le démuxeur est choisi par les octets magiques du fichier (`ftyp`
   → `mov`, EBML → `matroska`), revérifiés par le worker car l'objet a pu changer depuis l'API, et
   imposé avec `-f`. Le contenu du fichier ne peut donc jamais sélectionner HLS, concat, image2…
2. **Probe et listes blanches.** `ffprobe` (délai 30 s) doit confirmer le format, puis :
   codecs vidéo `h264`, `hevc`, `mpeg4`, `vp8`, `vp9`, `av1` ; audio `aac`, `mp3`, `opus`,
   `vorbis` ; dimensions 2 à 4096 px ; 4 flux audio/vidéo au plus, 16 flux au total. Les pochettes
   et pistes de données (métadonnées des téléphones) sont ignorées : seuls le premier flux vidéo et
   le premier flux audio sont décodés (`-map` explicite). Tout le reste : `invalid_media`.
3. **Invocation.** Pas de shell ; chemin passé en un seul argument préfixé `file:` ;
   `-protocol_whitelist file`, `-enable_drefs 0`, `-use_absolute_path 0` (références externes
   MOV) ; `-nostdin` ; `-threads 2` ; `-max_alloc 512 Mio` ; sortie forcée en `yuv420p`.
   L'environnement du processus est réduit à `PATH` : FFmpeg ne voit ni les clés S3 ni le mot de
   passe Postgres. Délai global de 10 minutes par vidéo ; un processus annulé est tué et
   l'attente de ses tubes bornée à 5 s.
4. **Conteneur.** UID 10001, `cap_drop: ALL`, `no-new-privileges`, racine en lecture seule,
   `/tmp` en tmpfs de 1 Gio `noexec,nosuid,nodev`, 2 Gio de mémoire sans swap, 2 CPU, 128
   processus, 1024 fichiers ouverts, pas de core dump, profil seccomp Docker par défaut, aucun
   volume. `infra/media-worker/check_sandbox.sh` vérifie ces réglages sur le conteneur lancé.

**Reprises.** Les lectures et écritures de stockage sont réessayées 3 fois (attente 1 s puis 2 s,
fichier source tronqué entre deux essais) ; au-delà la vidéo passe en `failed`, elle ne reste
jamais en `processing`. Le transcodage n'est jamais réessayé : un fichier qui fait échouer FFmpeg
le fera échouer encore.

**Écart avec la proposition initiale.** Le plafond « 4 flux » ne compte que les flux audio et
vidéo : les `.mov` d'iPhone portent souvent 2 à 4 pistes de métadonnées, qui auraient été refusées.

**Risques résiduels.**
- Le worker garde un accès réseau (Postgres, Kafka, S3). FFmpeg n'ouvre aucun protocole réseau,
  mais une faille d'exécution de code dans un décodeur permettrait d'atteindre ce réseau. La
  séparation complète (FFmpeg dans un conteneur ou un bac à sable sans réseau, type gVisor ou
  nsjail, alimenté par fichier) est la prochaine étape si le volume ou le modèle de menace l'exigent.
- Les limites de ressources s'appliquent au conteneur, pas à chaque processus FFmpeg ; le worker
  traite une vidéo à la fois, ce qui revient au même aujourd'hui.
- Les correctifs de sécurité FFmpeg arrivent avec l'image Alpine : la reconstruire régulièrement
  (Trivy en CI signale les CVE connues).
