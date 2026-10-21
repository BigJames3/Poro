# Tests d’acceptation — mode visiteur

Légende : **Bloqué** = ne pas tester comme livré tant que B-FEED / G1 absents · **Cible** = après implémentation autorisée · **NR** = membre inchangé.

Aucun de ces AC n’est vert dans l’app actuelle (mur `PhoneRoute`).

## AC-GST — Visiteur

| ID | Critère | Dépendance |
|---|---|---|
| AC-GST-01 | Splash SignedOut → **Home Tendances**, pas Phone | B-FEED + G1 |
| AC-GST-02 | `GET /feed/trending` **sans** header → 200, pager rempli | B-FEED |
| AC-GST-03 | Aucun appel `/users/me`, `/cart`, `/notifications`, `/auth/me` au cold start visiteur | G1 |
| AC-GST-04 | Défilement + lecture/pause HLS d’une vidéo ready | G1 + API vidéo déjà Pub |
| AC-GST-05 | Tap Pour toi / Abonnements → prompt, **pas** `GET /feed/for-you` anonyme | G1 |
| AC-GST-06 | Tap auteur → profil public 200 | API déjà Pub |
| AC-GST-07 | Sheet commentaires : liste GET ; composer → prompt | Social GET déjà Opt |
| AC-GST-08 | Like visible + tap → prompt ; compteur inchangé si annulation | G1 |
| AC-GST-09 | Like après login : returnTo + PUT + cœur plein | G1 |
| AC-GST-10 | Share Intent si `SHARE_BASE_URL` non vide ; pas de `POST /shares` visiteur | G1 |
| AC-GST-11 | ⋮ Signaler → prompt ; pas de POST anonyme | G1 |
| AC-GST-12 | Cloche → prompt ; **aucun** badge unread | G1 |
| AC-GST-13 | Search vidéos/users/hashtags sans compte | Mur Android levé |
| AC-GST-14 | Boutique handle + PDP images ; Ajouter → prompt ; pas de PUT cart | G1 |
| AC-GST-15 | Onglet + → prompt publish ; pas d’upload | G1 |
| AC-GST-16 | Onglet Profil = invitation + légal ; 0 `GET /users/me` | G1 |
| AC-GST-17 | Plus tard / back depuis auth → clip / PDP toujours là | G1 |
| AC-GST-18 | 404 vidéo / shop : copy introuvable, pas login | G1 |
| AC-GST-19 | Réseau trending : Réessayer, pas login | G1 |
| AC-GST-20 | 429 search : throttle, pas login | déjà + G1 |
| AC-GST-21 | Favoris / FCM / catalogue / post image : **pas** de bouton opérationnel | — |
| AC-GST-22 | Staff : aucune route cases/admin | — |

## AC-AUTH — Continuité

| ID | Critère |
|---|---|
| AC-AUTH-01 | Login succès depuis like : écran d’origine, pas un `goHome` destructif |
| AC-AUTH-02 | Commentaire : texte conservé, envoi **manuel** après login |
| AC-AUTH-03 | Checkout jamais auto-confirm après login |
| AC-AUTH-04 | Upload : pas d’URI rejoué |
| AC-AUTH-05 | Session expirée : snackbar ; abandon → visiteur sur contenu public (après B-FEED) |
| AC-AUTH-06 | Logout Settings : visiteur Tendances (cible) ou Phone si B-FEED encore là |

## NR — Membre

| ID | Critère |
|---|---|
| NR-01 | Start SignedIn → Home Pour toi, trois fils |
| NR-02 | Like / comment / share / follow inchangés |
| NR-03 | Report 201/200/422/429 |
| NR-04 | Panier COD, pas de libellé Payée |
| NR-05 | Publier vidéo compatible |
| NR-06 | Refresh mutex ; replay refresh hors 30 s = révocation (déjà) |

## Session / 401

| ID | Critère |
|---|---|
| AC-401-01 | Visiteur : zéro `Authorization` |
| AC-401-02 | 401 anonyme : pas de refresh |
| AC-401-03 | Après B-FEED : trending + Bearer pourri → 401 (bug client si ça arrive) |
| AC-401-04 | For You sans JWT → 401 **serveur** (même après B-FEED) |

## Hors tests

- Fil image social (B-IMG-POST).
- Enregistrer une vidéo.
- Push FCM.
- Mode invité livré **sans** B-FEED.
