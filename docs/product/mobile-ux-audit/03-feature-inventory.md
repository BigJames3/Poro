# Inventaire fonctionnel

Chaque ligne est un fait vérifié. Une fonctionnalité **planifiée** n’est jamais présentée comme disponible. Les identifiants `AUTH-xxx` etc. sont stables pour cette série.

Légende **État réel** : Implémenté · Partiel · Simulé · Planifié · Absent.

---

## AUTH — Identité

### AUTH-001 — Connexion / inscription par SMS OTP

| Champ | Contenu |
|---|---|
| Objectif | Ouvrir une session avec un numéro des pays de lancement |
| État réel | Implémenté |
| Preuve | `services/auth/internal/service/otp_service.go` ; `PhoneEntryScreen` / `OtpVerifyScreen` ; `docs/api/auth.openapi.yaml` |
| Acteur | Visiteur |
| Prérequis | Indicatif +225/+221/+237/+234 ; réseau ; fournisseur SMS hors `dev` |
| Actions | Saisir le numéro, recevoir un code, valider, renvoyer, changer de numéro |
| Données | Téléphone E.164, code 6 chiffres, `device_name` |
| API | `POST /api/v1/auth/otp/request`, `/otp/verify` |
| Permissions | Public |
| États UI | Saisie, envoi, countdown, code, erreur pays/réseau/throttle, succès → Home |
| Dépendances | AUTH-005 |
| Risques | SMS réel non validé en production ; coût SMS pumping borné (ADR-0017) mais fournisseur **À confirmer** |

### AUTH-002 — Inscription et connexion email

| Champ | Contenu |
|---|---|
| Objectif | Compte email + mot de passe |
| État réel | Implémenté |
| Preuve | `EmailRegisterScreen`, `EmailLoginScreen` ; `auth_service.go` |
| Acteur | Visiteur |
| Prérequis | Email unique, mot de passe 8–128 |
| Actions | Créer, se connecter, basculer vers le téléphone |
| Données | email, password, `full_name` (jeté côté auth) |
| API | `POST /email/register` (201), `/email/login` |
| Permissions | Public |
| États UI | Validation locale, throttle `login_throttled`, `email_taken`, `invalid_credentials` |
| Dépendances | AUTH-005 |
| Risques | Le nom d’inscription n’alimente pas le profil |

### AUTH-003 — Vérification d’email

| Champ | Contenu |
|---|---|
| Objectif | Prouver l’accès à l’adresse |
| État réel | Implémenté (non bloquant) |
| Preuve | `EmailVerifyScreen` ; `email_service.go` |
| Acteur | Membre avec email |
| API | `POST /email/verification/request` (Bearer), `/confirm` |
| Permissions | Session |
| États UI | Code 15 min, skip « Plus tard », déjà vérifié 409 |
| Risques | Ne débloque aucune autre fonctionnalité |

### AUTH-004 — Mot de passe oublié

| Champ | Contenu |
|---|---|
| État réel | Implémenté |
| Preuve | `ForgotPasswordScreen`, `ResetPasswordScreen` |
| API | `POST /email/password/forgot` (toujours 202), `/password/reset` (204) |
| Actions | Demander un code, choisir un nouveau mot de passe |
| Effet | Toutes les sessions révoquées ; adresse marquée vérifiée |
| Risques | 202 même si l’email est inconnu (anti-énumération) : l’UI doit rester neutre |

### AUTH-005 — Session, refresh, expiration

| Champ | Contenu |
|---|---|
| État réel | Implémenté |
| Preuve | `EncryptedTokenStore`, `TokenAuthenticator`, ADR-0002, ADR-0016 |
| API | `POST /auth/refresh`, `POST /auth/logout` |
| Règle | Un refresh à la fois ; replay hors 30 s → révocation famille |
| États UI | Splash `SessionState.Unknown` ; snackbar `session_expired` ; retour `PhoneRoute` |

### AUTH-006 — Déconnexion

| Champ | Contenu |
|---|---|
| État réel | Implémenté |
| Preuve | `SettingsScreen` ; `POST /auth/logout` |
| Effet | Jetons effacés, pile auth réinitialisée |

### AUTH-007 — Appareils connectés

| Champ | Contenu |
|---|---|
| État réel | Implémenté |
| Preuve | `SettingsViewModel` ; `GET /sessions`, `DELETE /sessions/{id}`, `POST /sessions/revoke-others` |
| Données | id, device, ip, created_at, last_active_at, current |
| Risques | Afficher `ip_address` est une donnée personnelle ; déjà renvoyée par l’API. L’UI l’affiche — à revoir (Q-12). |

### AUTH-008 / AUTH-009 — Suppression de compte et annulation

| Champ | Contenu |
|---|---|
| État réel | Implémenté (backend + Android pour la demande ; propagation ADR-0018) |
| Preuve | `DeleteAccountScreen` ; `POST /me/delete`, `/me/delete/cancel` |
| Prérequis | Mot de passe (email) ou OTP frais (téléphone) + case de confirmation |
| Effet | J+30, déconnexion globale ; reconnexion annule ; purge anonymise tous les services |
| Risques | Durée de conservation des commandes : validation juridique ouverte (ADR-0018) |

---

## USER — Profil

### USER-001 — Profil personnel

Implémenté. `GET/PATCH /users/me`, `ProfileScreen` / `EditProfileScreen`. Username, display_name, bio, avatar.

### USER-002 — Disponibilité du username

Implémenté. `GET /users/username-availability`. UI : checking / available / invalid / reserved / taken.

### USER-003 — Avatar

Implémenté. Upload présigné puis `PUT /me/avatar`, `DELETE /me/avatar`. Photo Picker, pas de permission stockage.

### USER-004 — Devenir créateur

Implémenté. `POST /me/creator`. N’accorde pas un droit de publication supplémentaire.

### USER-005 — Profil public

Implémenté. `GET /users/by-username/{username}` (public) ; Android ouvre aussi par `userId` via stats/follow. **Pas de grille de vidéos** sur le profil public.

---

## VIDEO

### VIDEO-001 — Publication

Implémenté. `POST /videos/uploads` → PUT parties 8 Mio → `complete` → poll `GET /videos/{id}`. Formats MP4/MOV/WebM, 256 Mio, 3 min, 4096 px. Statuts `uploading` → `processing` → `ready` | `failed` (`too_long`, `invalid_media`, `transcode_failed`).

Android : `UploadScreen`, `UploadManager`. **Partiel** : l’envoi ne survit pas à la mort du processus (pas de WorkManager).

### VIDEO-002 — Lecture HLS

Implémenté. ExoPlayer/Media3, boucle, tap pause/reprise, miniature jusqu’à la 1re frame, pause en arrière-plan. **Pas de mute, pas de barre de progression, pas de plein écran distinct, pas de sous-titres.**

### VIDEO-003 — Mes vidéos

Implémenté. `GET /videos` (propriétaire). Grille sur le profil. Statuts : traitement, échec, retirée.

### VIDEO-004 — Suppression

Implémenté. `DELETE /videos/{id}` (propriétaire), dialogue de confirmation. Soft-delete, médias en quarantaine. **Pas d’édition de titre/description après coup** (PORO-038).

### VIDEO-005 — Visibilité et modération média

Implémenté côté serveur. `moderation_status` `approved` | `removed`. Propriétaire : média retiré sans URL ; tiers : 404. Android : texte `video_removed`.

---

## FEED

### FEED-001 / 002 / 003 — Pour toi, Abonnements, Tendances

Implémenté. `GET /feed/for-you|following|trending`. Règles ADR-0009 (mix 60/30/10, 72 h trending, pas de vues). Android : `FeedScreen` pager vertical, 3 onglets.

`liked_by_me` n’est pas dans le feed : le client lit `GET /videos/{id}/stats`.

Pas de mode hors-ligne de lecture. Session Android obligatoire alors que GET vidéo est public.

---

## SOCIAL

| ID | Nom | État | API | Android |
|---|---|---|---|---|
| SOC-001 | J’aime vidéo | Implémenté | `PUT/DELETE /videos/{id}/like` | Oui |
| SOC-002 | Commentaires | Implémenté | `POST/GET /videos/{id}/comments` | Oui (`CommentsSheet`) |
| SOC-003 | Réponses (1 niveau) | Implémenté | `parent_id` ; 422 `comment_reply_depth` | Oui |
| SOC-004 | J’aime commentaire | Implémenté | `PUT/DELETE /comments/{id}/like` | Oui |
| SOC-005 | Éditer un commentaire | Implémenté API (15 min, auteur) | `PATCH /comments/{id}` | **Repo seulement, pas d’UI** |
| SOC-006 | Supprimer un commentaire | Implémenté | auteur ou propriétaire de la vidéo | Oui |
| SOC-007 | Abonnement | Implémenté | `PUT/DELETE /users/{id}/follow` | Oui (profil public) |
| SOC-008 | Partage comptabilisé | Partiel | `POST /videos/{id}/shares` `whatsapp\|copy_link\|other` | Oui ; URL absente si `SHARE_BASE_URL` vide |
| SOC-009 | Listes abonnés / abonnements | Implémenté API | `GET .../followers`, `/following` | **Pas de client** |
| SOC-010 | Liste des likes | Implémenté API | `GET /videos/{id}/likes`, `/users/{id}/likes` | **Pas de client** |
| SOC-011 | Favoris / signets | Absent | — | — |
| SOC-012 | Blocage / mute | Absent | — | — |

Limites commentaires : 1–1000 caractères (message Android : aussi 20 retours à la ligne). Rate : likes 60/min, comments 10/min, follows 30/min, shares 30/min.

---

## SEARCH

### SEARCH-001 — Recherche

Implémenté. `GET /api/v1/search?q&type=videos|users|hashtags`. Android : 3 onglets, pagination curseur.

### SEARCH-002 — Suggestions

Implémenté. `GET /search/suggest` → users ≤5, hashtags ≤5. Debounce Android.

### SEARCH-003 — Recherche boutique / produit

**Absent** (PORO-026). Android : saisie d’un `@handle` connu (`ShopHomeViewModel`).

---

## NOTIF

### NOTIF-001 — Boîte in-app

Implémenté. `GET /notifications`, mark read, read-all, unread-count. Types : like, comment, reply, follow, video_ready, order_*. Likes regroupés. Android : cloche + liste, navigation vers vidéo / profil / commande.

### NOTIF-002 — Préférences

Implémenté. `GET/PATCH /notifications/preferences` : `push_enabled`, `likes`, `comments`, `follows`, `video_ready`, `orders` (orders coupe le **push** seulement).

### NOTIF-003 — Push FCM

**Partiel.** Backend `PUT/DELETE /notifications/devices` prêt. Android : pas de Firebase, pas de `POST_NOTIFICATIONS`, pas d’appel devices. Le switch `prefs_push` écrit le serveur sans jeton d’appareil.

---

## MOD — Modération

### MOD-001 — Signalement

**Partiel.** API `POST /moderation/reports` (Bearer) : cible video \| comment \| user ; motifs spam, nudity, violence, harassment, hate, fraud, other. **Aucune UI Android.**

### MOD-002 — File modérateur

Implémenté API, **absent** Android (volontaire : staff). `GET /moderation/cases`, `POST .../decision` remove \| dismiss \| restore.

### MOD-003 — Filtre automatique texte FR

Implémenté serveur (`rules/fr.yml`). Pas d’UI. Retrait immédiat si règle bloquante (sauf profil → revue).

Produits / boutiques : **non** ciblables par un report (PORO-025). Suspension boutique : admin API seulement.

---

## SHOP

| ID | Nom | État | Preuve |
|---|---|---|---|
| SHOP-001 | Ouvrir / fermer / rouvir une boutique | Implémenté | `MyShopScreen` ; `POST /shops`, `/me/close`, `/me/reopen` |
| SHOP-002 | Identité boutique (nom, description, logo) | Partiel UI | Logo upload oui ; **DELETE logo** API sans Android |
| SHOP-003 | Catalogue vendeur (draft/active/archived) | Implémenté | `ProductEditorScreen` |
| SHOP-004 | Variantes, prix, stock | Implémenté | max 50 variantes ; stock 0–1_000_000 |
| SHOP-005 | Photos produit | Implémenté | max 10, WebP ≤1080 px |
| SHOP-006 | Vitrine publique | Implémenté | `GET /shops/{handle}` + products ; `ShopScreen` |
| SHOP-007 | Fiche produit | Implémenté | `ProductScreen` ; `in_stock` indicatif |
| SHOP-008 | Découverte / catégories / recherche | Absent | PORO-026 |
| SHOP-009 | Avis, remises, stats vendeur | Absent | — |
| SHOP-010 | Admin suspend/reinstate | Implémenté API | Pas d’Android |

Pays boutique → devise : CI/SN = XOF, CM = XAF, NG = NGN. Montants entiers en unités mineures.

---

## ORDER / PAY

| ID | Nom | État | Notes |
|---|---|---|---|
| ORDER-001 | Panier | Implémenté | qty 1–99, 100 lignes, pas d’achat chez soi |
| ORDER-002 | Checkout preview + confirm | Implémenté | TTL preview 15 min ; une commande par boutique |
| ORDER-003 | Paiement à la livraison | Implémenté | Seul `PAYMENT_METHODS` actif |
| ORDER-004 | Carnet d’adresses | Partiel | Préremplissage + save-on-checkout ; CRUD adresses API peu exposé |
| ORDER-005 | GPS livraison | Partiel API | Android n’envoie pas `location` (pas de permission LOCATION) |
| ORDER-006 | Commandes acheteur | Implémenté | liste, détail, annuler avant ship, confirmer réception |
| ORDER-007 | Commandes vendeur | Implémenté | ship + tracking texte, annuler avant ship |
| ORDER-008 | Frais de livraison calculés | Absent | Mention « payés au livreur » seulement |
| PAY-001 | Wave | Planifié | PORO-015 ; enum order, pas de service |
| PAY-002 | GeniusPay | Planifié | PORO-035 ; **absent des enums** |
| PAY-003 | Orange Money / CinetPay / MoMo | Planifié / vision | Pas de code |
| PAY-004 | Fournisseur `simulated` | Simulé | Refusé hors `APP_ENV=dev` ; aucun producteur d’événements |
| PAY-005 | Commissions / reversements | Planifié | ADR-0012 journal manuel, non codé |

Cycle commande : `pending` → (COD) `confirmed` → `shipped` → `completed` (confirm ou 7 jours) / `cancelled`. Chemins `awaiting_payment` / `paid` existent pour un futur payment service.

L’UI **ne doit jamais** afficher un succès de paiement en ligne : ce chemin n’existe pas. Pour le COD, le succès est « commande passée » (`order.placed` / stock réservé), pas « payé ».

---

## AUTRES

| ID | Nom | État |
|---|---|---|
| CHAT-001 | Messagerie / présence | Absent (PORO-040) |
| LIVE-001 | Live | Absent (P3 vision) |
| ADS-001 | Publicité | Absent |
| ANALYTICS-001 | Stats créateur | Absent (service analytics inexistant) |
| RECO-001 | Reco ML | Absent (feed heuristique seulement) |
| LEGAL-001 | CGU / confidentialité in-app | Absent |
| GUEST-001 | Parcours visiteur | Absent Android (certaines API publiques) |

---

## Dépendances majeures

```
AUTH-005 ── USER-001 ── USER-004 (créateur)
                └── VIDEO-001 ── FEED-* ── SOC-*
                                  └── SEARCH-001
                                  └── NOTIF-001
SHOP-001 ── SHOP-003 ── ORDER-001 ── ORDER-002 ── ORDER-003 (COD)
PAY-001 (Wave) dépend d’un service payment inexistant → ne pas concevoir l’UI comme disponible.
MOD-001 (report) API prête → UI manquante, indépendante du reste.
```
