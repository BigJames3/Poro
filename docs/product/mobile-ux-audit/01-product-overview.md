# Poro — Vue d’ensemble produit (audit UX/UI mobile)

- **Date de l’audit** : 21 octobre 2026.
- **Périmètre** : reconstitution du produit réel à partir du dépôt, en vue d’une spécification UX/UI mobile.
- **Méthode** : lecture des règles du dépôt, ADR, OpenAPI, runbooks, code des services et de `apps/mobile-android`. Aucun fichier applicatif n’a été modifié.
- **Statuts** : **Implémenté** (code + contrat + usage Android), **Partiel** (backend ou UI incomplet), **Simulé** (comportement de développement seulement), **Planifié** (ADR/backlog, pas de code), **Absent**.

## 1. Vision vérifiée

Poro est une plateforme sociale vidéo destinée à l’Afrique de l’Ouest et du Centre, avec une marketplace intégrée.

Preuves :

- Vision produit : `.cursor/rules/project.md`, `README.md`.
- Pays de lancement : Côte d’Ivoire (+225), Sénégal (+221), Cameroun (+237), Nigeria (+234) — [ADR-0004](../../architecture/adr/0004-decisions-produit-lancement.md).
- Périmètre MVP : **social d’abord** (vidéo, feed, interactions, notifications, modération) ; marketplace en **P1 produit**, immédiatement après — même ADR.
- Paiements prévus (non implémentés) : Wave et GeniusPay — ADR-0004 ; paiement à la livraison seul en v1 — [ADR-0012](../../architecture/adr/0012-marketplace-vague-3.md).

L’application Android n’est plus un gabarit vide : le module `apps/mobile-android` (`:app`, `com.poro.app`) expose déjà authentification, feed HLS, social, recherche, profil, publication, notifications in-app et marketplace en paiement à la livraison.

Les documents `docs/api/ANDROID-INTEGRATION.md`, `docs/audits/AUDIT-ETAT-AVANCEMENT.md` et `docs/architecture/00-etat-des-lieux.md` décrivent encore un client « gabarit Android Studio ». **Ce constat est périmé** pour Android. Le backend décrit dans ces audits l’est aussi en partie (effacement de compte et révocation de sessions ont été livrés ensuite, ADR-0016 et ADR-0018).

## 2. Module mobile concerné

| Élément | Valeur constatée |
|---|---|
| Chemin | `apps/mobile-android/` |
| Module Gradle | `:app` uniquement |
| Identifiant | `com.poro.app` (debug : `com.poro.app.debug`) |
| Langage / UI | Kotlin, Jetpack Compose, Material 3 |
| minSdk / targetSdk | 24 / 37 |
| Permission | `INTERNET` seulement |
| Navigation | Navigation Compose typée (`Routes.kt`, `PoroNavHost.kt`) |
| Injection | `AppContainer` manuel, pas de Hilt |
| Règle dépôt | Ne pas modifier ce dossier depuis Cursor ; développement dans Android Studio — `.cursor/rules/project.md` |

Il n’existe pas d’application iOS. Il n’existe pas de WebView produit. Il n’existe pas de second design system : `PoroTheme` dans `ui/theme/Theme.kt` (palette provisoire).

## 3. Utilisateurs

| Acteur | Preuve | Accès réel |
|---|---|---|
| Visiteur non connecté | Android exige une session pour toute l’app (`PoroNavHost`) | **Aucun parcours Android.** Le backend autorise toutefois certaines lectures publiques (vidéo `ready`, profil par username, boutique, recherche). |
| Membre authentifié | Rôle `PERSONAL` à la création — ADR-0001 | Feed, publication, social, recherche, boutique, commandes. |
| Créateur | `POST /users/me/creator` + événement → rôle JWT `CREATOR` | Badge profil. **La publication vidéo n’exige pas ce rôle** (ADR-0007). |
| Vendeur | `POST /shops` → événement → rôle JWT `BUSINESS` | Une boutique par compte. BUSINESS n’est **pas** exigé pour ouvrir la boutique. |
| Entreprise / Enterprise | Enum `ENTERPRISE` dans auth | **Aucun parcours, aucune permission dérivée.** |
| Modérateur / Admin | Rôles JWT `MODERATOR`, `ADMIN` | APIs de file et de suspension boutique sur la gateway publique. **Aucune UI Android.** |
| Support | Enum `SUPPORT` | **Jamais contrôlé.** |

Il n’existe pas de champ `account_type`. Les catégories « Personnel / Créateur / Entreprise / Enterprise » sont des **rôles cumulables**, pas des types de compte exclusifs. Détail : [02-accounts-roles-permissions.md](02-accounts-roles-permissions.md).

## 4. Architecture réelle (constat)

```
Android Compose  ──HTTPS JWT──▶  Traefik :8000  /api/v1/<domaine>
                                      │
  auth :8081   user :8082   video :8083   feed :8084   social :8085
  notification :8086   moderation :8087   search :8088
  shop :8090   order :8091
                                      │
  outbox SQL → outbox-relay → Redpanda → consommateurs inbox
  Postgres (une base / service) · Redis · SeaweedFS
```

**Absents du dépôt** : `services/payment`, `services/chat`, `services/analytics`, `services/reco`. Contrats d’événements payment présents, aucun producteur.

Point d’entrée unique : `http://127.0.0.1:8000/api/v1/...` (`README.md`, `infra/gateway`).

## 5. Fonctionnalités — synthèse

L’inventaire détaillé est dans [03-feature-inventory.md](03-feature-inventory.md). Vue condensée :

| Domaine | État réel | Android | Limite principale |
|---|---|---|---|
| Auth OTP + email, sessions, suppression J+30 | Implémenté | Oui | SMS réel / SMTP réel jamais validés hors fake |
| Profil, username, avatar, créateur | Implémenté | Oui | `country_code` souvent null ; pas de liaison phone+email |
| Vidéo upload multipart, HLS, lecture | Implémenté | Oui | Envoi tué si l’app est tuée ; pas de mute ; pas d’édition métadonnées |
| Feed Pour toi / Abonnements / Tendances | Implémenté | Oui | Pas de vues ni de watch time ; session obligatoire |
| Like, commentaire (1 niveau), follow, partage | Implémenté | Partiel | Pas d’édition commentaire, pas de listes d’abonnés |
| Recherche vidéos / comptes / hashtags | Implémenté | Oui | Pas de recherche boutique/produit (PORO-026) |
| Notifications in-app + préférences | Implémenté | Partiel | FCM backend prêt, Android sans Firebase |
| Signalement | Implémenté (API) | **Absent** | Aucune UI de report |
| Blocage utilisateur, favoris, chat | Absent | Absent | — |
| Boutique, produits, stock | Implémenté | Oui | Découverte par `@handle` seulement |
| Panier, checkout, commandes | Implémenté | Oui | Paiement à la livraison uniquement |
| Wave / GeniusPay / Mobile Money | Planifié | Absent | Service payment inexistant |
| Admin mobile | APIs existantes | Absent | Contrôle par JWT seulement |

## 6. Décisions produit déjà prises (à conserver)

1. Social d’abord, marketplace ensuite (ADR-0004).
2. Quatre pays, OTP SMS limité à ces indicatifs (ADR-0004, `LaunchCountry`).
3. Un compte peut cumuler PERSONAL + CREATOR + BUSINESS (ADR-0001).
4. Toute personne authentifiée peut publier une vidéo ; CREATOR n’est pas un prérequis (ADR-0007).
5. Une boutique par compte, pays = devise définitive (ADR-0012).
6. Le client n’est jamais source de vérité d’un paiement (ADR-0004, ADR-0012).
7. Suppression de compte à J+30, réversible par reconnexion (ADR-0013).
8. Interface Android en français ; SMS Nigeria en anglais (ADR-0004).

## 7. Limites et incohérences à traiter comme des faits

| Constat | Preuve | Impact UX |
|---|---|---|
| L’app n’a pas de mode visiteur alors que certaines API sont publiques | `PoroNavHost` vs OpenAPI video/user/shop/search | Impossible de parcourir le catalogue ou une vidéo partagée sans compte |
| Onglet Boutique déjà dans la barre principale alors que la marketplace est P1 produit | `TopLevelTab.Shop`, ADR-0004 | Décision de lancement à trancher (Q-01) |
| Palette et typographie provisoires | Commentaire dans `Theme.kt` | Identité visuelle non figée |
| `full_name` d’inscription email ignoré par auth | `auth_service.go` | Le nom saisi à l’inscription n’apparaît pas sur le profil |
| Limites display_name / bio : Android et OpenAPI 200/1000, service user 50/160 | `AccountRules.kt`, `profiles.service.ts` | Échec serveur après une saisie pourtant acceptée par l’UI |
| Liens de partage sans URL publique | `SHARE_BASE_URL` vide par défaut | Partage titre seul, pas de deep link |
| Badge « Créateur » cliquable sans action | `ProfileScreen` `AssistChip(onClick = {})` | Contrôle mort pour TalkBack |

## 8. Documents de cette série

| Fichier | Contenu |
|---|---|
| [02-accounts-roles-permissions.md](02-accounts-roles-permissions.md) | Comptes, rôles, permissions |
| [03-feature-inventory.md](03-feature-inventory.md) | Inventaire fonctionnel |
| [04-user-journeys.md](04-user-journeys.md) | Parcours |
| [05-mobile-screen-inventory.md](05-mobile-screen-inventory.md) | Écrans |
| [06-navigation-architecture.md](06-navigation-architecture.md) | Navigation |
| [07-design-system.md](07-design-system.md) | Design system |
| [08-api-ui-contracts.md](08-api-ui-contracts.md) | Contrats écran ↔ API |
| [09-accessibility-and-mobile.md](09-accessibility-and-mobile.md) | Accessibilité et contraintes mobiles |
| [10-acceptance-criteria.md](10-acceptance-criteria.md) | Critères d’acceptation |
| [11-prioritized-roadmap.md](11-prioritized-roadmap.md) | Priorités P0 / P1 / P2 |
| [12-open-questions.md](12-open-questions.md) | Décisions restantes |

**Consolidation proposée (sans duplication)** : cet audit complète `docs/audits/` (état technique) et `docs/api/` (contrats). Il ne les remplace pas. Une mise à jour de `ANDROID-INTEGRATION.md` et de `00-etat-des-lieux.md` est recommandée (PORO-032) pour retirer l’affirmation « client inexistant ».
