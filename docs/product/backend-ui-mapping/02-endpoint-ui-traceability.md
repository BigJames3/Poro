# Matrice endpoint → UI

Contrats : `docs/api/*.openapi.yaml`. UI : `apps/mobile-android/...` (lecture).  
Écart : **Aucun** · **Partiel** · **API seule** · **UI seule** · **Bloqué** · **Interne** · **Staff**.  
Priorité : P0 sécurité/honnêteté · P1 API prête sans UI · P2 · **Hors app**.

`returnTo` / visiteur : [guest-mode](../guest-mode/01-guest-mode-product-spec.md). Médias : [mobile-media](../mobile-media/01-scope-and-decisions.md).

## AUTH

| ID | Endpoint | Métier | Accès | UI existante | UI cible | Écart | P | Test |
|---|---|---|---|---|---|---|---|---|
| E-A01 | `POST /auth/otp/request` | Envoyer OTP | Pub | `PhoneEntryScreen` | idem | Aucun | P0 | AC-AUTH-01 |
| E-A02 | `POST /auth/otp/verify` | Session téléphone | Pub | `OtpVerifyScreen` | + `returnTo` guest | Partiel guest | P0 | AC-AUTH-01 |
| E-A03 | `POST /email/register` | Créer compte email | Pub | `EmailRegisterScreen` | idem | Aucun* | P0 | AC-AUTH-06 |
| E-A04 | `POST /email/login` | Session email | Pub | `EmailLoginScreen` | + returnTo | Partiel guest | P0 | — |
| E-A05 | `POST /email/verification/request` | Renvoyer code | Auth | `EmailVerifyScreen` | idem | Aucun | P1 | AC-AUTH-12 |
| E-A06 | `POST /email/verification/confirm` | Marquer vérifié | Auth | idem | idem | Aucun | P1 | — |
| E-A07 | `POST /email/password/forgot` | 202 toujours | Pub | `ForgotPasswordScreen` | idem | Aucun | P0 | AC-AUTH-07 |
| E-A08 | `POST /email/password/reset` | Nouveau mdp + révocation | Pub | `ResetPasswordScreen` | idem | Aucun | P0 | AC-AUTH-08 |
| E-A09 | `POST /auth/refresh` | Rotation | Intercepteur | `TokenAuthenticator` | invisible | Aucun | P0 | NR refresh |
| E-A10 | `POST /auth/logout` | Fin session appareil | Auth | Settings | + option visiteur | Aucun | P0 | — |
| E-A11 | `GET /auth/me` | Compte auth | Auth | Settings implicite | ne pas appeler visiteur | Aucun | P0 | GST-03 |
| E-A12 | `GET /auth/sessions` | Liste appareils | Auth | `SettingsScreen` | masquer IP ? Q-09 | Partiel PII | P1 | AC-AUTH-10 |
| E-A13 | `DELETE /auth/sessions/{id}` | Révoquer un | Auth | Settings | idem | Aucun | P0 | — |
| E-A14 | `POST /sessions/revoke-others` | Tous sauf ici | Auth | Settings | idem | Aucun | P0 | AC-AUTH-10 |
| E-A15 | `POST /auth/me/delete` | J+30 | Auth | `DeleteAccountScreen` | honnête sur conservation | Aucun | P0 | — |
| E-A16 | `POST /me/delete/cancel` | Annuler | Auth | Reconnexion `deletion_cancelled` | écran dédié optionnel | Partiel | P2 | — |

\* `full_name` inscription jeté (Q-22).

## USER

| ID | Endpoint | Accès | UI existante | Écart | P |
|---|---|---|---|---|---|
| E-U01 | `GET/PATCH /users/me` | Auth | `ProfileScreen` / `EditProfileScreen` | Limites UI 200/1000 vs serveur 50/160 | P0 |
| E-U02 | `GET /username-availability` | Auth | EditProfile | Aucun | P0 |
| E-U03 | `GET /users/by-username/{u}` | Pub | `PublicProfileScreen` | Android session aujourd’hui | P0 |
| E-U04–06 | Avatar upload-url / PUT / DELETE | Auth | EditProfile `ImageOnly` | Aucun | P0 |
| E-U07 | `POST /me/creator` | Auth | Profile CTA | Copie trompeuse possible Q-04 | P1 |

## VIDEO

| ID | Endpoint | Accès | UI | Écart | P |
|---|---|---|---|---|---|
| E-V01 | `POST /videos/uploads` | Auth | `UploadScreen` | Desc UI 1000 vs 500 runes | P1 |
| E-V02 | `POST …/complete` | Auth | `UploadManager` | invisible | — |
| E-V03 | `DELETE …/uploads/{id}` | Auth | Cancel upload | Peu exposé | P2 |
| E-V04 | `GET /videos` | Owner | `MyVideos` sur profil | Aucun | P0 |
| E-V05 | `GET /videos/{id}` | Opt | `VideoScreen`, player | Guest mur Android | P0 |
| E-V06 | `DELETE /videos/{id}` | Owner | Dialogue Mes vidéos | Pas d’édition titre PORO-038 | P1 |
| E-V07 | HLS GET objet | Pub URL | `VideoPlayer` | Mute P1 ; pas d’offline | P1 |

Worker : **aucune UI directe** — cartes `processing` / `failed` / `video_ready` notif.

## FEED / SEARCH

| ID | Endpoint | Accès | UI | Écart | P |
|---|---|---|---|---|---|
| E-F01 | `GET /feed/for-you` | Auth | `FeedScreen` kind | Guest interdit (correct) | P0 |
| E-F02 | `GET /feed/following` | Auth | idem | Empty + CTA search | P0 |
| E-F03 | `GET /feed/trending` | Auth | idem | **B-FEED** si visiteur | P0 |
| E-S01 | `GET /search` | Opt | `SearchScreen` 3 onglets | Pas products | P0 |
| E-S02 | `GET /search/suggest` | Opt | debounce | Aucun | P1 |

## SOCIAL

| ID | Endpoint | Accès | UI existante | UI cible | Écart | P |
|---|---|---|---|---|---|---|
| E-SO01/02 | PUT/DELETE `/videos/{id}/like` | Auth | Overlay ♥ | Prompt visiteur | Aucun membre | P0 |
| E-SO03 | GET `…/likes` | Opt | — | P2 liste | API seule | P2 |
| E-SO04 | GET `…/stats` | Opt | Compteurs overlay | idem | Aucun | P0 |
| E-SO05 | GET `/users/{id}/likes` | Opt | — | « Aimées » P2 | API seule | P2 |
| E-SO06 | POST `…/comments` | Auth | `CommentsSheet` | Prompt visiteur | Aucun membre | P0 |
| E-SO07 | GET `…/comments` | Opt | Sheet | Guest lecture | Mur Android | P0 |
| E-SO08 | GET `/comments/{id}/replies` | Opt | Sheet 1 niveau | idem | Aucun | P0 |
| E-SO09 | PATCH `/comments/{id}` | Auteur 15 min | Repo seulement | Menu Modifier | API seule | P1 |
| E-SO10 | DELETE `/comments/{id}` | Auteur ou owner vidéo | Sheet | idem | Aucun | P0 |
| E-SO11/12 | PUT/DELETE like comment | Auth | Sheet | Prompt guest | Aucun | P1 |
| E-SO13/14 | PUT/DELETE follow | Auth | `PublicProfileScreen` | Prompt guest | Aucun | P0 |
| E-SO15/16 | GET followers/following | Opt | — | `FollowListRoute` | API seule | P1 |
| E-SO17 | GET `/users/{id}/stats` | Opt | Compteurs profil (si branchés) | idem | Partiel | P1 |
| E-SO18 | POST `…/shares` | Auth | `ShareSheet` | Intent ± POST | Partiel si `SHARE_BASE_URL` vide | P1 |

## NOTIF / MOD

| ID | Endpoint | Accès | UI | Écart | P |
|---|---|---|---|---|---|
| E-N01 | GET `/notifications` | Auth | `NotificationsScreen` | Guest = prompt | P0 |
| E-N02 | GET unread-count | Auth | Badge cloche | 0 inventé interdit visiteur | P0 |
| E-N03 | PATCH `/{id}/read` | Auth | Tap item | Aucun | P1 |
| E-N04 | POST read-all | Auth | Écran | Aucun | P1 |
| E-N05/06 | GET/PATCH preferences | Auth | `NotificationPreferencesScreen` | Switch push **mensonger** sans FCM | P0 |
| E-N07/08 | PUT/DELETE devices | Auth | — | API seule | P1 |
| E-M01 | POST `/moderation/reports` | Auth | `ReportSheet` | Guest prompt | P0 |
| E-M02–04 | cases* | Staff | **Aucune** | Staff | Hors app |

## SHOP

| ID | Endpoint | Accès | UI | Écart | P |
|---|---|---|---|---|---|
| E-SH01 | POST `/shops` | Auth | `MyShopScreen` form | Pays figé | P0 |
| E-SH02/03 | GET/PATCH `/shops/me` | Owner | Dashboard | `hasShop` null | P0 |
| E-SH04/05 | close / reopen | Owner | Dashboard | Suspendue : pas d’admin | P0 |
| E-SH06/07 | logo upload-url / PUT | Owner | Picker ImageOnly | Aperçu local manquant | P1 |
| E-SH08 | DELETE logo | Owner | — | API seule | P2 |
| E-SH09–13 | CRUD produits + stock | Owner | `ProductEditorScreen` | Photos après create | P0 |
| E-SH14–16 | images upload / PUT / DELETE | Owner | Éditeur max 10 | Pending local spec M2 | P1 |
| E-SH17/18 | GET `{handle}`, `{handle}/products` | Pub | `ShopScreen` | Guest mur | P0 |
| E-SH19 | GET `/products/{id}` | Pub | `ProductScreen` | Images only | P0 |
| E-SH20/21 | admin suspend/reinstate | Staff | — | Hors app | Hors app |

## ORDER

| ID | Endpoint | Accès | UI | Écart | P |
|---|---|---|---|---|---|
| E-O01 | GET `/cart` | Auth | `CartScreen` | Guest prompt | P0 |
| E-O02 | DELETE `/cart` | Auth | — | API seule | P2 |
| E-O03/04 | PUT/DELETE item | Auth | Stepper | Somme ≤99 lot 1 | P0 |
| E-O05 | GET prefill | Auth | Checkout | Erreur visible lot 1 | P0 |
| E-O06/07 | POST preview / confirm | Auth | 2 étapes | Jamais « Payée » ; pas de replay auto | P0 |
| E-O08–12 | addresses CRUD + default | Auth | Chips checkout | CRUD écran carnet **absent** | P1 |
| E-O13–16 | orders get/cancel/receipt | Acheteur | `OrdersRoute` / `OrderRoute` | Aucun | P0 |
| E-O17–20 | seller orders / ship / cancel | Vendeur | `asSeller=true` | Tracking texte | P0 |

## INTERNES (pas d’écran)

Health, metrics, JWKS, outbox-relay, media-worker HTTP, seaweed init, Kafka DLQ : **Interne**. Résultat visible = latence, `processing`, notif `video_ready`, 502 gateway.

## UI sans backend

| UI | Manque | Traitement |
|---|---|---|
| Switch push OS | Firebase + devices | Labelliser inopérant Q-21 |
| Onglet Boutique « catalogue » | PORO-026 | Ne pas inventer |
| Fil image | B-IMG-POST | Spec only |
| Favoris / blocage / chat | Absent | Pas de bouton |
| Wave au checkout | `services/payment` | COD only |
| Mode visiteur fil | B-FEED + mur Android | Spec guest-mode |

## Navigation Android (référence)

`Routes.kt` : Phone, Otp, Email*, Forgot, Reset, Home, EmailVerify, Search, Notifications, NotifPrefs, Upload, Video, PublicProfile, Profile, EditProfile, Settings, Legal*, DeleteAccount, ShopHome, Shop, Product, Cart, Checkout, Orders, Order, MyShop, ProductEditor.

Sheets : `CommentsSheet`, `ShareSheet`, `ReportSheet`. Pas de `FollowListRoute`, pas de carnet d’adresses dédié, pas de `GuestAuthSheet`.
