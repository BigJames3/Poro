# Rapport de couverture

## 1. Méthode

Deux unités, **jamais mélangées** :

1. **Endpoint** = une méthode HTTP produit dans `docs/api/*.yaml`, hors `/health*`, `/metrics`, JWKS. Vérifiée dans le handler.
2. **Capacité** = `CAP-*` de [01](01-backend-capability-inventory.md) (1 capacité = 1 à 3 endpoints).

Un parcours (ex. publier) utilise plusieurs endpoints. Un GET stats sert like **et** overlay. D’où deux comptes.

Date du décompte : 21 octobre 2026. Lecture du code ; lots 1–3 Android **présents** dans l’arbre, non commités.

## 2. Endpoints produit

| Domaine | n |
|---|---|
| auth | 16 |
| user | 8 |
| video | 6 |
| feed | 3 |
| social | 18 |
| search | 2 |
| notification | 8 |
| moderation | 4 |
| shop | 22 |
| order | 20 |
| **Total** | **107** |

### Répartition (endpoints)

| Catégorie | n | % | Exemples |
|---|---|---|---|
| UI adaptée (action ou écran membre) | 72 | 67 % | OTP, feed, like, PDP, confirm COD, report |
| Partiel | 14 | 13 % | adresses, share sans URL, prefs push, limites texte, guest, aperçu photo |
| API seule (prête, pas d’UI) | 11 | 10 % | PATCH comment, followers, likes lists, devices, DELETE logo/cart |
| Staff / hors app grand public | 5 | 5 % | cases*, admin shop |
| Interne exposé HTTP mais hors produit | 0 dans les 107 | — | health exclus |

*Partiel* = utilisé mais incomplet (carnet, honnêteté, ou mur visiteur).

Hors 107 : HLS objet, worker, outbox, erasure.

## 3. Capacités métier (`CAP-*`)

| Catégorie | Lecture |
|---|---|
| Représentées correctement pour un **membre** | Auth, profil, upload vidéo, lecture HLS, 3 fils, like/comment/follow/share, search, inbox, boutique COD, report |
| Sans UI ou UI à compléter | Follow lists, edit comment, devices, DELETE logo/cart, carnet, mute |
| UI / promesse sans backend | Fil visiteur, post image, save vidéo, FCM, catalogue, favoris, chat, Wave, grille publique, TOS opposables |
| Bloquées dépendance | B-FEED, B-IMG-POST, B-VID-SAVE, Q-21, PORO-015/026 |
| Internes sans UI directe | Worker, outbox, storage, JWKS, metrics |
| Staff | File + suspend — documentées, pas dans l’APK |

Chiffre unique « 80 % du produit est dans l’app » : **refusé** (mélange endpoints / parcours / guest).

## 4. Couverture membre vs visiteur

| Parcours principal membre | Couverture UI |
|---|---|
| Se connecter, fil, like, commenter, publier, chercher, notifier, acheter COD, vendre | **Oui** (écarts P0/P1 ci-dessus) |
| Découvrir sans compte | **Non** (API partielle, app wall) |

## 5. Recommandation

P0 = honnêteté (push, limites, COD déjà en cours) + décision B-FEED.  
P1 = API déjà payées (listes, comment edit, guest après trending).  
P2 = nouveaux services.

Preuve détaillée : [02](02-endpoint-ui-traceability.md), [07](07-ui-gaps-and-backend-gaps.md).
