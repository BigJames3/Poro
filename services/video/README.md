# Video Service

Upload multipart présigné, métadonnées et lecture des vidéos PORO
([ADR-0007](../../docs/architecture/adr/0007-pipeline-video.md)). Le transcodage
HLS tourne dans `media-worker` (même module, `cmd/worker`).

- Contrat HTTP : [`docs/api/video.openapi.yaml`](../../docs/api/video.openapi.yaml)
- Exploitation : [`docs/runbooks/video.md`](../../docs/runbooks/video.md)

## Événements

| Sens | Topic | Groupe | Effet |
|---|---|---|---|
| Produit | `poro.video.uploaded`, `poro.video.deleted` | — | Outbox, relais `outbox-relay-video` |
| Produit (worker) | `poro.video.ready`, `poro.video.failed` | — | Outbox |
| Consommé (worker) | `poro.video.uploaded` | `poro-media-transcode` | Transcodage HLS, miniature |
| Consommé (API) | `poro.moderation.content.removed` (vidéo) | `poro-video-moderation` | `moderation_status = removed`, médias en quarantaine |
| Consommé (API) | `poro.moderation.content.restored` | `poro-video-moderation` | `approved`, médias remis en ligne (sauf vidéo supprimée) |
| Consommé (API) | `poro.video.deleted` | `poro-video-moderation` | Médias de la vidéo supprimée en quarantaine |

## Quarantaine des médias

Les médias d'une vidéo retirée ou supprimée (source, HLS, miniature, sous
`videos/<user_id>/<video_id>/`) passent du bucket public `S3_BUCKET` au bucket
**privé** `S3_QUARANTINE_BUCKET` : copie puis suppression, objet par objet,
idempotent. Le statut est enregistré avant le déplacement et l'inbox réclamée
après, donc une relivraison termine un déplacement interrompu. Le worker relit
le statut après avoir publié ses fichiers : une vidéo retirée ou supprimée
pendant son transcodage part aussi en quarantaine.

Seul le propriétaire voit une vidéo retirée (`moderation_status: removed`,
sans URL de média) ; les autres reçoivent `404 video_not_found`.

## Variables d'environnement

Voir `internal/config/config.go`. Ajout de cette version :

| Variable | Défaut | Rôle |
|---|---|---|
| `S3_QUARANTINE_BUCKET` | `poro-quarantine` | Bucket privé des médias retirés ou supprimés ; doit différer de `S3_BUCKET` |

## Tests

```powershell
go test ./... -count=1 -race
```

Postgres via testcontainers ; S3 via un serveur S3 en mémoire (`gofakes3`).

## Limites

- Le CDN (Bunny) peut servir des segments en cache jusqu'à leur expiration ;
  aucune purge automatique.
- En local, SeaweedFS n'a pas d'authentification : `poro-quarantine` y reste
  lisible. La protection repose sur un bucket privé en production.
