# Matrice — boutons et permissions visiteur

Chaque contrôle visible a **un** comportement. On ne grise pas « tout ce qui est social ».

Légende **Visiteur** : Pub = s’exécute sans compte ; Prompt = ouvre [06](06-authentication-prompts.md) ; Absent = pas de bouton (API ou feature manquante).

`returnTo` : restaurer l’écran. Replay : voir colonne.

## 1. Fil et vidéo

| Contrôle | API | Visiteur | Membre | Replay après login |
|---|---|---|---|---|
| Défiler le pager | — | Pub (si le fil a chargé) | Oui | — |
| Lecture / pause (tap) | HLS public | Pub | Oui | — |
| Onglet Tendances | `GET /feed/trending` | **Bloqué** jusqu’à B-FEED ; ensuite Pub | Oui | — |
| Onglet Pour toi | `GET /feed/for-you` | Prompt (fil personnel) | Oui | Recharger For You, pas un like |
| Onglet Abonnements | `GET /feed/following` | Prompt | Oui | Recharger Following |
| Tap auteur | `GET /users/by-username` | Pub | Oui | — |
| Like / unlike | `PUT/DELETE /videos/{id}/like` | Prompt | Optimistic + revert | **Oui** (idempotent) |
| Compteur likes | stats / item feed | Pub (chiffre) | + `liked_by_me` | — |
| Commentaires (ouvrir sheet) | `GET …/comments` | Pub (liste) | Oui | — |
| Composer / envoyer / répondre | `POST …/comments` | Prompt | Oui | **Non** (texte saisi : le garder dans le champ, envoyer après confirm du composer, pas en silence) |
| Like commentaire | `PUT …/comments/{id}/like` | Prompt | Oui | Oui |
| Share (Intent) | local + `SHARE_BASE_URL` | Pub **si** URL réelle, sinon titre seul | Oui | — |
| Share (compteur `POST /shares`) | Auth | Ne pas incrémenter en visiteur | Oui | Optionnel après login, une fois |
| ⋮ Signaler | `POST /moderation/reports` | Prompt | Oui | **Non** (rouvrir la feuille motifs) |
| Cloche | `GET /notifications` | Prompt (boîte perso) | Oui | — |
| Badge unread | `GET …/unread-count` | **Absent** (0 inventé interdit) | Oui | — |
| Mute / seek | — | P1 lecture, hors guest | idem | — |
| Enregistrer la vidéo | — | Absent (B-VID-SAVE) | Absent | — |
| Favoris | — | **Absent** (pas d’API) | Absent | — |
| Follow sur overlay | `PUT /users/{id}/follow` | Pas sur l’overlay actuel ; si ajouté : Prompt | Oui | Oui |

## 2. Recherche

| Contrôle | API | Visiteur | Membre |
|---|---|---|---|
| Query vidéos / users / hashtags | `GET /search` | Pub (60/min IP) | Quota compte |
| Suggest | `GET /search/suggest` | Pub | Oui |
| Ouvrir une vidéo | `GET /videos/{id}` | Pub si ready | Oui |
| Ouvrir un compte | profil public | Pub | Oui |
| Onglet produits | — | **Absent** (PORO-026) | Absent |

## 3. Publier

| Contrôle | Visiteur | Membre |
|---|---|---|
| Onglet Publier (icône +) | **Visible** → Prompt « Publiez votre première vidéo » | `UploadRoute` |
| Picker image sociale | Absent jusqu’à B-IMG-POST | idem |

Ne pas cacher l’onglet (cinq destinations).

## 4. Marketplace

| Contrôle | API | Visiteur | Replay |
|---|---|---|---|
| Trouver `@handle` | `GET /shops/{handle}` | Pub | — |
| Vitrine / grille | GET products | Pub | — |
| Fiche produit, galerie images | `GET /products/{id}` | Pub | — |
| Variante / stepper (local) | — | Pub (état local) | — |
| Ajouter au panier | `PUT /cart/items/{id}` | Prompt | **Non** (revoir le CTA, l’utilisateur confirme) |
| Icône panier | `GET /cart` | Prompt (ou badge 0 sans GET) | — |
| Mes commandes | `GET /orders` | Prompt | — |
| Ma boutique / ouvrir | `GET/POST /shops/me` | Prompt | Non |
| Checkout / confirm COD | Auth | Prompt | **Jamais** auto-confirm |
| Recherche catalogue | — | Absent | — |
| Lecteur vidéo produit | — | Interdit | — |

## 5. Profil et réglages

| Contrôle | Visiteur | Membre |
|---|---|---|
| Onglet Profil | Écran « Rejoindre Poro » + Connexion / Inscription + légal | `ProfileScreen` |
| Follow sur profil public | Prompt | Oui + replay idempotent |
| Compteurs followers (chiffres) | Pub via `GET /users/{id}/stats` si l’UI les montre | Oui |
| Listes followers | API Opt, **pas d’UI** — ne pas inventer | P1 existant |
| Éditer profil / avatar / mes vidéos | Prompt (depuis onglet Profil) | Oui |
| Sessions, logout, delete account | Invisibles | Oui |
| Légal / à propos | Pub (déjà) | Oui |
| Switch push | Inopérant sans FCM — ne pas le présenter comme actif | Labelliser (Q-21) |

## 6. Notifications (deux besoins)

| Besoin | Visiteur |
|---|---|
| Compteurs like/comment **sur le contenu** | Pub (données feed/stats) |
| Boîte de réception, cloche opérationnelle | Prompt — aucun item fictif |
| Push FCM | Non livré ; pas de bouton « activer les notifs » visiteur |

## 7. Règles transverses

1. Prompt, jamais un 401 brut dans un snackbar comme seul feedback d’un tap visiteur.
2. 403 staff / `forbidden` : ne pas offrir de contournement.
3. 404 contenu : copy « introuvable / retiré », pas login.
4. 429 : `Retry-After`, pas login.
5. Réseau : Réessayer, pas login.
6. Token optionnel : visiteur = **zéro** header. Un Bearer pourri sur une route Opt → 401 : le traiter comme session morte, pas comme « page publique KO ».
