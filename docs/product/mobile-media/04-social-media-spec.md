# Spec — médias sociaux (vidéo **ou** image)

**Décision produit** : une page du fil est **soit** une vidéo, **soit** une image. Pas les deux sur la même page. Pas de carrousel, pas de stories.

**État du contrat (21 oct. 2026)** : feed, social, search et video ne portent que des clips `ready` (`video_id`, `hls_url`, `thumbnail_url`). Il n’existe **aucun** type de post image. Like, commentaire, share et signalement sont scopés `/videos/{id}`.

Conséquence : l’UI cible est spécifiée ici. **Elle ne se code pas** tant que B-IMG-POST n’est pas levé. On n’envoie pas une photo vers `POST /videos/uploads` (refus `content_type_invalid`). On n’invente pas de cartes image à partir des miniatures ou des photos boutique.

Fichiers concernés (quand le contrat existera) : `UploadScreen.kt`, `UploadViewModel.kt`, `FeedScreen.kt`, `VideoActions.kt`, `VideoPlayer.kt`, `VideoScreen.kt`.

## 1. Consulter — une page, un média

### Règle d’affichage

| `kind` (cible) | Surface | Player | Image |
|---|---|---|---|
| `video` | Fil, item isolé, recherche | HLS (`VideoSurface`) + miniature jusqu’à la 1re frame | Miniature seulement |
| `image` | Idem | **Aucun** | Plein cadre, `ContentScale.Crop` (même crop ZOOM que la vidéo) |
| Absent / inconnu | — | Traiter comme aujourd’hui (vidéo) | Ne pas inventer un `kind` |

Aujourd’hui tous les items sont implicitement `video`. Le client ne branche sur l’image que si le DTO le dit **explicitement** (champ stable à définir côté OpenAPI).

Overlay inchangé (scrims, like, commentaires, partage texte, ⋮ signaler, auteur). **Aucun overlay produit.** Tap page :

- vidéo : play / pause (déjà) ;
- image : pas de play ; pas de double-tap like (déjà interdit).

### Espaces

| Surface | Aujourd’hui | Cible |
|---|---|---|
| Pour toi / Abonnements / Tendances | Vidéos `ready` | Items `video` **ou** `image` |
| Item isolé | `GET /videos/{id}` | Route générique ou deux GET selon `kind` |
| Recherche | `type=videos\|users\|hashtags` | + résultats image **si** le search les indexe |
| Profil public | Pas de grille | Toujours pas (API grille absente) |
| « Mes vidéos » | Liste proprio | Renommer / étendre en « Mes publications » **quand** l’API liste les images |

### États

| État | Vidéo | Image |
|---|---|---|
| Prêt + approuvé | Overlay + HLS | Overlay + photo |
| Traitement | Texte d’attente | Texte d’attente (si le pipeline image est async) |
| Échec traitement | Codes vidéo existants | Codes image du **futur** contrat, pas ceux de la boutique recyclés à l’aveugle |
| Retiré / 404 | Copy existante | Même famille de copy |
| Réseau / décodage | Retry player (P1) | Placeholder + Réessayer Coil |
| URLs absentes | Pas de like/share média | Idem |

## 2. Importer — choisir le type, puis le fichier

### Cible (après B-IMG-POST)

```
Onglet Publier
  → « Choisir une vidéo ou une photo » (Photo Picker ImageAndVideo)
  → Branche selon MIME
        vidéo → règles ADR-0007 (MP4/MOV/WebM, 256 Mo, 3 min)
        image → règles du **contrat social image** (pas inventées ici)
  → Aperçu local (frame / photo) + titre + description
  → Publier → envoi du pipeline correspondant
  → %, succès, échec, un seul envoi à la fois
```

Deux CTA séparés (« Choisir une vidéo » / « Choisir une photo ») sont acceptables si plus clairs que `ImageAndVideo`. Même branche de validation.

Caméra in-app : non (Q-MED-05).

### Aujourd’hui (contrat vidéo seul)

Conserver **`VideoOnly`**. Copies actuelles (« Publier une vidéo », `upload_rules`).

**Interdit tant que B-IMG-POST tient**

- `ImageAndVideo` ou bouton photo qui poste sur `/videos/uploads` ;
- prévisualiser une image comme si elle allait être publiée ;
- réutiliser l’upload logo / produit / avatar pour remplir le fil.

### Règles d’envoi (vidéo, déjà vraies)

- Refuser avant `POST /uploads`.
- Un seul envoi (`isBusy`).
- « Gardez l’application ouverte… »
- Annuler / réessayer / 429 : inchangés.
- Description : aligner sur **500 runes** (limite serveur).

### Règles d’envoi (image, **quand le contrat existe**)

Ne pas recopier 5 Mo / jpeg/png/webp ici comme s’ils étaient sociaux. Ces chiffres sont ceux de **shop et avatar**. Un post social peut avoir d’autres plafonds.

Dès qu’OpenAPI publie `content_type`, `max_bytes` et les codes d’erreur : les afficher dans `upload_rules` et les contrôler localement, comme pour la vidéo.

## 3. Contrat backend manquant (B-IMG-POST)

Pour afficher et publier une image sociale, il faut **au minimum** (proposition de travail, **non réalisée**) :

| Besoin UI | Pourquoi c’est absent |
|---|---|
| Discriminant `kind` : `video` \| `image` sur l’item feed | `Item` n’a que `video_id` + `hls_url` |
| URL publique de l’image de post | Pas de champ |
| Upload présigné image **sociale** (init / confirm / statuts) | Seuls video / shop / avatar existent |
| Like, commentaire, share, stats sur cet item | Routes `/videos/{videoId}/…` seulement |
| Signalement | `target_type` enum `video \| comment \| user` — pas `image` / `post` |
| Search | `type=videos` seulement pour le média |
| Modération retrait | Pipeline `poro.video.*` |

Sans ces routes, le fil **ne peut pas** montrer un post photo réel. Les avatars et `thumbnail_url` ne comptent pas.

**Ne pas** inventer les formats, la taille max, le recadrage ni le bucket. Les écrire dans OpenAPI + ADR, puis seulement brancher Android (lot M5).

Réutiliser le pipeline shop (5 Mo, WebP) est une **option d’implémentation serveur**, pas une limite à afficher tant qu’elle n’est pas dans le contrat social.

## 4. Enregistrer

Voir [06](06-download-ux-spec.md).

- Vidéo : non exposé (B-VID-SAVE).
- Image de post **publique**, une fois le contrat là : même machine d’états que Q-MED-01 (décision juridique partagée).
- Miniature / avatar : pas d’entrée « Enregistrer la miniature ».

⋮ = Signaler seulement, jusqu’à décision contraire.

## 5. Copies

| Situation | Titre / CTA |
|---|---|
| Aujourd’hui | `upload_title` = Publier une vidéo |
| Cible mixte | Publier ; « Choisir une vidéo ou une photo » |
| Envoi vidéo | Conserver `upload_sending` / `upload_ready` |
| Envoi image (futur) | Envoi de « %1$s » : %2$d %% — **pas** « Téléchargement » |
| Fil image, décodage KO | Impossible d’afficher cette photo. Réessayer. |

Ne pas renommer l’onglet en « Télécharger ».

## 6. Accessibilité

- Page vidéo : titre + auteur + play/pause (déjà).
- Page image : « Photo de {auteur}. {titre} ». Pas d’annonce play.
- Picker cible : « Choisir une vidéo ou une photo depuis la galerie ».
- Actions like/comment/share : inchangées, compteurs inclus.

## 7. Non-régression

Tant que seul le contrat vidéo existe : fil, like, commentaire, follow, share texte, recherche, signalement, Publier vidéo **inchangés**. Aucune carte image fictive.
