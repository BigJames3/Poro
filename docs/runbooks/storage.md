# Runbook — Stockage objet (SeaweedFS)

Décision : [ADR-0015](../architecture/adr/0015-controle-acces-stockage-objet.md).

## Ce qui tourne

| Service compose | Rôle |
|---|---|
| `seaweedfs` | Serveur S3, sur `127.0.0.1:9000` (S3), `9001` (master), `9002` (filer) |
| `seaweedfs-init` | `infra/storage/init.sh` : buckets, identités S3, nettoyage ; à chaque `up` |
| `video-storage-migrate` | Déplace les anciennes sources vidéo vers `poro-video-sources` ; à chaque `up` |

## Buckets et identités

| Bucket | Visibilité | Écrit par |
|---|---|---|
| `poro-videos` | lecture publique (HLS, miniatures) | `poro-video` |
| `poro-video-sources` | privé | `poro-video` |
| `poro-quarantine` | privé | `poro-video` |
| `poro-avatars` | lecture publique | `poro-user` |
| `poro-avatar-uploads` | privé | `poro-user` |
| `poro-shop` | lecture publique | `poro-shop` |
| `poro-shop-uploads` | privé | `poro-shop` |

Identifiants dans `.env` : `S3_USER_*`, `S3_VIDEO_*`, `S3_SHOP_*` (`ACCESS_KEY_ID` et
`SECRET_ACCESS_KEY`). Ils doivent être distincts, d'au moins 8 caractères parmi
`A-Z a-z 0-9 . _ ~ + / = -`.

## Opérations courantes

### Après une mise à jour depuis une version sans contrôle d'accès

Ajouter les six variables à `.env` (voir `.env.example`), puis :

```powershell
docker compose up -d
docker compose logs seaweedfs-init video-storage-migrate
```

Attendu :

- `storage-init: buckets and S3 access control are configured` ;
- `legacy video sources moved to the private bucket` avec le nombre de sources déplacées (0 aux
  démarrages suivants).

### Changer un secret (rotation)

1. Modifier la variable dans `.env`.
2. Lancer `docker compose up -d seaweedfs-init`, qui applique la configuration.
3. Lancer `docker compose up -d --force-recreate <service>` pour le service concerné (`user`,
   `shop`, ou `video` et `media-worker`).

Entre les étapes 2 et 3, le service concerné reçoit des `403` sur S3 : enchaîner sans attendre.

### Vérifier la configuration appliquée (sans secrets)

```powershell
docker compose exec seaweedfs sh -c 'echo s3.config.show | weed shell'
```

`s3.iam.export` affiche les secrets : ne pas l'utiliser dans un ticket ou un log partagé.

### Lire un objet privé pour un diagnostic

Utiliser l'identifiant du service propriétaire du bucket, par exemple avec l'AWS CLI :

```powershell
$env:AWS_ACCESS_KEY_ID = "<S3_VIDEO_ACCESS_KEY_ID>"; $env:AWS_SECRET_ACCESS_KEY = "<S3_VIDEO_SECRET_ACCESS_KEY>"
aws --endpoint-url http://127.0.0.1:9000 s3 ls s3://poro-video-sources/videos/ --recursive
```

## Incidents

| Symptôme | Cause probable | Action |
|---|---|---|
| Compose refuse de démarrer : `S3_USER_ACCESS_KEY_ID must be set` | `.env` d'avant ADR-0015 | Ajouter les six variables |
| `seaweedfs-init` sort en erreur `each service needs its own S3 access key id` | Deux services partagent un identifiant | Rendre les identifiants distincts |
| `user`, `shop` ou `video` reçoivent `403 AccessDenied` | Secret modifié sans relancer `seaweedfs-init`, ou l'inverse | Rotation, étapes 2 et 3 |
| Une miniature ou un HLS renvoie `403` | Objet absent du bucket public : vidéo en quarantaine, ou mauvais bucket | Vérifier `moderation_status` et le bucket |
| Un upload d'avatar ou de photo échoue juste après la mise à jour | Original supprimé par le nettoyage d'`init.sh` | Recommencer l'upload |
| Un upload vidéo commencé avant la mise à jour échoue à `complete` | Son upload multipart est resté dans `poro-videos` | Recommencer l'upload ; l'ancien multipart sera purgé avec PORO-008 |
| `video-storage-migrate` échoue | SeaweedFS indisponible ou identifiant vidéo refusé | Lire ses logs, corriger, relancer `docker compose up -d` (idempotent) |

## Tests

```bash
pip install -r infra/storage/tests/requirements.txt
(cd services/user && npm ci) && (cd services/shop && npm ci)
infra/storage/tests/run.sh
```

Le script lance SeaweedFS avec l'image de `docker-compose.yml` et applique le vrai `init.sh`,
puis vérifie :

- chaque droit de chaque identité et de l'anonyme ;
- la migration ;
- la rotation des clés ;
- le refus d'une clé partagée ;
- l'absence de secrets dans les logs ;
- le code de stockage de video, user et shop contre ce SeaweedFS.

Port de test : `127.0.0.1:18333` (`STORAGE_TEST_PORT`).
