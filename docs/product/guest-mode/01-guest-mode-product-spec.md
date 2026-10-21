# Mode visiteur — définition produit

- **Date** : 21 octobre 2026.
- **Nature** : conception. **Aucun fichier de `apps/mobile-android`, des services ou des OpenAPI n’est modifié.**
- **Pas de commit, branche, push.**
- **Base** : OpenAPI, handlers, Kotlin actuel ; [phase-2/03](../mobile-ux-phase-2/03-guest-mode-feasibility.md) ; [mobile-media](../mobile-media/01-scope-and-decisions.md).
- **Référence UX** : fluidité type TikTok (fil vertical, compte seulement à l’action). Pas une copie des écrans TikTok.

## 1. Promesse

Ouvrir Poro = **voir du contenu public tout de suite**. Créer un compte n’est demandé que lorsqu’une action a besoin d’une identité.

Regarder n’est pas un compte. Aimer, commenter, suivre, publier, commander, signaler, ouvrir sa boîte de notifications : oui.

Cette promesse **ne contourne pas** le serveur. Une route Bearer reste Bearer. Un contenu `removed` ou non `ready` reste 404 pour un visiteur.

## 2. Verdict de faisabilité (résumé)

| Objectif | Faisable maintenant ? |
|---|---|
| Lire HLS / miniature d’une vidéo `ready`+`approved` | Oui (API) |
| Profil public, commentaires GET, stats, search, boutique, produit | Oui (API) |
| Démarrer l’app sur le fil **sans** login | **Non** — Android force `PhoneRoute` + éjecte tout écran hors auth/légal |
| Remplir l’onglet Accueil (Pour toi / Abonnements / Tendances) | **Non** — les trois `GET /feed/*` exigent un JWT |
| Posts **image** dans le fil | **Non** — B-IMG-POST ([mobile-media/04](../mobile-media/04-social-media-spec.md)) |
| Favoris, FCM push, recherche catalogue | Absents — ne pas les inventer |

**Livrer le mode visiteur « fil d’abord » est bloqué par B-FEED** (trending public + quota IP + ADR) **et** par le mur Android (B-AND-START). Une app qui ouvre Accueil sans ces deux levées affiche une erreur réseau. Interdit.

Détail : [02](02-public-api-audit.md), [09](09-backend-dependencies.md).

## 3. Décisions produit

| # | Décision |
|---|---|
| 1 | Cinq onglets **conservés** : Accueil, Rechercher, Publier, Boutique, Profil. Pas de second `NavHost`. |
| 2 | Accueil visiteur = **Tendances seulement**. Pour toi et Abonnements : visible, tap → invite de connexion (pas un 401 brut). |
| 3 | Les boutons sociaux **restent visibles**. Une action protégée ouvre un prompt de connexion, elle ne disparaît pas. |
| 4 | Après login : retour à l’écran d’origine (`returnTo`). Logout / session morte : pile vidée vers login (comportement actuel de `goToSignIn`). |
| 5 | Replay automatique **seulement** pour les écritures idempotentes sans argent (like, follow). Jamais checkout, publication, paiement. |
| 6 | Marketplace : lecture publique images ; panier / commande → login. Pas de recherche catalogue (PORO-026). |
| 7 | Notifications **personnelles** → login. Compteurs publics (likes, commentaires) visibles. Pas de FCM visiteur. Pas de badge unread fake. |
| 8 | Légal déjà consultable déconnecté : conserver. |
| 9 | Staff / admin : jamais dans l’app grand public. |
| 10 | PoroTheme, Compose natif, pas de WebView, pas de nouvelle dépendance UI. |

## 4. Ce qu’un visiteur **voit** (cible, après B-FEED)

- Fil Tendances, défilement vertical, une lecture à la fois.
- Vidéos publiques ; images de post **si** le contrat social image existe (sinon clips seuls).
- Overlay : auteur, titre, hashtags, like (compteur), commentaires (liste GET), partage, ⋮ signaler.
- Recherche vidéos / comptes / hashtags.
- Vitrine `@handle` et fiche produit (images).
- Onglet Profil : invitation à se connecter + légal. Pas de `GET /users/me`.
- Paramètres non personnalisés accessibles depuis cet onglet (à propos, légal). Pas la liste des sessions.

## 5. Ce qu’un visiteur **ne voit pas** / n’obtient pas

- Pour toi, Abonnements (données).
- Panier, commandes, adresses, `GET /shops/me`.
- Boîte de notifications, préférences push, appareils.
- Upload, mes vidéos, édition de profil.
- Téléphone, email, pays d’un tiers ; commandes ; file modération.
- Favoris (API absente) : **pas de bouton**.
- Notification push (Firebase absent).

## 6. Vocabulaire

| Terme | Sens |
|---|---|
| Visiteur | Aucun jeton. Aucun header `Authorization`. |
| Membre | Session Bearer valide. |
| Prompt de connexion | Feuille ou écran qui explique **pourquoi**, puis le parcours auth existant. |
| `returnTo` | Route + args à restaurer après succès. Pas un replay d’achat. |

Ne pas inventer un rôle JWT `GUEST`.

## 7. Livrables de ce dossier

| Fichier | Contenu |
|---|---|
| [02-public-api-audit.md](02-public-api-audit.md) | Routes publiques vs protégées |
| [03-guest-permissions-matrix.md](03-guest-permissions-matrix.md) | Chaque bouton |
| [04-guest-user-journeys.md](04-guest-user-journeys.md) | Parcours |
| [05-guest-ui-specifications.md](05-guest-ui-specifications.md) | Écrans |
| [06-authentication-prompts.md](06-authentication-prompts.md) | Clic protégé |
| [07-security-review.md](07-security-review.md) | Risques |
| [08-acceptance-tests.md](08-acceptance-tests.md) | AC-GST-* |
| [09-backend-dependencies.md](09-backend-dependencies.md) | B-FEED et le reste |
| [10-implementation-roadmap.md](10-implementation-roadmap.md) | Lots G0–G4 |

Rien n’est **implémenté**. Documentation seulement.
