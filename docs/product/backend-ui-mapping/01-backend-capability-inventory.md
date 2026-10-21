# Inventaire des capacités backend

- **Date** : 21 octobre 2026.
- **Nature** : audit et conception. **Aucun** fichier `apps/mobile-android`, service, OpenAPI, infra. **Aucun** commit / push / changement de branche.
- **Preuves** : `docs/api/*.openapi.yaml`, `services/*/internal/routes` ou `*.controller.ts`, tests cités. Une route OpenAPI n’est retenue que si le handler existe.
- **Santé / métriques / JWKS** : inventoriés comme internes (aucune UI produit).

Identifiants `CAP-*` stables pour cette série. Les `AUTH-001` etc. de [mobile-ux-audit/03](../mobile-ux-audit/03-feature-inventory.md) restent valables.

## 1. Services réellement présents

| Service | Port | Preuve routes | Outbox / async |
|---|---|---|---|
| auth | 8081 | `services/auth/internal/routes/routes.go` | — |
| user | 8082 | `profiles.controller.ts` | avatar ré-encodage sync ; événements profil |
| video | 8083 | `services/video/internal/routes/routes.go` | `poro.video.uploaded` → **media-worker** HLS |
| media-worker | — | pas d’HTTP public | FFmpeg, `ready` / `failed` (ADR-0007) |
| feed | 8084 | `services/feed/internal/routes/routes.go` | projection `video.ready` / social |
| social | 8085 | `services/social/internal/routes/routes.go` | compteurs + events like/follow/comment |
| notification | 8086 | `notifications.controller.ts` | inbox depuis events |
| moderation | 8087 | OpenAPI + handlers Python | reports → cases ; `content.removed` |
| search | 8088 | `services/search/internal/routes/routes.go` | projection events |
| shop | 8090 | `shops.controller.ts`, `products.controller.ts` | images sharp ; events shop/product |
| order | 8091 | `cart/checkout/orders/addresses` controllers | COD ; events order.* |
| outbox-relay | — | `/metrics` seulement | SQL → Kafka |
| gateway Traefik | 8000 | `infra/gateway` | pas de JWT |
| stockage | — | SeaweedFS, ADR-0015 | buckets publics/privés |

**Absents du dépôt** (contrats d’événements parfois présents) : `services/payment`, `services/chat`, `services/analytics`, `services/reco`. ClickHouse/Qdrant : profil compose `analytics`, pas d’API produit.

## 2. Capacités par domaine

### AUTH — `docs/api/auth.openapi.yaml`

| ID | Capacité | Routes | Accès | Preuve code / test |
|---|---|---|---|---|
| CAP-AUTH-OTP | OTP request/verify | `POST /auth/otp/request`, `/otp/verify` | Pub | `otp_service.go` ; `PhoneEntryScreen` |
| CAP-AUTH-EMAIL | Register / login email | `POST /email/register`, `/email/login` | Pub | `email_handler_test.go` |
| CAP-AUTH-VERIFY | Vérif email | `POST /email/verification/request`, `/confirm` | Auth / Pub confirm | `EmailVerifyScreen` |
| CAP-AUTH-RESET | Mot de passe oublié | `POST /password/forgot`, `/reset` | Pub | 202 anti-énumération |
| CAP-AUTH-TOKEN | Refresh / logout | `POST /refresh`, `/logout` | Refresh pub+body ; logout Auth | ADR-0002 ; `TokenAuthenticator` |
| CAP-AUTH-ME | Identité session | `GET /auth/me` | Auth | Settings / session |
| CAP-AUTH-SESS | Appareils | `GET /sessions`, `DELETE /sessions/{id}`, `POST /revoke-others` | Auth | `SettingsScreen` |
| CAP-AUTH-DEL | Suppression J+30 | `POST /me/delete`, `/me/delete/cancel` | Auth | `DeleteAccountScreen` ; cancel aussi via reconnexion |
| CAP-AUTH-JWKS | Clés JWT | `GET /.well-known/jwks.json` | Interne services | **Aucune UI** |

### USER — `user.openapi.yaml`

| ID | Capacité | Routes | Accès |
|---|---|---|---|
| CAP-USR-ME | Profil perso | `GET/PATCH /users/me` | Auth |
| CAP-USR-UNAME | Dispo username | `GET /username-availability` | Auth (30/min) |
| CAP-USR-PUB | Profil public | `GET /users/by-username/{u}` | Pub |
| CAP-USR-AVATAR | Avatar | `POST upload-url`, `PUT/DELETE /me/avatar` | Auth ; JPEG/PNG/WebP 5 Mio → WebP 512 |
| CAP-USR-CREATOR | Devenir créateur | `POST /me/creator` | Auth ; **n’ouvre pas** la publication (ADR-0007) |

### VIDEO + WORKER

| ID | Capacité | Routes / job | Accès |
|---|---|---|---|
| CAP-VID-UP | Upload multipart | `POST /videos/uploads`, `…/complete`, `DELETE …/uploads/{id}` | Auth ; 256 Mio, 3 min, MP4/MOV/WebM |
| CAP-VID-GET | Lire une vidéo | `GET /videos/{id}` | Opt ; ready+approved public |
| CAP-VID-LIST | Mes vidéos | `GET /videos` | Auth owner |
| CAP-VID-DEL | Soft-delete | `DELETE /videos/{id}` | Owner |
| CAP-VID-HLS | Diffusion | objets `poro-videos` | GET anonyme si URL |
| CAP-VID-WORK | Transcode | media-worker, pas d’HTTP | Interne ; statuts `processing`/`ready`/`failed` |

### FEED

| ID | Routes | Accès | Note |
|---|---|---|---|
| CAP-FEED-FY | `GET /feed/for-you` | Auth | Session Redis, jamais ses vidéos |
| CAP-FEED-FO | `GET /feed/following` | Auth | |
| CAP-FEED-TR | `GET /feed/trending` | Auth **aujourd’hui** | Algo sans `sub` — B-FEED pour visiteur |

### SOCIAL

| ID | Capacité | Accès write / read |
|---|---|---|
| CAP-SOC-LIKE | Like vidéo | Auth / GET likes+stats Opt |
| CAP-SOC-CMT | Commentaires + replies 1 niv. | POST Auth / GET Opt |
| CAP-SOC-CMT-ED | Éditer commentaire 15 min | Auth auteur — **repo Android, pas d’UI** |
| CAP-SOC-CMT-LK | Like commentaire | Auth |
| CAP-SOC-FLW | Follow | Auth / listes GET Opt — **listes sans écran** |
| CAP-SOC-SHR | Share compté | Auth POST ; Intent local à part |
| CAP-SOC-ULIKES | Vidéos aimées d’un user | GET Opt — **pas d’UI** |

### SEARCH / NOTIF / MOD

| ID | Routes | Accès | UI |
|---|---|---|---|
| CAP-SRC | `GET /search`, `/suggest` | Opt, types videos\|users\|hashtags | `SearchScreen` |
| CAP-NTF-IN | Liste, unread, read, read-all | Auth | `NotificationsScreen` |
| CAP-NTF-PREF | GET/PATCH preferences | Auth | `NotificationPreferencesScreen` |
| CAP-NTF-FCM | PUT/DELETE devices | Auth | **API seule** (pas Firebase) |
| CAP-MOD-REP | `POST /moderation/reports` | Auth | `ReportSheet` (lots 1–3, non commité) |
| CAP-MOD-STAFF | cases + decision | Staff | **Hors app** |

### SHOP / ORDER

Voir [mobile-ux-phase-2/02](../mobile-ux-phase-2/02-marketplace-audit.md). Capacités : ouvrir/fermer boutique, logo, CRUD produit, 10 photos, vitrine, PDP, panier, checkout COD, commandes acheteur/vendeur, carnet d’adresses (CRUD API, UI partielle), admin suspend.

### INTERNES (aucune UI directe)

| ID | Quoi | Visibilité indirecte |
|---|---|---|
| CAP-INT-OUTBOX | outbox-relay | Délai notif / feed |
| CAP-INT-ERASE | consommateurs ADR-0018 | Compte disparu, 404 profil |
| CAP-INT-STORE | buckets ADR-0015 | HLS / images chargent ou 403 |
| CAP-INT-GW | Traefik | Timeout / 502 |
| CAP-INT-OBS | Prometheus, `/metrics` | Hors app |

### ABSENTES (ne pas exposer)

Chat, live, favoris, blocage, payment Wave, search catalogue, reco ML, analytics créateur, post image social, download MP4, grille publique de vidéos.

## 3. Méthode de comptage

- **Capacité** = intention métier (souvent 1–3 routes).
- **Endpoint** = une méthode HTTP documentée hors health/live/ready/metrics.
- Santé : ~2–4 GET par service, hors produit.
- Détail endpoint → UI : [02](02-endpoint-ui-traceability.md).
