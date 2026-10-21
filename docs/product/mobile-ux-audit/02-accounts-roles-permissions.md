# Comptes, rôles et permissions

Les six notions suivantes sont **distinctes** dans le dépôt. Elles ne forment pas une hiérarchie unique.

| Notion | Existe ? | Représentation réelle |
|---|---|---|
| Type de compte | **Non** | Pas de champ `account_type`. |
| Rôle technique | **Oui** | Table `user_roles`, JWT `roles[]`. |
| Permission | **Non stockée** | Dérivée dans chaque handler (`seller_id == sub`, `MODERATOR`/`ADMIN`). |
| Abonnement commercial | **Non** | Aucun plan payant. « Abonnement » = follow social. |
| État du compte | **Oui** | `users.status` + `deletion_scheduled_at`. |
| Statut de vérification | **Partiel** | `email_verified`. OTP prouve le téléphone à la connexion, sans flag persisté. Pas de KYC. |

Preuve de principe : [ADR-0001](../../architecture/adr/0001-roles-multiples.md).

## 1. Rôles techniques

Liste CHECK SQL / OpenAPI auth / modèle Go :

`PERSONAL`, `CREATOR`, `BUSINESS`, `ENTERPRISE`, `ADMIN`, `MODERATOR`, `SUPPORT`.

Un compte possède N rôles. Le JWT porte `roles: []`, relu à chaque refresh (access token 15 min).

| Rôle | Attribution réelle | Usage runtime |
|---|---|---|
| `PERSONAL` | Création OTP ou email | Rôle de base. Suffisant pour publier, liker, ouvrir une boutique. |
| `CREATOR` | Consommateur `poro.user.creator.activated` (`services/auth/internal/consumer/creator.go`) | Présent au prochain refresh. **Aucune route ne le exige.** |
| `BUSINESS` | Consommateur `poro.shop.shop.created` ; **conservé si la boutique ferme** | **Aucune route shop/order ne le exige.** Le vendeur est `owner_id` / `seller_id`. |
| `ENTERPRISE` | Aucun grant | Enum seulement. |
| `ADMIN` | SQL uniquement (PORO-024) | File modération + suspendre/rétablir une boutique. |
| `MODERATOR` | SQL uniquement | Idem ADMIN pour ces deux surfaces. |
| `SUPPORT` | SQL uniquement | Jamais testé dans un handler. |

`packages/shared-go/jwtauth.RequireRole` existe et **n’est monté nulle part**.

Android parse `UserDto.roles` (`GET /auth/me`) et **ne s’en sert pas**. L’UI créateur lit `ProfileDto.is_creator`. L’UI vendeur lit `GET /shops/me`.

## 2. Matrice de permissions (faits vérifiés)

Légende : **Oui** = autorisé pour un JWT actif non bloqué ; **Si propriétaire** = contrôle d’identité, pas de rôle ; **API seulement** = pas d’UI Android ; **—** = rôle sans effet ; **Non** = pas de code.

| Fonctionnalité | PERSONAL | CREATOR | BUSINESS | ENTERPRISE |
|---|---|---|---|---|
| Consulter le feed | Oui (session Android obligatoire) | Oui | Oui | — |
| Lire une vidéo `ready` | Oui (API aussi sans JWT) | Oui | Oui | — |
| Publier une vidéo | Oui (ADR-0007) | Oui | Oui | — |
| Gérer son profil | Oui | Oui | Oui | — |
| Devenir créateur | Oui si username | Déjà créateur (idempotent) | Oui | — |
| Créer une boutique | Oui (`POST /shops`) | Oui | Non : 409 `shop_exists` | — |
| Gérer produits / stock | Si propriétaire | Si propriétaire | Si propriétaire | — |
| Acheter | Oui, sauf sa propre boutique | Idem | Idem | — |
| Consulter des statistiques créateur/vendeur | Non (pas d’analytics) | Non | Non | Non |
| Accéder aux outils avancés / Enterprise | Non | Non | Non | Non défini |
| Signaler un contenu | Oui (API) | Oui | Oui | — |
| File de modération | Non (403) | Non | Non | — |

Staff (hors colonnes produit) : `ADMIN`/`MODERATOR` → `GET/POST /moderation/cases*` et `POST /admin/shops/{id}/suspend|reinstate`. Ces routes sont sur la **même gateway publique**. Un JWT staff dans l’app Android y aurait accès réseau. Aucun écran mobile ne les appelle.

## 3. Inscription et activation par parcours

### 3.1 Compte téléphone (OTP)

1. `POST /api/v1/auth/otp/request` `{phone}` E.164.
2. SMS 6 chiffres, TTL 5 min, 3 essais, cooldown 60 s, 5 codes/heure/numéro, budgets ADR-0017.
3. `POST /otp/verify` `{phone, code, device_name?}`.
4. Premier verify : création `PERSONAL` + `ACTIVE`, événement `poro.auth.user.created`.
5. Service user upsert le profil (consommateur ou repli `GET /users/me`).

Pays autorisés : `OTP_ALLOWED_CALLING_CODES=+225,+221,+237,+234`. Autre → `422 phone_country_not_supported`. Android : `LaunchCountry` (CI, SN, CM, NG), défaut CI.

Texte SMS : français (CI, SN, CM), anglais (+234).

`country_code` n’est **pas** déduit de l’indicatif à la création. **À confirmer** opérationnellement : le profil user peut rester sans pays.

### 3.2 Compte email + mot de passe

1. `POST /email/register` `{email, password 8–128, full_name 2–100, device_name?}`.
2. Compte `PERSONAL` + `ACTIVE`. Email de vérification envoyé en arrière-plan (échec ≠ échec d’inscription).
3. `full_name` est validé puis **jeté** : le profil user n’en hérite pas.
4. Connexion possible **sans** email vérifié.
5. Android enchaîne sur `EmailVerifyRoute` (skippable).

Pas d’API pour lier ensuite un téléphone à un compte email, ni l’inverse.

### 3.3 Créateur

Prérequis : username. `POST /users/me/creator` pose `is_creator` + `creator_since`, outbox `poro.user.creator.activated`. Auth accorde `CREATOR` de façon idempotente. Visible dans le JWT au refresh suivant.

Android : bouton « Devenir créateur » désactivé sans username. Aucune information supplémentaire (catégorie, documents, bio minimale) n’est demandée.

**Pas de vérification d’identité créateur.**

### 3.4 Boutique / vendeur

`POST /shops` : `name` 1–60, `handle` 3–30 immutable, `country_code` ∈ CI/SN/CM/NG (devise XOF/XAF/NGN **définitive**). Une boutique par compte.

Auth accorde `BUSINESS` à `poro.shop.shop.created`. Fermer la boutique ne retire pas le rôle.

**Pas de KYC entreprise, pas de documents, pas de compte boutique séparé.**

### 3.5 Enterprise

Aucun processus d’inscription, aucune condition d’activation, aucun écran. L’enum existe pour ne pas casser une migration future.

## 4. Profil — champs

| Champ | Obligatoire | Public | Source | Notes |
|---|---|---|---|---|
| `user_id` | Oui | Oui | JWT `sub` | |
| `username` | Non à la création ; oui pour créateur et pour être trouvable | Oui | user | 3–30 `[a-z0-9._]`, pas seulement chiffres, liste de réservés |
| `display_name` | Non | Oui | user | Code : max **50** code points. OpenAPI/Android : 200. **Écart.** |
| `bio` | Non | Oui | user | Code : max **160**, 5 lignes. OpenAPI/Android : 1000. **Écart.** |
| `avatar_url` | Non | Oui | user | JPEG/PNG/WebP, 5 MiB, ré-encodé WebP 512×512 |
| `is_creator` / `creator_since` | Non | Oui | user | |
| `country_code` | Non | Non (moi seulement) | événement auth | Souvent null |
| `phone` / `email` | Un des deux à l’identité | Non (auth/me) | auth | Jamais dans le profil public |
| `email_verified` | Non | Non | auth | Ne gate aucune route |
| `roles[]` | Oui | Non | auth | Inutilisé par l’UI |
| `status` | Oui | Non | auth | Voir §5 |
| `language` | Défaut `fr` | Non | auth | Pas de sélecteur Android |
| `deletion_scheduled_at` | Non | Non | auth | Pendant le délai J+30 |

## 5. États du compte

**Pas de statut `PENDING_DELETION`.** La suppression programmée est `deletion_scheduled_at` pendant que `status` reste `ACTIVE`.

| État | Sens | Effet interface |
|---|---|---|
| `PENDING` | Reliquat SQL | Promu `ACTIVE` au login (`ensureActive`). Nouveaux comptes créés `ACTIVE`. |
| `ACTIVE` | Utilisable | App normale. |
| `ACTIVE` + `deletion_scheduled_at` | Suppression à J+30 | Tous les appareils déconnectés à la demande. Reconnexion → snackbar « suppression annulée ». |
| `SUSPENDED` | Bloqué | `403 account_blocked` login/refresh/API. Message Android : « Ce compte est bloqué. Contactez le support. » **Aucune API de suspension de compte** (SQL seulement). |
| `DELETED` | Purge effectuée | Identifiants vidés, profil 404, username redevient libre (ADR-0018). Irrécupérable. |

Boutique (indépendant) : `active` | `closed` | `suspended`.

## 6. Sessions et sécurité du compte

- Access token 15 min RS256 ; refresh opaque 30 jours, rotation, détection de réutilisation (ADR-0002).
- `device_name` max 60 caractères, affiché dans `GET /sessions`.
- Logout, révocation d’une session, révocation des autres : ADR-0013 + registre `sid` (ADR-0016) — **tous les services refusent immédiatement**.
- Reset mot de passe : révoque toutes les sessions.
- Android : `EncryptedTokenStore` + Keystore ; un seul refresh à la fois (`TokenAuthenticator`).

## 7. Restrictions et actions interdites (vérifiées)

- Auto-follow : `400 cannot_follow_self`.
- Auto-achat : `422 own_shop`.
- Auto-signalement : `422 cannot_report_self`.
- Publier un produit sans photo : `422 image_required`.
- Annuler une commande après expédition : interdit en v1.
- Restaurer un commentaire retiré par modération : interdit.
- Retirer un **compte** via la file de modération : `action_not_allowed`.
- Changer de « type » Enterprise : impossible (rôle inexistant en runtime).
- Retirer CREATOR ou BUSINESS : pas d’API (sauf purge compte).

**Changer de type de compte** : on **ajoute** un rôle (créateur, boutique), on ne bascule pas. On ne redevient pas « seulement personnel » par l’UI.

## 8. Écrans et menus spécifiques

| Surface | Condition UI réelle |
|---|---|
| Badge Créateur + CTA « Devenir créateur » | `is_creator` / username — `ProfileScreen` |
| « Ma boutique » / ouverture | `GET /shops/me` — `ShopHomeScreen` |
| Commandes reçues, expédition | Flag de navigation `asSeller`, pas le rôle JWT |
| File modération, admin boutique | Aucun écran |
| Espace Enterprise | Aucun écran |

## 9. Lacunes (sans nouvelle architecture)

1. ENTERPRISE et SUPPORT sont des valeurs d’enum sans produit.
2. Les permissions ne sont pas dans le JWT ; l’UI ne doit pas se fier à `roles[]` pour cacher une action sensible — le serveur décide. Cacher un bouton n’est pas une autorisation.
3. Un membre PERSONAL peut déjà tout publier et tout vendre : le libellé « Devenir créateur » n’ouvre pas un droit de publication, seulement un badge et un rôle futur.
4. Qui pose `SUSPENDED` en production : **À confirmer** (pas d’API, pas de runbook mobile).
5. Politique d’employés d’une boutique : reportée par ADR-0001 tant qu’il n’y a pas de rôles personnalisés.
