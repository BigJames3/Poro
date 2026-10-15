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

Les médias publics d'une vidéo retirée ou supprimée (HLS, miniature, sous
`videos/<user_id>/<video_id>/`) passent du bucket public `S3_BUCKET` au bucket
**privé** `S3_QUARANTINE_BUCKET` : copie puis suppression, objet par objet,
idempotent. La source n'est jamais publique : elle reste dans
`S3_SOURCE_BUCKET`. Le statut est enregistré avant le déplacement et l'inbox réclamée
après, donc une relivraison termine un déplacement interrompu. Le worker relit
le statut après avoir publié ses fichiers : une vidéo retirée ou supprimée
pendant son transcodage part aussi en quarantaine.

Seul le propriétaire voit une vidéo retirée (`moderation_status: removed`,
sans URL de média) ; les autres reçoivent `404 video_not_found`.

## Stockage

Trois buckets ([ADR-0015](../../docs/architecture/adr/0015-controle-acces-stockage-objet.md)),
accessibles avec l'identité S3 `poro-video` (`S3_VIDEO_*` dans `.env`) :

| Bucket | Visibilité | Contenu |
|---|---|---|
| `S3_BUCKET` (`poro-videos`) | lecture publique | HLS et miniatures |
| `S3_SOURCE_BUCKET` (`poro-video-sources`) | privé | Fichiers originaux, envoyés par parties présignées et conservés après transcodage |
| `S3_QUARANTINE_BUCKET` (`poro-quarantine`) | privé | Médias retirés ou supprimés |

`cmd/storage-migrate` (service compose `video-storage-migrate`) déplace une fois
les sources des versions précédentes vers `S3_SOURCE_BUCKET` ; il est idempotent.

## Variables d'environnement

Voir `internal/config/config.go`.

| Variable | Défaut | Rôle |
|---|---|---|
| `S3_SOURCE_BUCKET` | `poro-video-sources` | Bucket privé des sources ; doit différer de `S3_BUCKET` |
| `S3_QUARANTINE_BUCKET` | `poro-quarantine` | Bucket privé des médias retirés ou supprimés ; doit différer de `S3_BUCKET` |

## Tests

```powershell
go test ./... -count=1 -race
```

Postgres via testcontainers ; S3 via un serveur S3 en mémoire (`gofakes3`).
`TestS3AgainstSeaweedFS` tourne contre un vrai SeaweedFS avec contrôle d'accès,
lancé par `infra/storage/tests/run.sh` (ignoré sinon).

## Limites

- Le CDN (Bunny) peut servir des segments en cache jusqu'à leur expiration ;
  aucune purge automatique.
- La purge des uploads multipart abandonnés n'est pas encore faite (PORO-008).

## Sessions révoquées

Les access tokens des sessions révoquées par auth (déconnexion, « déconnecter les autres
appareils », réinitialisation du mot de passe, suppression du compte, vol de refresh token) sont
refusés grâce au registre partagé `REVOCATION_REDIS_URL`
([ADR-0016](../../docs/architecture/adr/0016-revocation-des-sessions.md)) : obligatoire hors dev,
réponse `503` s'il ne répond pas.
