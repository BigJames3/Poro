# Axe B — Faisabilité du mode invité

Conclusion : **le mode invité tel que demandé (consulter le fil public sans compte) est BLOQUÉ.**
Les trois routes du fil exigent un jeton, dans le contrat comme dans le code. Il faut une
décision et une modification backend avant toute implémentation mobile (voir § 5 et `10`).

Un **mode invité partiel** (vidéo ouverte par lien, profils, vitrines, recherche) est techniquement
possible avec les routes actuelles, mais il ne répond pas à l'objectif « consulter le fil » et
n'est pas recommandé seul (§ 4).

## 1. Routes réellement publiques (contrats `origin/main`, vérifiées dans le code)

| Domaine | Route | Accès | Vérification code |
|---|---|---|---|
| Feed | `GET /api/v1/feed/for-you`, `/following`, `/trending` | **Jeton obligatoire** | `services/feed/internal/routes/routes.go:31` : `jwtauth.Middleware` sur tout le groupe |
| Vidéo | `GET /api/v1/videos/{id}` | Jeton facultatif | `services/video/internal/routes/routes.go:36` (`opt`) ; « Ready videos are public », autres statuts réservés au propriétaire |
| Vidéo | `GET /api/v1/videos` (ses vidéos), envois, suppression | Jeton obligatoire | |
| Médias HLS | `poro-videos` (bucket public) | Lecture anonyme | ADR-0015, `infra/storage/init.sh` : `Read` anonyme sur `poro-videos`, `poro-avatars`, `poro-shop` uniquement |
| Social | stats de vidéos (une ou plusieurs), likes d'une vidéo, commentaires et réponses, abonnés, abonnements, stats d'un compte | Jeton facultatif | `security: [{}, bearerAuth]` |
| Social | aimer, commenter, modifier, supprimer, suivre, partager | Jeton obligatoire | |
| User | `GET /api/v1/users?ids=`, `GET /api/v1/users/by-username/{u}` | Public (champs publics, sans pays) | `security: []` |
| Search | `GET /api/v1/search`, `/search/suggest` | Jeton facultatif ; limite 60/min **par compte, sinon par IP** | `services/search/internal/routes/routes.go:31`, `rate_limit.go:43-47` |
| Shop | `GET /shops/{handle}`, `/shops/{handle}/products`, `/products/{id}` | Public | |
| Order, notification, moderation | toutes les routes métier | Jeton obligatoire | |
| Gateway | seul `/api/v1/**` et `/health` exposés | — | ADR-0014 |

## 2. Matrice d'accès par profil

| Capacité | Visiteur | Utilisateur connecté | Propriétaire (contenu/boutique) | Modérateur / Admin |
|---|---|---|---|---|
| Fil Pour toi / Tendances | ❌ (401) | ✅ | ✅ | ✅ |
| Fil Abonnements | ❌ | ✅ | ✅ | ✅ |
| Vidéo `ready` par identifiant | ✅ | ✅ | ✅ | ✅ |
| Vidéo non prête ou retirée | ❌ | ❌ | ✅ (la sienne) | selon API modération |
| Compteurs, commentaires (lecture) | ✅ | ✅ (+ `liked_by_me`) | ✅ | ✅ |
| Aimer, commenter, suivre, partager (compté) | ❌ → connexion | ✅ | ✅ (ne peut pas se signaler) | ✅ |
| Profil public, abonnés | ✅ | ✅ | ✅ | ✅ |
| Recherche | ✅ (limite par IP) | ✅ (limite par compte) | ✅ | ✅ |
| Vitrine, fiche produit | ✅ | ✅ | ✅ | ✅ |
| Panier, commande | ❌ → connexion | ✅ | ❌ sur ses propres produits (`own_shop`) | ✅ |
| Signaler | ❌ → connexion | ✅ | ❌ soi-même (`cannot_report_self`) | ✅ |
| Publier une vidéo, ouvrir une boutique | ❌ → connexion | ✅ | ✅ | ✅ |
| File de modération, suspension de boutique | ❌ | ❌ | ❌ | ✅ (MODERATOR/ADMIN, hors app grand public) |

Le client n'accorde **aucun** droit : chaque ❌ ci-dessus est appliqué par le serveur. L'app ne
fait que masquer ou rediriger, pour l'ergonomie.

## 3. Risques identifiés

| Risque | Détail | Mitigation proposée |
|---|---|---|
| Fil fermé | 401 sur les 3 routes du feed | Décision backend B1 (§ 5) |
| Limite par IP | Les opérateurs mobiles africains utilisent massivement le CGNAT : des milliers de visiteurs partagent une IP et se bloquent mutuellement (search 60/min/IP) | Décision B2 : limite anonyme adaptée ou jeton d'appareil anonyme |
| Données personnelles | Routes publiques sans pays ; l'adresse de livraison n'est jamais publique | Aucune nouvelle exposition côté mobile ; revue sécurité de toute route rendue publique |
| Contenu retiré | Les vidéos retirées par la modération ne sont plus `ready` publiquement et quittent le bucket public (`video.openapi.yaml`) | Gérer 404 comme « Vidéo indisponible » |
| Session expirée vs invité | Aujourd'hui une session refusée renvoie à la connexion (`PoroNavHost`, `isSignInScreen`) | Distinguer « jamais connecté » (invité) de « session expirée » (message + connexion) |
| Abus | Lecture anonyme massive (scraping) | Limites gateway par IP déjà en place ; à dimensionner par l'équipe backend |

## 4. Expérience visiteur proposée (quand B1 sera levé)

```
Lancement sans session ──► Fil « Tendances » (lecture seule)
     │                         │
     │                         ├─ ♥ / 💬 envoyer / Suivre / Panier / Publier / Signaler
     │                         │        └─► Feuille « Connectez-vous pour continuer »
     │                         │                 [Se connecter] → PhoneRoute → retour à l'action
     │                         │                 [Plus tard]    → ferme la feuille
     │                         └─ Profil, vitrine, produit, recherche : lecture
     └─ Onglets visibles : Accueil, Rechercher, Boutique ; « Publier » et « Profil » ouvrent la feuille
```

Règles :
1. Une seule feuille de connexion réutilisable (`SignInPromptSheet`), avec le motif de l'action
   (« pour aimer cette vidéo »).
2. Après connexion, reprendre l'action demandée **sans l'exécuter automatiquement** quand elle est
   sensible (commande, signalement) : on revient sur l'écran, l'utilisateur confirme.
3. Une réponse 401 en mode invité ne doit jamais vider la pile ni afficher « session expirée ».
4. Aucune donnée mise en cache d'un compte précédent ne doit rester visible après déconnexion.

Mode partiel possible **sans** changement backend (non recommandé seul) : écran d'accueil invité
avec recherche, ouverture d'une vidéo, d'un profil ou d'une vitrine par lien. Sans fil, l'intérêt
pour un visiteur est faible et les liens d'application n'existent pas encore (`SHARE_BASE_URL`
vide).

## 5. Changements backend nécessaires (à décider, non réalisés)

| ID | Changement | Service | Questions à trancher |
|---|---|---|---|
| B1 | `GET /api/v1/feed/trending` (au minimum) en jeton facultatif, comme `search` (`jwtauth.OptionalMiddleware`) | feed | Quel fil pour un visiteur (Tendances seul ? Pour toi sans personnalisation ?) ; contrat OpenAPI et ADR-0009 à mettre à jour |
| B2 | Limite anonyme adaptée au CGNAT | feed, search, gateway | Seuil par IP, ou jeton d'appareil anonyme signé |
| B3 | Revue sécurité des routes rendues publiques | feed | Champs exposés, pagination (curseurs non devinables), contenu retiré |

Tant que B1 n'est pas livré et documenté dans `docs/api/feed.openapi.yaml`, le Lot 4 reste **bloqué**.
