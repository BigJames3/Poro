# Phase 2 — Périmètre, décisions et exclusions

Statut : **proposition pour revue technique et produit**. Rien de ce dossier n'est implémenté.
Aucun fichier de `apps/mobile-android/` n'a été modifié pendant cette phase.

## 1. Sources vérifiées

| Source | Version lue | Remarque |
|---|---|---|
| Code Android | branche `claude/gifted-ritchie-t5ej2z` @ `237075f` (PR #58, brouillon) | L'application n'est pas encore sur `main` |
| Contrats OpenAPI `docs/api/*.yaml` | `origin/main` @ `f1ea1ec` | 6 commits de plus que la base de la PR #58, dont les lectures groupées `GET /api/v1/users?ids=` et `GET /api/v1/videos/stats?ids=` |
| Code des services | `origin/main` @ `f1ea1ec` | Vérification que le code applique les contrats (feed, video, search, moderation, order) |
| ADR | `docs/architecture/adr/0001` à `0018` | ADR-0004 (périmètre), 0009 (feed), 0010 (modération), 0012 (marketplace), 0014 (gateway), 0015 (stockage) |
| Audits | `docs/audits/AUDIT-ETAT-AVANCEMENT.md`, `docs/audits/BACKLOG-PORO.md` | |
| Essai réel sur émulateur | journaux `okhttp` transmis le 2026-10-21 | Voir § 5 |

### Écart documentaire bloquant (Lot 0)

**Le dossier `docs/product/mobile-ux-audit/` (« 12 documents de la phase 1 ») n'existe dans aucune
branche ni aucune PR du dépôt** (vérifié sur les 30+ branches distantes et les PR ouvertes ou
fermées). Il n'existe probablement que sur un poste local, sans avoir été poussé. Ce dossier de
phase 2 s'appuie donc uniquement sur le code, les contrats et les ADR. Quand l'audit sera
disponible, il faudra comparer ses conclusions avec ce dossier (tâche L0-1 du plan).

Il n'y a pas non plus de `CONTRIBUTING.md`, d'`AGENTS.md` ni de `CLAUDE.md` dans le dépôt : les
conventions retenues sont celles observées dans le code (messages en français, commits
conventionnels, contrats `docs/api` comme source de vérité, branchement sur `error.code`).

## 2. Décisions produit reprises (validées par le propriétaire)

| # | Décision | Conséquence dans ce dossier |
|---|---|---|
| D1 | La boutique reste visible ; la marketplace existante est stabilisée | Axe A, Lot 1 : corrections sans nouvelle fonctionnalité commerciale |
| D2 | Mode invité souhaité pour le fil public, sous réserve backend et sécurité | Axe B : **bloqué** en l'état, le feed exige un jeton (voir `03`) |
| D3 | Écrans légaux préparés, textes non publiés sans validation | Axe C : textes locaux marqués « À valider », aucun consentement enregistré (pas d'API) |
| D4 | `PoroTheme` reste la référence visuelle | Axe D : extension du thème existant, pas de second design system |
| D5 | Interfaces natives Kotlin / Jetpack Compose | Aucune WebView pour l'interface ; Material 3 déjà en place |
| D6 | Aucun écran sur une fonctionnalité backend inexistante | Recherche catalogue, Mobile Money, signalement de produit ou de boutique : exclus |
| D7 | Préserver les fonctions sociales et les parcours opérationnels | Tests de non-régression (`08`) obligatoires à chaque lot |

Décision ADR-0004 rappelée : social d'abord, marketplace en P1, paiements Wave et GeniusPay
plus tard ; **le client n'est jamais source de vérité d'un paiement**.

## 3. Dans le périmètre

| Axe | Contenu | Document |
|---|---|---|
| A | Stabilisation des parcours marketplace existants, matrice écran → API | `02` |
| B | Faisabilité du mode invité, matrice d'accès | `03` |
| C | Signalement (vidéo, commentaire, compte) et écrans légaux | `04`, `05` |
| D | Extension documentée de `PoroTheme` | `06` |
| — | Spécifications d'écrans, tests d'acceptation, plan, décisions | `07` à `10` |

## 4. Hors périmètre (explicitement exclu)

| Exclu | Raison vérifiée |
|---|---|
| Découverte et recherche de boutiques ou de produits | Aucune route de liste ou de recherche catalogue (`shop.openapi.yaml`, `search.openapi.yaml` : vidéos, comptes, hashtags) — PORO-026 |
| Paiement Mobile Money (Wave, GeniusPay) | `services/payment` inexistant (PORO-015) ; seul `cash_on_delivery` est servi |
| Signalement d'un produit ou d'une boutique | `target_type` limité à `video`, `comment`, `user` (`moderation.openapi.yaml`, `cases.py`) |
| Enregistrement du consentement aux CGU | Aucune route, aucune table (recherche `consent`, `terms`, `cgu`, `privacy` : 0 résultat) |
| Notifications push | Aucun projet Firebase ; `FCM_CREDENTIALS_B64` vide par défaut |
| Outils de modération (file des cas, décisions) | API réservée aux rôles MODERATOR/ADMIN ; pas un parcours de l'app grand public |
| Modification du backend, de la gateway ou de l'infra | Interdit en phase 2 ; les besoins sont listés dans `10` |

## 5. Constats de l'essai réel sur émulateur (2026-10-21)

| Constat | Cause | Statut |
|---|---|---|
| `GET /api/v1/auth/sessions` → 404 | Image Docker `auth` antérieure à la fonctionnalité (route présente dans le code) | Environnement : `docker compose up -d --build` |
| `GET /api/v1/users/{id}/stats` → 503 | Service `social` absent ou non sain derrière Traefik | Environnement, à confirmer |
| Envoi d'avatar et de photo produit : URL obtenue (201), aucune confirmation | Envoi direct au stockage (port 9000) en échec ; tunnel `adb reverse tcp:9000` probablement absent | **À confirmer** avec le journal `PoroMedia` |
| `PATCH …/products/{id}` → 422 | Publication refusée (`image_required`) en conséquence du point précédent | Dépend du point précédent |
| `PATCH /api/v1/shops/me` envoyé 4 fois | Aucun retour visuel après enregistrement | Corrigé dans `237075f` (message « Boutique enregistrée ») |
| `GET /api/v1/cart` 2 à 3 fois par affichage | Double chargement | Corrigé dans `237075f` |

## 6. Règles de cette phase (rappel)

- Aucun fichier de `apps/mobile-android/` créé, modifié ou supprimé.
- Aucun changement de branche, aucun commit, aucun déploiement.
- Seul ce dossier `docs/product/mobile-ux-phase-2/` est créé.
