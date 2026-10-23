# Dépendances backend

Aucun service n’est modifié dans cette phase. Cette page liste ce qui **doit** changer pour honorer « regarder sans compte », et ce qui est déjà suffisant.

## 1. Bloquant — B-FEED

Sans ceci, Accueil visiteur = 401.

| # | Changement | Où |
|---|---|---|
| 1 | `GET /api/v1/feed/trending` accepte l’absence de JWT | `services/feed` : `OptionalMiddleware` **sur trending seulement** (ou groupe séparé). For You / Following : `Middleware` inchangé |
| 2 | Handler trending : ne plus exiger `ClaimsFrom` | `Trending` n’utilise déjà pas le `userId` |
| 3 | Rate limit anonyme **par IP**, ~60/min (comme search) | `services/feed/internal/middleware` + Redis existant |
| 4 | Bearer invalide → 401 | Ne pas traiter un token pourri comme anonyme |
| 5 | OpenAPI `feed.openapi.yaml` : `security: [{}, {bearerAuth}]` sur trending | Contrat |
| 6 | Tests : 200 sans header ; For You 401 sans header ; token pourri 401 | `services/feed` + e2e gateway si la suite le couvre |
| 7 | ADR court, complément [0009](../../architecture/adr/0009-feed-v1.md) | Trending = surface publique, pas de PII ajoutée |

**Hors besoin** : Pour toi ou Following publics (incorrect : follows + session Redis).

Gateway : **rien**. Elle ne vérifie pas le JWT.

## 2. Déjà suffisant (ne pas « ouvrir » davantage)

| Domaine | Routes | Action |
|---|---|---|
| Vidéo | `GET /videos/{id}` Opt | Aucune |
| Social reads | comments, stats, likes list, follow lists Opt | Aucune |
| Search | Opt + quota IP | Aucune |
| User | `GET /users/by-username` Pub | Aucune |
| Shop | handle, products, product Pub | Aucune |
| Auth | OTP / email Pub | Aucune |
| HLS | bucket public GET | Aucune (ne pas exposer les sources) |

## 3. Volontairement fermé (prompt login)

Writes sociales, report, notif, cart, order, upload, me, shops/me. **Ne pas** les passer en Opt pour « simplifier l’app ».

## 4. Non requis pour le visiteur v1

| Sujet | ID | Note |
|---|---|---|
| Post image social | B-IMG-POST / Q-MED-06 | Cible fil mixte ; autre chantier |
| Grille profil | — | API absente |
| Search boutique | PORO-026 | Ne pas inventer |
| Favoris | — | Absent |
| Followers UI | API Opt déjà | Client seulement |
| FCM | Q-21 | Pas visiteur |
| `SHARE_BASE_URL` | Config mobile | Pas un endpoint ; sans elle le share visiteur n’a pas de lien |
| App Links | — | Nice-to-have après URL publique |
| Commentaires GET plus riches (username) | DTO social | UI « Membre Poro » déjà honnête |

## 5. Android (pas backend, mais bloquant UX)

Même après B-FEED :

- `startDestination` Phone si SignedOut
- `SignedOut` → `goToSignIn()` hors auth/légal

Lots G1+ **après** autorisation explicite. Sans B-FEED, G1 livrerait un Accueil en erreur : **ne pas** le faire.

## 6. Ordre de validation humaine

1. **Fait le 21 octobre 2026** : B-FEED accepté ([ADR-0019](../../architecture/adr/0019-feed-trending-public.md)).
2. **Fait le 21 octobre 2026** : backend trending Opt + quota IP 60/min + OpenAPI + tests (`services/feed`, ADR-0019).
3. Autoriser alors le Kotlin guest (G1) — **pas autorisé** le 21 octobre 2026. Vérifier d’abord trending 200 sans header sur la gateway.
4. Q-MED-06 / images sociales : toujours indépendant ; **pas autorisé**.

Le refus de B-FEED n’est plus le choix par défaut. Sans l’implémentation, l’app garde l’auth-wall.
