# Expériences visiteur et membre

Détail normatif : [guest-mode](../guest-mode/01-guest-mode-product-spec.md). Ici : synthèse mapping.

## 1. État réel

| | Visiteur | Membre |
|---|---|---|
| Android start | `PhoneRoute` | `HomeRoute` Pour toi |
| GET publics (vidéo, search, shop, comments, profil) | API OK, **app refuse** | OK |
| `GET /feed/*` | 401 | OK |
| Writes sociales / panier / notif / upload | 401 | OK |

## 2. Boutons sociaux (un par un)

| Bouton | Visible visiteur (cible) | Tap visiteur | Membre |
|---|---|---|---|
| ♥ Like | Oui + compteur public | Prompt ; replay PUT | Optimistic |
| 💬 Commentaires | Oui + compteur | Sheet GET ; composer → prompt | Sheet + POST |
| ↗ Partage | Oui | Intent si URL ; pas de POST count | Intent + POST |
| ⋮ Signaler | Oui | Prompt ; rouvrir sheet | ReportSheet |
| Cloche | Oui, **sans badge** | Prompt inbox | Liste réelle |
| Suivre (profil) | Oui | Prompt ; replay PUT | Toggle |
| Publier (tab) | Oui | Prompt | Upload |
| Pour toi / Abonnements | Tabs visibles | Prompt (pas GET anonyme) | Load |

Favoris : **pas de bouton** (API absente).

## 3. Fil

Cible visiteur : Tendances seulement, même chrome [mobile-design/05](../mobile-design/05-social-feed-design.md). Images de post : seulement si B-IMG-POST levé. Une lecture à la fois.

Bloqué : B-FEED + `LaunchedEffect` SignedOut → `goToSignIn`.

## 4. Après connexion

`returnTo` ; replay like/follow seulement. Jamais confirm checkout / upload. `goHome()` destructif interdit sur un challenge. Logout cible : Tendances visiteur (après B-FEED).

## 5. Marketplace visiteur

Lecture handle / PDP images. Ajouter / panier / commandes / ma boutique → prompt. Pas de catalogue. COD honnête.

## 6. Membre — non-régression

Parcours actuels (auth, fil 3 kinds, upload, social, search, shop COD, report, légal) **préservés**. CREATOR n’est pas un gate publish.
