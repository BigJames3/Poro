# Contrats écran ↔ API

Référence des contrats : `docs/api/*.openapi.yaml`. Enveloppe unique `{data, error, meta.request_id}`. L’UI se branche sur `error.code`, jamais sur `error.message`.

Auth : `Authorization: Bearer`. Rafraîchissement : une requête à la fois. Gateway : `http://127.0.0.1:8000/api/v1/...`.

Légende **Couverture** : OK (écran + API) · UI seule · API seule · Absent.

---

## Enveloppe, retry, 401

| Sujet | Contrat | Client Android |
|---|---|---|
| Erreur | `{code, message}` | `toErrorMessage()` dans `AuthErrorMessages.kt` |
| `X-Request-ID` | UUID par requête | `RequestIdInterceptor` ; affiché si erreur inattendue |
| 429 | `Retry-After` secondes | Countdown auth ; ailleurs message throttle |
| 401 métier | refresh puis replay | `TokenAuthenticator` |
| `refresh_token_invalid` / `session_revoked` | login | Jetons effacés, `PhoneRoute` |

---

## Authentification

| Écran | Méthode | Chemin | Corps / query | Succès | Erreurs UI mappées |
|---|---|---|---|---|---|
| UI-AUTH-001 | POST | `/auth/otp/request` | `{phone}` | 200 `{expires_in}` | country, unreachable, otp_throttled, sms_unavailable, rate_limited |
| UI-AUTH-002 | POST | `/auth/otp/verify` | `{phone,code,device_name?}` | 200 tokens + user | otp_invalid/expired/too_many, account_blocked |
| UI-AUTH-003 | POST | `/auth/email/login` | `{email,password,device_name?}` | 200 | invalid_credentials, login_throttled, account_blocked |
| UI-AUTH-004 | POST | `/auth/email/register` | `{email,password,full_name,device_name?}` | 201 | email_taken |
| UI-AUTH-005 | POST | `/auth/email/verification/request` | — | 202 | email_already_verified, email_missing, email_throttled, email_unavailable |
| UI-AUTH-005 | POST | `/auth/email/verification/confirm` | `{code}` | 200 user | code_invalid/expired/too_many |
| UI-AUTH-006 | POST | `/auth/email/password/forgot` | `{email}` | 202 toujours | throttle |
| UI-AUTH-007 | POST | `/auth/email/password/reset` | `{email,code,new_password}` | 204 | code_*, account_blocked |
| — | POST | `/auth/refresh` | `{refresh_token}` | 200 tokens | intercepté |
| UI-PRF-003 | POST | `/auth/logout` | — | 204 | |
| UI-PRF-003 | GET | `/auth/sessions` | — | 200 | |
| UI-PRF-003 | DELETE | `/auth/sessions/{id}` | — | 204 | session_not_found |
| UI-PRF-003 | POST | `/auth/sessions/revoke-others` | — | 200 `{revoked}` | |
| UI-PRF-001 | GET | `/auth/me` | — | 200 | |
| UI-PRF-004 | POST | `/auth/me/delete` | `{password?}` ou `{otp_code?}` | 202 `{deletion_scheduled_at}` | reauth_required, invalid_credentials, otp_* |
| — | POST | `/auth/me/delete/cancel` | — | 200 | **API sans écran** (reconnexion suffit) |

`login_throttled` : mappé Android, **absent de l’enum OpenAPI** — écart contrat.

---

## Profil

| Écran | Méthode | Chemin | Notes |
|---|---|---|---|
| UI-PRF-001 | GET | `/users/me` | upsert si événement auth en retard |
| UI-PRF-002 | PATCH | `/users/me` | username, display_name, bio |
| UI-PRF-002 | GET | `/users/username-availability?username=` | 30/min |
| UI-PRF-002 | POST | `/users/me/avatar/upload-url` | puis PUT S3, `PUT /me/avatar` |
| UI-PRF-002 | DELETE | `/users/me/avatar` | |
| UI-PRF-001 | POST | `/users/me/creator` | 422 username_required |
| UI-SOC-001 | GET | `/users/by-username/{username}` | public |

Limites réelles serveur : display_name 50, bio 160 — plus strictes que l’OpenAPI.

---

## Vidéo, feed, social, search

| Écran | API | Auth | Remarque |
|---|---|---|---|
| UI-UPL-001 | `POST /videos/uploads` ; PUT parties ; `POST .../complete` ; GET poll | Bearer | `DELETE /uploads/{id}` peu exposé UI |
| UI-UPL-002 | `GET /videos` ; `DELETE /videos/{id}` | Bearer owner | |
| UI-FEED-002 | `GET /videos/{id}` | JWT optionnel | Android envoie le token |
| UI-FEED-001 | `GET /feed/for-you\|following\|trending` | Bearer | limit 1–30, défaut 10 |
| Like | `PUT/DELETE /videos/{id}/like` ; `GET .../stats` | Bearer écriture | |
| UI-SOC-002 | `GET/POST /videos/{id}/comments` ; replies ; like comment ; DELETE | | PATCH edit **API seule** |
| Follow | `PUT/DELETE /users/{id}/follow` ; `GET .../stats` | | followers/following **API seule** |
| Share | `POST /videos/{id}/shares` | `{channel}` | |
| UI-SRC-001 | `GET /search` ; `GET /search/suggest` | JWT optionnel | types videos\|users\|hashtags |

Titre vidéo : code max 100 runes, OpenAPI 200. Description : 500 vs OpenAPI 1000. Android message titre 1–100.

---

## Notifications

| Écran | API |
|---|---|
| UI-NTF-001 | `GET /notifications` ; `PATCH .../read` ; `POST /read-all` ; `GET /unread-count` |
| UI-NTF-002 | `GET/PATCH /notifications/preferences` |
| — | `PUT/DELETE /notifications/devices` | **API seule** (FCM) |

---

## Modération

| Écran | API | Couverture |
|---|---|---|
| UI-MOD-001 (à créer) | `POST /moderation/reports` | API seule aujourd’hui |
| Staff | `GET /moderation/cases` ; `POST .../decision` | **Ne pas brancher** dans l’app grand public |
| Staff shop | `POST /admin/shops/{id}/suspend\|reinstate` | Idem |

---

## Boutique et commandes

| Écran | API | Couverture |
|---|---|---|
| UI-SHP-008 | `POST /shops` ; `GET/PATCH /shops/me` ; close/reopen ; logo upload | DELETE logo **API seule** |
| UI-SHP-009 | CRUD `/shops/me/products` ; stock ; images | |
| UI-SHP-002 | `GET /shops/{handle}` ; `GET .../products` | public |
| UI-SHP-003 | `GET /products/{id}` | public |
| UI-SHP-004 | `GET /cart` ; `PUT/DELETE /cart/items/{variantId}` | `DELETE /cart` **API seule** |
| UI-SHP-005 | `GET /checkout/prefill` ; `POST /preview` ; `POST /confirm` | `payment_method` = `cash_on_delivery` |
| Adresses | `GET/POST /addresses` ; PUT/DELETE ; default | **Partiel** (save_address du preview) |
| UI-SHP-006/007 | `GET /orders` ; `GET /seller/orders` ; cancel ; ship ; confirm-receipt | |

Si l’API n’existe pas (Wave, chat, favoris, block, search produits) : **lacune d’intégration**. Pas de contrat fabriqué. L’écran affiche l’état réel ou n’existe pas.

---

## Chargement, succès, retry (règle unique)

1. Afficher `Loadable.Loading` ou spinner inline.
2. Succès : rendre `data` ; ne pas inventer un statut (ex. ne pas passer une commande à « payée » après un tap).
3. Échec métier : string du `code`.
4. Réseau : `error_network` + Réessayer. Pas de boucle immédiate.
5. 429 : attendre `Retry-After`.
6. Interruption checkout : re-GET commandes plutôt que d’annoncer le succès.

---

## Lacunes d’intégration (UI souhaitable, backend manquant)

| Besoin UX | Backend | Décision d’audit |
|---|---|---|
| Grille vidéos d’un profil public | `GET /videos` = **owner only** | Pas d’écran grille publique |
| Recherche produit | PORO-026 | Pas d’écran résultats produit |
| Favoris | Absent | — |
| Bloquer un utilisateur | Absent | — |
| Chat | Absent | — |
| Wave / Mobile Money | Service payment absent | Checkout reste COD |
| GPS livraison | Champ API optionnel | Android n’a pas la permission ; adresse texte suffit au contrat v1 |
| Stats créateur | analytics absent | — |
| Édition métadonnées vidéo | PORO-038 | — |
| Push OS | devices API ok, Firebase non | Préférence inerte |

## Lacunes inverses (API prête, UI absente)

| API | Action UX |
|---|---|
| `POST /moderation/reports` | UI-MOD-001 P0 |
| `GET .../followers` `.../following` | UI-SOC-004 P1 |
| `PATCH /comments/{id}` | Action « Modifier » dans la sheet, fenêtre 15 min |
| `PUT /notifications/devices` | Après Firebase |
| `DELETE /shops/me/logo` | Action dans MyShop |
| `GET /users/{id}/likes` | P2 « vidéos aimées » |

## Divergences contrat / code / UI

| Sujet | OpenAPI | Code serveur | Android |
|---|---|---|---|
| display_name / bio | 200 / 1000 | 50 / 160 | 200 / 1000 |
| titre / description vidéo | 200 / 1000 | 100 / 500 | titre 100, description 1000 dans l’erreur |
| Token logout valable 15 min (user.yaml) | Documenté | Faux depuis ADR-0016 | Refresh refuse le sid |
| `login_throttled` | Enum incomplet | Renvoyé | Mappé |
| `country_code` user | Présent | Souvent null | Non saisi à l’OTP |
| Guest read | Plusieurs GET publics | Oui | Session obligatoire |

Ces écarts sont des bugs de spécification, pas des libertés de maquette.
