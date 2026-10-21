# Audit — routes publiques et restrictions

L’auth n’est **pas** dans la gateway Traefik (`infra/gateway/routes.yml` : préfixe `/api/v1/{domaine}`). Elle est dans **chaque service**. Rendre une route publique = changer ce service + OpenAPI + tests. Cette phase ne le fait pas.

Légende : **Pub** = pas de JWT ; **Opt** = JWT optionnel (`OptionalMiddleware` : pas de header = OK ; header invalide = 401) ; **Auth** = Bearer obligatoire ; **Staff** = `ADMIN`/`MODERATOR`.

Un bouton visible ≠ route publique.

## 1. Feed — tout Auth

| Route | Accès | Algo utilise le `sub` ? |
|---|---|---|
| `GET /api/v1/feed/for-you` | Auth | Oui (session Redis, follows) |
| `GET /api/v1/feed/following` | Auth | Oui |
| `GET /api/v1/feed/trending` | Auth | **Non** — JWT seulement pour passer le middleware et le quota « par user » |

Preuve : `services/feed/internal/routes/routes.go` groupe entier + `jwtauth.Middleware` ; `handler.go` `ClaimsFrom` sinon `unauthorized`.

OpenAPI `feed.openapi.yaml` : `security: bearerAuth` sur les trois. **B-FEED** : trending en Opt + quota IP. For You / Following restent Auth.

## 2. Vidéo

| Route | Accès | Visiteur |
|---|---|---|
| `GET /videos/{id}` | Opt | `ready`+`approved` : 200 + `hls_url` / `thumbnail_url`. Autres statuts / `removed` : 404 |
| HLS / miniature (objet `poro-videos`) | GET/HEAD anonyme | Si l’URL est connue (ADR-0015). Pas de listing. |
| Original `poro-video-sources` | Privé | Jamais d’URL |
| `GET /videos` (liste) | Auth | 401 |
| Upload / complete / abort / DELETE | Auth | 401 |

## 3. Social

Writes Auth. Reads Opt (`liked_by_me` / `followed_by_me` seulement si JWT).

| Route | Accès |
|---|---|
| `GET /videos/{id}/stats` | Opt |
| `GET /videos/{id}/comments`, `/comments/{id}/replies` | Opt |
| `GET /videos/{id}/likes`, `GET /users/{id}/likes` | Opt (liste : **pas d’UI** Android aujourd’hui) |
| `GET /users/{id}/stats`, `/followers`, `/following` | Opt (listes follow : **pas de client** Android) |
| `PUT/DELETE …/like`, `POST …/comments`, `PATCH/DELETE` comment | Auth |
| `PUT/DELETE …/follow`, `POST …/shares` | Auth |

Tout s’applique à une vidéo `ready` projetée. Pas de post image (B-IMG-POST).

## 4. Recherche

| Route | Accès | Types |
|---|---|---|
| `GET /search` | Opt (anonyme = quota IP 60/min) | `videos`, `users`, `hashtags` |
| `GET /search/suggest` | Opt | users ≤5, hashtags ≤5 |

**Pas** de `products` / `shops`. Ne pas inventer d’onglet catalogue.

## 5. Profils

| Route | Accès | Données |
|---|---|---|
| `GET /users/by-username/{username}` | Pub | username, display_name, bio, avatar. **Pas** email, téléphone, pays |
| `GET /users/me`, PATCH, avatar, creator | Auth | — |
| `GET /auth/me`, sessions, delete | Auth | — |

## 6. Marketplace

| Route | Accès |
|---|---|
| `GET /shops/{handle}` | Pub si boutique **active**. `closed` / `suspended` → 404 (pas d’énumération) |
| `GET /shops/{handle}/products` | Pub |
| `GET /products/{id}` | Pub si publié + boutique active |
| `GET/PATCH /shops/me`, upload logo, CRUD produits | Auth |
| Panier, checkout, adresses, commandes | Auth (`docs/api/order.openapi.yaml`) |
| `POST /admin/shops/{id}/suspend\|reinstate` | Staff — **hors app** |

Images seulement (`images[].url`). Pas de lecteur vidéo boutique.

## 7. Notifications et signalement

| Route | Accès | Note |
|---|---|---|
| `GET /notifications`, unread-count, mark read, preferences | Auth | Boîte **personnelle** |
| `PUT/DELETE /notifications/devices` | Auth | FCM : **pas de client** Android, pas de `google-services.json` |
| `POST /moderation/reports` | Auth | 10/h ; `target_type` video\|comment\|user |
| `GET /moderation/cases*` | Staff | Hors app |

## 8. Auth (entrée dans le compte)

OTP, email register/login, forgot/reset : **Pub**. Le visiteur les utilise depuis le prompt. `device_name` à envoyer à la connexion.

## 9. Gateway vs client

- Gateway : pas de JWT.
- Android `AuthInterceptor` : pas de jeton → **aucun** header (déjà correct pour un futur visiteur).
- `TokenAuthenticator` : 401 **sans** header → pas de refresh (déjà correct).
- 401 **avec** jeton : refresh une fois ; `refresh_token_invalid` / `session_revoked` → clear + aujourd’hui `goToSignIn` (vide la pile). En mode visiteur, une session morte ne doit pas empêcher de **revenir** au contenu public après fermeture du login.

## 10. Android : mur actuel (indépendant des API publiques)

```91:101:apps/mobile-android/app/src/main/java/com/poro/app/ui/PoroNavHost.kt
    val start: Any = remember { if (startSignedIn) HomeRoute else PhoneRoute }
    // ...
            if (state == SessionState.SignedOut && !destination.isSignInScreen()) nav.goToSignIn()
```

`isSignInScreen` = parcours auth + légal seulement. Tout le reste (Home, Search, Shop, vidéo isolée) **éjecte** le visiteur.

Même si trending devenait public demain, **sans** changer ce `LaunchedEffect` et le `startDestination`, le mode visiteur reste impossible. Changement Android = lot G1, après autorisation et après B-FEED (ou le Home 401).

## 11. Synthèse

| Capable sans compte (API) | Bloqué API | Bloqué Android seul |
|---|---|---|
| Vidéo ready, HLS, search, profil public, comments GET, stats, shop/produit GET, légal local | `/feed/*`, writes sociales, panier, notif, upload, me, report | Démarrer ailleurs que Phone ; rester sur Home/Search/Shop en `SignedOut` |
