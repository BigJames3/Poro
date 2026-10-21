# Spec — enregistrer un média sur l’appareil

« Enregistrer » = écrire un média **déjà public** dans la galerie. Ce n’est **jamais** la publication.

Composant cible unique : `SaveMediaHost` + `SaveMediaViewModel` (noms indicatifs). Même machine d’états pour toutes les surfaces qui l’exposeront.

## 1. Matrice de faisabilité

| Média | URL publique | Fichier unique | Enregistrer dans l’app | Motif |
|---|---|---|---|---|
| Vidéo HLS (`hls_url`) | Oui (playlist + segments) | **Non** | **Non livrable** | Pas de MP4 ; original privé (B-VID-SAVE) |
| Original uploadé par l’auteur | Non | Oui côté serveur | **Interdit** | `poro-video-sources` privé ; EXIF |
| Miniature vidéo | Oui (JPEG) | Oui | Non proposé | Confusion avec « enregistrer la vidéo » |
| Photo produit / logo | Oui (WebP) | Oui | **Possible** si Q-MED-01 | GET anonyme = déjà récupérable |
| Avatar | Oui (WebP) | Oui | Non proposé v1 | L’auteur l’a déjà ; tiers : Q-MED-01 étendu plus tard |
| Média `removed` / quarantaine | Non | — | Indisponible | Pas d’URL |
| URL présignée d’upload | Privée, TTL 300 s | — | **Interdit** | Ne jamais persister |

Tant que B-VID-SAVE n’est pas levé : **aucun bouton** « Enregistrer » sur le fil, la vidéo isolée ou Mes vidéos.

Si Q-MED-01 = non : aucun bouton Enregistrer nulle part. Les états ci-dessous restent la spec d’interface pour le jour où une surface est activée.

## 2. Placement (quand l’action est autorisée)

| Surface | Contrôle | Label | Commentaire |
|---|---|---|---|
| PDP, photo courante du pager | ⋮ ou bouton icône `Download` **renommé** « Enregistrer » | `media_save_photo` | Uniquement si Q-MED-01 |
| Éditeur produit / vitrine vendeur | Pas nécessaire | — | Le vendeur a le fichier |
| Overlay fil | — | — | Interdit (vidéo) |
| Publier | — | — | Confusion avec l’import |

Icône Material `Download` acceptable **si** le `contentDescription` et le texte visible disent « Enregistrer la photo ». Jamais « Télécharger la vidéo » à côté de Publier.

## 3. Machine d’états

Une sauvegarde à la fois **par média** (clé = URL canonique). Un second tap pendant `InProgress` est ignoré (anti double écriture).

```
Idle ──tap──► Permission? ──refus──► PermissionDenied
                 │ accord / inutile
                 ▼
              CheckSpace ──insuffisant──► StorageFull
                 │
                 ▼
              InProgress ──200 + insert──► Success (snackbar 4 s) ──► Idle
                 │
                 ├ réseau / HTTP 5xx / timeout ──► FailedNetwork (Réessayer)
                 ├ HTTP 404 / 403 / plus d’URL ──► Unavailable (pas de retry infini)
                 ├ insert MediaStore échoue ──► FailedWrite
                 └ utilisateur quitte l’écran ──► annuler la coroutine (pas de file système)
```

Pas de notification barre d’état. Pas de `DownloadManager` (il enregistrerait `master.m3u8` pour une vidéo, ou un WebP sans contrôle d’état UI). Pas de reprise après kill du process.

### UI par état

| État | Visuel | TalkBack |
|---|---|---|
| Idle | Icône + « Enregistrer la photo » | idem |
| InProgress | Spinner sur l’icône, label « Enregistrement… » | « Enregistrement de la photo en cours » |
| Success | Snackbar `media_save_done` | Annoncé par le snackbar |
| FailedNetwork | Snackbar + action Réessayer | « Échec réseau. Réessayer » |
| PermissionDenied | Snackbar + action Paramètres | « Autorisation refusée » |
| StorageFull | Snackbar `media_save_storage` | « Espace de stockage insuffisant » |
| Unavailable | Snackbar `media_save_unavailable` | « Média indisponible » |
| FailedWrite | Snackbar `media_save_write` | « Impossible d’enregistrer » |

Images publiques ré-encodées : quelques centaines de Ko typiquement, 5 Mio max à l’origine. Une barre de **pourcentage** n’est pas obligatoire ; un état indéterminé suffit. Si le `Content-Length` est connu, une barre déterminée est un plus, pas un AC.

Fichiers volumineux / liaison instable : pertinent pour une **vidéo**. Comme l’enregistrement vidéo est bloqué, ne pas dessiner un écran « 48 % de 180 Mo » aujourd’hui. Le documenter comme **cible** si un MP4 public apparaît :

| État cible (futur MP4) | UI |
|---|---|
| InProgress déterminé | « Enregistrement de la vidéo… 48 % » |
| FailedNetwork | Réessayer **la même** URL, pas un second fichier parallèle |
| Process mort | Message au prochain ouverture : « L’enregistrement a été interrompu. » — **seulement** si WorkManager est alors choisi et livré |

Ne pas livrer ces copies tant que B-VID-SAVE tient.

## 4. Permissions (au tap seulement)

```
si API ≤ 28 et Q-MED-01 :
    demander WRITE_EXTERNAL_STORAGE
    refus définitif → snackbar + lien vers les paramètres de l’app
sinon :
    aucun dialogue
```

Manifest (le jour de l’implémentation) :

```xml
<uses-permission
    android:name="android.permission.WRITE_EXTERNAL_STORAGE"
    android:maxSdkVersion="28" />
```

Ne pas ajouter `READ_*`. Ne pas demander au splash.

Photo Picker (import) reste indépendant : **zéro** permission.

## 5. Espace de stockage

Avant l’écriture : `StorageManager` / `getUsableSpace` sur le volume primaire. Si l’espace libre est inférieur à `Content-Length` (ou à 5 Mio si inconnu) + marge 1 Mio → `StorageFull` sans GET du fichier.

Pendant l’écriture : `IOException` → `FailedWrite` ou `StorageFull` selon le message / `ErrnoException` `ENOSPC` quand disponible.

## 6. Médias protégés ou absents

| Cas | Détection | UI |
|---|---|---|
| Vidéo non `ready` | Pas d’URL | Bouton absent |
| Retirée / 404 | GET 404 ou DTO sans URL | `Unavailable` |
| Quarantaine | Pas d’URL publique | Bouton absent |
| HLS seulement | Type vidéo | Bouton absent (blocage) |
| URL présignée upload | Préfixe / query signature | Ne jamais proposer Enregistrer |

Pas de DRM / token de lecture dans le contrat actuel. Les objets publics sont anonymes : l’app ne « protège » pas mieux qu’un navigateur. On n’expose simplement **pas** les originaux privés.

## 7. Copies

| Clé | Texte |
|---|---|
| `media_save_photo` | Enregistrer la photo |
| `media_save_saving` | Enregistrement… |
| `media_save_done` | Photo enregistrée dans la galerie. |
| `media_save_retry` | Réessayer |
| `media_save_network` | Enregistrement impossible. Vérifiez le réseau. |
| `media_save_permission` | Autorisez l’accès au stockage pour enregistrer la photo. |
| `media_save_storage` | Espace de stockage insuffisant. |
| `media_save_unavailable` | Ce média n’est plus disponible. |
| `media_save_write` | Impossible d’enregistrer la photo. |

Interdit : « Téléchargement réussi », « Vidéo hors ligne », « Partager le fichier ».

## 8. Distinction visuelle d’avec Publier

| | Publier / Envoyer | Enregistrer |
|---|---|---|
| Direction | Appareil → Poro | Poro → galerie |
| Écran type | Onglet Publier, éditeur produit | Fiche / galerie d’un média **déjà** en ligne |
| CTA primaire | `SubmitButton` « Publier » / « Envoyer cette photo » | Action secondaire ⋮ ou icon button |
| Progression | « Envoi de … % » | « Enregistrement… » |
| Succès | « Votre vidéo est en ligne » / photo dans la rangée | « Photo enregistrée dans la galerie » |

Un même écran ne porte les deux CTA que si les deux sens existent (ex. éditeur : Envoyer une photo locale). Sur le fil : Publier est un **onglet**, Enregistrer est absent.

## 9. Ce que cette spec ne promet pas

- Téléchargement une fois l’app tuée.
- File système / pause / reprise HTTP multi-parties pour un MP4.
- Lecture hors ligne dans ExoPlayer.
- `ACTION_SEND` d’un `video/mp4` ou `image/webp` (le share reste du texte).
- Enregistrement de l’original qualité upload.
