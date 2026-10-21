# Tests d’acceptation — médias

Légende : **OK** = déjà vrai dans l’app actuelle (à ne pas régresser) · **Cible** = lot M* après autorisation Android · **Bloqué** = ne pas tester comme livré.

Les AC s’appuient sur le contrat réel, pas sur un backend fictif.

## AC-SOC — Réseau social

| ID | Critère | Statut | Preuve / lot |
|---|---|---|---|
| AC-SOC-01 | Tant que B-IMG-POST : picker Publier = `VideoOnly` (pas d’image vers `/videos/uploads`) | OK | `UploadScreen` |
| AC-SOC-02 | Une vidéo MP4/MOV/WebM ≤ 256 Mo et ≤ 3 min peut être sélectionnée | OK | `UploadViewModel` + tests |
| AC-SOC-03 | AVI / trop lourde / trop longue : erreur **avant** `POST /uploads` | OK | `UploadViewModelTest` |
| AC-SOC-04 | Envoi : % , succès (`ready`) et échec (`Failed` / `failed`) visibles | OK | `UploadStatusCard` |
| AC-SOC-05 | Réseau pendant l’envoi : état Failed + possibilité de republier | OK | `UploadManager` |
| AC-SOC-06 | Un second Publier pendant `Uploading` ne démarre pas un 2ᵉ envoi | OK | `isBusy` |
| AC-SOC-07 | Après contrat image : picker vidéo **ou** photo ; branche MIME ; image **sans** player | Bloqué | lot M5 |
| AC-SOC-08 | Après contrat : le fil affiche une page vidéo **ou** une page image selon `kind` | Bloqué | lot M5 |
| AC-SOC-09 | ⋮ du fil = Signaler, pas Enregistrer | OK | `VideoActions` |
| AC-SOC-10 | Description limitée à 500 runes (limite serveur) | Cible | lot M1 |
| AC-SOC-11 | Copies « Publier / Envoi » jamais « Télécharger » | OK + revue strings | M0 |
| AC-SOC-12 | Pas de carte image fictive (miniature, avatar ou photo boutique présentés comme un post) | OK | à préserver jusqu’à M5 |

## AC-MX-IMG — Marketplace images

| ID | Critère | Statut | Lot |
|---|---|---|---|
| AC-MX-IMG-01 | Picker produit = `ImageOnly` | OK | préserver |
| AC-MX-IMG-02 | Picker logo = `ImageOnly` | OK | préserver |
| AC-MX-IMG-03 | Aucun écran `ui/shop` n’instancie un player vidéo | OK | préserver |
| AC-MX-IMG-04 | 11ᵉ photo refusée localement (`too_many_images`) | OK | lot 1 déjà |
| AC-MX-IMG-05 | MIME hors jpeg/png/webp refusé avant upload | OK | `ShopViewModelsTest` |
| AC-MX-IMG-06 | Fichier > 5 Mo refusé avant upload | OK | idem |
| AC-MX-IMG-07 | Erreurs format et taille ont des messages **distincts** | Cible | M2 |
| AC-MX-IMG-08 | Après pick : aperçu local, Retirer et Envoyer **avant** le POST | Cible | M2 |
| AC-MX-IMG-09 | Photo confirmée visible (éditeur 96 dp, PDP pager, carte 1:1) | OK | préserver + clip design |
| AC-MX-IMG-10 | Retrait d’une photo confirmée (sauf dernière d’un `active`) | OK | DELETE existant |
| AC-MX-IMG-11 | `image_required` si publication sans photo | OK | serveur + string |
| AC-MX-IMG-12 | HEIC → message format, zéro requête | Cible (déjà refusé, copie à préciser) | M2 |
| AC-MX-IMG-13 | Envoi image : pas de double POST (`working` / pending) | OK / renforcer | M2 |
| AC-MX-IMG-14 | Réseau pendant envoi image : aperçu conservé + Réessayer | Cible | M2 |
| AC-MX-IMG-15 | Compteur « n / 10 » visible | Cible | M2 |

## AC-SAV — Enregistrer

| ID | Critère | Statut | Lot |
|---|---|---|---|
| AC-SAV-01 | Aucun bouton Enregistrer sur une vidéo | **Bloqué / à vérifier OK** | M0 revue |
| AC-SAV-02 | Si Q-MED-01 = non : aucun bouton Enregistrer photo | Cible | M0 |
| AC-SAV-03 | Si Q-MED-01 = oui : Idle → InProgress → Success snackbar | Cible | M3 |
| AC-SAV-04 | Second tap pendant InProgress sans 2ᵉ fichier | Cible | M3 |
| AC-SAV-05 | Coupe réseau → FailedNetwork + Réessayer | Cible | M3 |
| AC-SAV-06 | 404 / plus d’URL → Unavailable, pas de retry auto | Cible | M3 |
| AC-SAV-07 | API 24–28 : permission au tap seulement ; refus → snackbar | Cible | M3 |
| AC-SAV-08 | Espace insuffisant → StorageFull sans GET inutile si détectable | Cible | M3 |
| AC-SAV-09 | L’enregistrement n’envoie **rien** vers `/videos/uploads` ni shop upload | Cible | M3 |
| AC-SAV-10 | TalkBack : « Enregistrer la photo », jamais « Télécharger » | Cible | M3 |
| AC-SAV-11 | Enregistrer une vidéo HLS / un original : **non implémenté**, pas d’écran factice | Bloqué | — |

## NR — Non-régression (obligatoire à chaque lot M)

| ID | Parcours |
|---|---|
| NR-01 | Fil : like, commentaire, share texte |
| NR-02 | Publier une vidéo compatible jusqu’à `processing` |
| NR-03 | Signalement ⋮ (lots 1–3) |
| NR-04 | Panier COD / checkout (lot 1) |
| NR-05 | PDP images + add to cart |
| NR-06 | Avatar ImageOnly |

## Hors tests (ne pas écrire de cas)

- Lecture hors ligne.
- WorkManager upload ou save.
- Partage d’un fichier média.
- Post image social.
- Capture caméra.
- Staff / file modération.
- Wave, catalogue, GPS.
