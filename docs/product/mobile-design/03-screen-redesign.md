# Poro — Conception écran par écran

Spécifications visuelles des destinations **réelles** (`Routes.kt`) et des feuilles embarquées. Les écrans sans API (chat, Wave, catalogue global, grille publique) ne sont **pas** conçus.

Format unique :

- Objectif · Structure visuelle · Composants · Actions · États · Accès · API · Petit écran · AC

Priorité visuelle : ce qui change par rapport à l’existant est marqué **Δ**.

---

## A. Session et authentification

### UI-AUTH-000 — Démarrage (`LoadingScreen`)

| | |
|---|---|
| Objectif | Attendre `SessionState` (jetons DataStore) |
| Structure | Fond `background`, spinner centré, aucun logo animé |
| Composants | `CircularProgressIndicator` « Chargement… » |
| Actions | Aucune |
| États | Unique |
| Accès | Tous |
| API | Aucun appel écran (lecture locale) |
| Petit écran | Centré, pas de décalage |
| AC | Pas de flash Phone puis Home ; pas de flash blanc sur thème sombre |

### UI-AUTH-001 — Téléphone (`PhoneEntryScreen`)

| | |
|---|---|
| Objectif | Entrer avec un numéro des 4 pays |
| Structure | **Δ** Haut : mot-symbole Poro (texte `Poro` `headlineMedium` + point orange, pas une illustration). Sous-titre. Carte surface : picker pays + champ. CTA pleine largeur. Lien email. **Δ** Pied : liens légaux `TextButton` 48 dp |
| Composants | `AuthForm`, `SubmitButton`, picker pays, `ErrorMessageText` |
| Actions | Recevoir le code → OTP ; email ; ouvrir légal |
| États | idle, invalid hint, sending, retry countdown, error |
| Accès | Déconnecté |
| API | `POST /api/v1/auth/otp/request` |
| Petit écran | Scroll + `adjustResize` ; CTA reste au-dessus du clavier |
| AC | +33 refusé ; CI valide envoie ; un seul CTA orange |

**Δ visuel** : sortir du formulaire « générique M3 ». Le picker pays (drapeau + indicatif) est le premier élément tactile, pas un détail.

### UI-AUTH-002 — OTP (`OtpVerifyScreen`)

| | |
|---|---|
| Objectif | Valider le SMS 6 chiffres |
| Structure | Titre, numéro masqué partiel, `CodeField` centré, timer, Valider, Renvoyer, Modifier le numéro |
| Composants | `CodeField` **Δ** cellules 48×56, `SubmitButton` |
| Actions | verify, resend, back |
| États | verifying, expired, resent (vert secondary), error, countdown |
| Accès | Déconnecté (phone en argument) |
| API | `POST /auth/otp/verify`, `POST /auth/otp/request` |
| Petit écran | Cases en une rangée ; si largeur < 360 dp, gap 4 dp |
| AC | Autofill SMS ; succès → Home pile effacée ; `deletion_cancelled` snackbar |

### UI-AUTH-003 — Email login

`AuthForm` : email, mot de passe, Se connecter, mot de passe oublié, créer un compte, retour téléphone. **Δ** mêmes liens légaux.

API : `POST /auth/email/login`.

### UI-AUTH-004 — Inscription email

Nom, email, mot de passe. **Δ** Hint honnête : le nom peut ne pas apparaître sur le profil (lacune `full_name` auth) — ne pas promettre « ce nom sera public » tant que le backend l’ignore.

API : `POST /auth/email/register` → `EmailVerifyRoute`.

### UI-AUTH-005 — Vérification email

Authentifié. Code + renvoyer + **Plus tard**. Ne pas culpabiliser le skip.

API : `POST /email/verification/request`, `.../confirm`.

### UI-AUTH-006 / 007 — Forgot / Reset

Toujours 202 sur forgot (pas d’énumération d’emails). Reset : code + nouveau mot de passe → login.

---

## B. Accueil et vidéo

### UI-FEED-001 — Fil (`HomeRoute`)

Détail : [05-social-feed-design.md](05-social-feed-design.md).

| | |
|---|---|
| Objectif | Consommer Pour toi / Abonnements / Tendances |
| Structure | Noir plein écran. **Δ** scrim haut 40 % sous onglets + cloche. Pager vertical. Overlay bas : auteur (avatar 48) + titre + hashtags + colonne actions (like, comment, share, **Δ** ⋮). Nav bar reste |
| Composants | `VideoPage`, `VideoSurface`, `Avatar`, `VideoActions`, `OverflowMenu`, `FeedNotificationBell` |
| Actions | kind, play/pause, auteur, like, comments, share, **Δ** signaler |
| États | loading, empty (2 copies), error+retry, playing, paused |
| Accès | Connecté. Invité : bloqué (trending 401) — [phase-2/03](../mobile-ux-phase-2/03-guest-mode-feasibility.md) |
| API | `GET /feed/for-you\|following\|trending` ; likes/stats |
| Petit écran | Actions au-dessus de la nav ; titre 2 lignes ; ne pas recouvrir le like |
| AC | Contraste overlay sur vidéo claire ; pas de 6e action inventée |

### UI-FEED-002 — Vidéo isolée (`VideoRoute`)

Même `VideoPage` + `BackTopBar` overlay blanc. États processing / failed / removed / 404.

API : `GET /videos/{id}`.

**Δ P1** : mute, échec HLS distinct, spinner média — pas un nouvel écran.

### UI-SOC-002 — Commentaires (sheet)

| | |
|---|---|
| Objectif | Lire / écrire commentaires 1 niveau |
| Structure | Sheet 85 %, titre, liste, composer ancré bas + IME |
| Composants | `ModalBottomSheet`, `Avatar`, like, **Δ** ⋮ Signaler (pas soi) |
| Actions | post, like, reply, delete, load more, report |
| États | empty, paging, error, reply banner, posting |
| Accès | Connecté |
| API | `GET/POST /videos/{id}/comments`, replies, like, `DELETE /comments/{id}` |
| Petit écran | Composer toujours visible au-dessus du clavier |
| AC | Auteur = « Vous » ou **Δ** nom si l’API l’expose ; sinon rester « Membre Poro » (ne pas inventer un nom) |

**Δ** : séparateurs légers, avatar 40, zone like 48. Pas d’édition (API 15 min non branchée — P1 hors ce redesign visuel).

### UI-SOC-003 — Partage (sheet)

WhatsApp / Autres. Ne pas afficher une URL si `SHARE_BASE_URL` vide — copie titre seulement, déjà le cas.

API : `POST /videos/{id}/shares`.

---

## C. Recherche et publication

### UI-SRC-001 — Recherche (`SearchRoute`)

| | |
|---|---|
| Objectif | Trouver vidéos, comptes, hashtags |
| Structure | **Δ** Champ search en haut (surface, radius 12, icône Search). Suggestions. `TabRow` Vidéos / Comptes / Hashtags. Listes |
| Composants | `OutlinedTextField`, `TabRow` (**Δ** aujourd’hui à confirmer vs TextButton), `Avatar`, miniature 9:16 |
| Actions | submit, open video/user, hashtag → nouvelle query |
| États | hint, suggesting, results, empty, error, paging |
| Accès | Connecté (API pourtant publique) |
| API | `GET /search`, `GET /search/suggest` |
| Petit écran | Tabs scrollables si police large |
| AC | Pas d’onglet Produits ; `is_creator` non affiché ou chip non cliquable |

### UI-UPL-001 — Publier (`UploadRoute`)

| | |
|---|---|
| Objectif | Envoyer une vidéo (picker, pas caméra) |
| Structure | Titre écran. **Δ** Zone drop/picker 9:16 surface-variant (pas une Card générique). Titre, description. CTA Publier. Carte statut (idle / % / processing / ready / failed) |
| Composants | `SubmitButton`, `Card` statut, `ErrorMessageText` |
| Actions | pick, publish, cancel/abort, voir la vidéo |
| États | Idle, file check, Uploading %, Processing, Finished, Failed |
| Accès | Connecté (tout membre, CREATOR non requis — ADR-0007) |
| API | `POST /videos/uploads`, PUT S3, complete, abort, poll |
| Petit écran | Scroll ; CTA visible ; consigne « Gardez l’application ouverte » persistante pendant upload |
| AC | 256 Mo / 3 min / titre 1–100 ; pas de permission CAMERA |

---

## D. Profils, paramètres, notifications

### UI-SOC-001 — Profil public

| | |
|---|---|
| Objectif | Voir quelqu’un et s’abonner |
| Structure | `BackTopBar` + **Δ** ⋮ Signaler. Avatar 96, nom, @, badge Créateur **texte**, bio, `FollowCounts`, CTA S’abonner. **Pas de grille** (API owner-only) |
| Composants | `Avatar`, `FollowCounts`, `Button` follow, `OverflowMenu` |
| Actions | follow/unfollow, back, report, **Δ P1** ouvrir listes follow (API prête) |
| États | loading, error, followBusy, isMe (masquer follow, masquer report self) |
| Accès | Connecté |
| API | `GET /users/by-username/{u}` ou id, `GET /users/{id}/stats`, `PUT/DELETE /follow` |
| Petit écran | CTA sticky bas optionnel seulement si bio très longue — sinon scroll simple |
| AC | Pas de shop, pas de videos inventées ; badge non cliquable |

### UI-PRF-001 — Mon profil

| | |
|---|---|
| Objectif | Identité, mes vidéos, accès édition |
| Structure | **Δ** Header centré plus serré (gap 8). Badge Créateur non cliquable (`SuggestionChip` / texte). Bannière email = `secondaryContainer` **vert Poro**. Compteurs. Grille mes vidéos 3 col 9:16. Rangée Modifier / Paramètres. Devenir créateur en secondaire si pas encore |
| Composants | `Avatar` 96, `MyVideosSection`, `EmptyState` si 0 vidéo |
| Actions | edit, settings, verify, become creator, open/delete video |
| États | Loadable, becomingCreator, empty grid |
| Accès | Moi |
| API | `GET /users/me`, `GET /auth/me`, `POST /users/me/creator`, `GET /videos` |
| Petit écran | Grille 3 col min 96 dp ; pas 4 col |
| AC | Chip morte corrigée ; hint username avant créateur |

**Δ copie** : « Devenir créateur » = badge public, **pas** un droit de publication (déjà possible). Q-04.

### UI-PRF-002 — Éditer

Avatar picker, username (dispo live), display_name **max 50**, bio **max 160** (**Δ** aligner `AccountRules` sur le serveur).

API : `PATCH /users/me`, availability, avatar upload/delete.

### UI-PRF-003 — Paramètres

| | |
|---|---|
| Objectif | Sessions, notifications, légal, sortie |
| Structure | **Δ** Groupes : Appareils · Notifications · **Δ** À propos (CGU, confidentialité, mentions, version `versionName`) · Compte (logout, supprimer) |
| Composants | `Loadable` sessions, `ListItem`, `AlertDialog` revoke |
| Actions | revoke, revoke others, prefs, legal, delete, logout |
| États | sessions loading/error, revoking |
| Accès | Moi |
| API | `GET/DELETE /auth/sessions`, `POST /sessions/revoke-others`, `POST /auth/logout` |
| Petit écran | ListItems 56–72 dp |
| AC | Pas de toggle thème ; pas de Wave ; légal tappable même indisponible |

### UI-PRF-004 — Supprimer le compte

Conserver la gravité : explication J+30, password **ou** OTP, checkbox, bouton error. Pas d’illustration.

API : `GET /auth/me`, `POST /otp/request`, `POST /auth/me/delete`.

### UI-NTF-001 — Notifications

Liste `ListItem` + avatar. Unread = `secondaryContainer` vert, plus lavande. Tap → cible (vidéo, profil, commande).

API : `GET /notifications`, `PATCH .../read`, `POST .../read-all`.

### UI-NTF-002 — Préférences

Switches. **Δ** Label push : tant que Firebase absent, hint « Bientôt sur cet appareil » ou switch OS masqué (Q-21). `orders` hint obligatoire.

API : `GET/PATCH /notifications/preferences`.

---

## E. Marketplace

Détail : [06-marketplace-design.md](06-marketplace-design.md).

### UI-SHP-001 — Accueil boutique **Δ majeur**

Aujourd’hui : pile de `OutlinedButton`. Cible : un **hub honnête**, pas un faux catalogue.

```
[ TopBar « Boutique »          🛒 badge ]
[ Carte « Trouver une boutique »         ]
  hint + champ @handle + CTA orange
[ Rangée 2 tuiles : Mes commandes | Vendre ]
```

Pas de carrousel, pas de catégories, pas de « populaires ».

API : `GET /shops/{handle}`, `GET /cart`, `GET /shops/me`.

`hasShop == null` (réseau) : tuile Vendre = « Ma boutique » neutre, pas « Ouvrir » par défaut.

### UI-SHP-002 — Vitrine

Header logo + nom + @handle + description. Grille 2 col `ProductCard` clipée. Empty `shop_no_products`.

API : `GET /shops/{handle}`, `GET /shops/{handle}/products`. Closed/suspended → 404 « introuvable » (conserver).

### UI-SHP-003 — Fiche produit

Galerie 1:1, **prix `titleLarge`**, titre, vendeur (lien vitrine), variantes chips, stepper, CTA Ajouter. Rupture : CTA disabled + texte.

API : `GET /products/{id}`, `PUT /cart/items/{variantId}` (**somme**, pas replace — correctif phase 2).

### UI-SHP-004 — Panier

Groupes par boutique, thumbs clipées, stepper, notice COD, CTA Commander si `canCheckout`.

API : `GET /cart`, `PUT/DELETE /cart/items/{id}`.

### UI-SHP-005 — Checkout

Deux étapes visuellement distinctes (titres différents). Étape 2 : total fort, label **Paiement à la livraison**, pas de picker. CTA Confirmer.

API : prefill, addresses, `POST /checkout/preview`, `POST /checkout/confirm`.

### UI-SHP-006 / 007 — Commandes

Liste : `StatusChip` + date + total. Détail : lignes, paiement COD, livraison, actions selon flags serveur (`cancellable`, ship, receipt).

API : `GET /orders` ou `/seller/orders` ; cancel / ship / confirm-receipt.

### UI-SHP-008 / 009 — Ma boutique / éditeur

Formulaire ouverture (pays définitif) **ou** dashboard. Produits + nouveau. Éditeur : photos max **10**, publish exige photo.

API : `POST/GET/PATCH /shops`, close/reopen, logo, CRUD products, stock.

---

## F. Nouveaux écrans (API ou obligation)

### UI-MOD-001 — Signaler (sheet)

Voir [phase-2/04](../mobile-ux-phase-2/04-reporting-ux-spec.md). 7 motifs radio, détail 500, CTA. Pas de route.

API : `POST /moderation/reports`.

### UI-LEG-001 / 002 — Légal

Liste depuis Settings + pied auth. Document : corps **ou** « Document non encore validé. » Version/date seulement si fournies.

Pas d’API. Accessible déconnecté.

### UI-SOC-004 — Listes follow (P1)

`FollowListRoute(userId, kind)` si on branche `GET /users/{id}/followers|following`. Liste Avatar + nom + @. Pas dans le lot visuel P0.

---

## G. Écrans volontairement non conçus

| Tentation | Raison |
|---|---|
| Messages | Pas de service chat |
| Analytics créateur | Pas de service |
| Wave / MoMo | Pas de `services/payment` |
| Recherche produits | PORO-026 |
| Grille profil public | `GET /videos` owner-only |
| Admin / file modération | App grand public interdite |
| Caméra in-app | Picker suffit, permissions absentes |
| Guest home | Bloqué tant que trending 401 |

---

## H. Synthèse des Δ visuels prioritaires

| Priorité | Écran | Δ |
|---|---|---|
| P0 | Theme global | Containers Poro, plus de lavande |
| P0 | Fil | Scrim haut + bas 72 % + menu ⋮ |
| P0 | Shop home | Hub 1 carte + 2 tuiles |
| P0 | Profil | Chip morte, banner vert |
| P0 | Report + Legal | Nouveaux, specs phase 2 |
| P0 | Checkout / orders | Hiérarchie prix / statuts, copies COD |
| P1 | Overlay | Mute, avatar 48, play avec ombre |
| P1 | Cart / product | Images clipées, add-to-cart somme |
| P1 | Settings | Groupe À propos + version |
| P1 | Commentaires | ⋮ report, rythme vertical |
| P2 | Empty illustré | Seulement si assets légers |
