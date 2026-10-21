# Poro — Plan d’implémentation design

Aucun lot n’est commencé dans `apps/mobile-android`. **Autorisation explicite distincte** requise avant tout Kotlin.

Ce plan **complète** [phase-2/09](../mobile-ux-phase-2/09-implementation-plan.md) : il y ajoute les lots **visuels** sans les fusionner. Les lots fonctionnels phase 2 (somme panier, report, légal, invité) restent la source pour le métier. Ici : tokens, chrome, hub boutique, états.

Ne pas réécrire tous les écrans d’un coup.

## 1. Principes d’exécution

1. Un lot = une PR mentale / un commit thématique, après OK Android.
2. Social et auth **non régressés** à chaque lot (`NR-*` phase 2).
3. Pas de nouvelle lib UI.
4. Pas de changement OpenAPI pour un choix de couleur.
5. Strings métier COD **intouchables** sauf typo.
6. Tests unitaires ViewModel existants étendus ; tests Compose ciblés sur tokens/composants nouveaux.

## 2. Lots design

### Lot D0 — Tokens PoroTheme (fondation)

| | |
|---|---|
| But | Fin de la fuite lavande M3 ; scrims et like nommés |
| Prérequis | Autorisation Android |
| Travaux | ColorScheme complet ; `Color.kt` (`PoroLike`, scrims) ; `Dimens.kt` ; XML thème suit `uiMode` |
| Fichiers | `ui/theme/Theme.kt`, `Color.kt`, `Dimens.kt`, `res/values/themes.xml`, éventuellement `values-night/themes.xml` |
| Interdit | Police custom, module thème, toucher les ViewModels |
| Dépendances | Aucune |
| Tests | Capture/theme preview light+dark ; avatar contrast |
| Terminé | Avatars et unread ne sont plus violets ; fil status bar icônes claires |

### Lot D1 — Composants partagés

| | |
|---|---|
| But | EmptyState, StatusChip, PriceText, OverflowMenu ; Avatar 40/48/96 ; CodeField 48 |
| Prérequis | D0 |
| Travaux | Nouveaux fichiers `ui/common/` ; brancher PriceText sur cartes/PDP/panier progressivement **ou** wrapper `Formats.money` d’abord sur ProductCard |
| Accroches | `Components.kt`, `Loadable.kt`, `CodeField.kt` |
| Interdit | Lottie, illustrations |
| Tests | Preview Compose ; sémantique OverflowMenu |
| Terminé | Menu ⋮ réutilisable pour D3 / lot 2 report |

Peut **fusionner** avec le lot 2 phase 2 (report) : le menu sert les deux.

### Lot D2 — Chrome du fil

| | |
|---|---|
| But | Lisibilité overlay + tabs + ⋮ |
| Prérequis | D0, D1 (menu) |
| Travaux | Scrim haut ; bas 72 % ; avatar overlay 48 ; underline tab orange ; `Role.Tab` ; play contrast ; accroche report |
| Fichiers | `FeedScreen.kt`, `VideoActions.kt`, `SocialGraph.kt` |
| Interdit | Mute (P1 lot 5 phase 2), double-tap like, overlay produit |
| Tests | NR feed ; TalkBack labels |
| Terminé | AC §15 de [05](05-social-feed-design.md) hors mute |

### Lot D3 — Hub boutique + clip images + hiérarchie prix

| | |
|---|---|
| But | Shop home reconnaissable ; commerce lisible |
| Prérequis | D0 ; **aligné** lot 1 phase 2 (somme panier, hasShop, photos 10, confirm réseau) |
| Travaux | Layout hub ; clip thumbs ; PriceText ; StatusChip commandes ; icône tab `Store` |
| Fichiers | `ShopHomeScreen.kt`, `ShopScreen.kt` ProductCard, `CartScreen.kt`, `OrderScreens.kt`, `PoroNavHost` icône |
| Interdit | Catalogue, Wave, GPS |
| Tests | MX-* phase 2 + revue visuelle hub |
| Terminé | AC §12 de [06](06-marketplace-design.md) |

Si le lot 1 phase 2 n’est pas encore autorisé : faire **seulement** le layout hub + clip (pas la somme panier).

### Lot D4 — Auth, profil, settings

| | |
|---|---|
| But | Auth moins générique ; profil vrai ; settings À propos |
| Prérequis | D0, D1 |
| Travaux | Header auth ; chip créateur non cliquable ; banner `secondaryContainer` ; limites 50/160 ; groupe légal (si lot 3 phase 2) ; versionName |
| Fichiers | `PhoneEntryScreen.kt`, `ProfileScreen.kt`, `EditProfileScreen.kt`, `AccountRules.kt`, `SettingsScreen.kt` |
| Interdit | Promettre que `full_name` devient le profil |
| Tests | Auth inchangé ; a11y chip |
| Terminé | Chip morte absente ; settings exposent légal même indisponible |

### Lot D5 — États vides / erreurs homogènes

| | |
|---|---|
| But | EmptyState branché ; boutons retry contrastés sur fil |
| Prérequis | D1 |
| Travaux | Remplacer `Text` empty par `EmptyState` là où un CTA réel existe (following, mes vidéos) |
| Interdit | Illustrations lourdes |
| Terminé | Listes P0 ont empty cohérent |

### Lot D6 — Lecture P1 (avec phase 2 lot 5)

Mute, HLS error, WorkManager — **pas du design pur**. Le design fournit : icône mute, copy erreur player. Dépend Firebase / WorkManager hors design.

### Lot D7 — Invité (bloqué)

Interdit tant que trending public + quota IP + ADR. Voir phase 2 lot 4.

## 3. Ordre recommandé le jour J

```
Autorisation Android
    → D0 tokens
    → D1 composants
    → en parallèle possible :
         D2 fil (+ lot 2 report)
         D3 shop visuel (+ lot 1 marketplace métier)
         D4 profil/auth (+ lot 3 légal)
    → D5 empty
    → stop invité
    → D6 avec P1 lecture
```

Ne pas ouvrir D2+D3+D4 le même jour sur les mêmes fichiers de nav.

## 4. Dépendances hors design

| ID | Bloque | Owner |
|---|---|---|
| B-AND | Tous les lots D* | Utilisateur (autorisation) |
| B-LEGAL | Contenu opposable légal | Juridique Q-03 |
| B-FEED | D7 | Backend trending |
| B-FCM | Switch push honnête | Mobile + ops |
| Q-10 | Police custom | Charte — on n’attend pas |
| PORO-026 | Vraie découverte boutique | Backend |
| PORO-015 | UI paiement | `services/payment` |

## 5. Risques

| Risque | Mitigation |
|---|---|
| Refactor Theme casse les previews | ColorScheme additif, ne pas renommer `PoroTheme` |
| Double padding shop | Corriger dans D3 seulement |
| Fil plus sombre trop | Tester 60 vs 72 % sur appareil ; token unique |
| Icône Store déroute | Label « Boutique » reste |
| Report + ⋮ sans API client | Livrer ModerationApi dans le même lot que D2 |
| Scope creep « vrai catalogue » | Refuser tant que PORO-026 |

## 6. Ce que ce plan ne fait pas

- Commits, branches, PR, deploy.
- Modification backend.
- Réécriture de `PoroNavHost` hors ajouts Legal / returnTo.
- Remplacement des écrans par des maquettes.

## 7. Définition de terminé d’un lot D

- AC du lot verts.
- Light + dark smoke.
- Aucune route staff.
- Aucun secret.
- Audit phase 1 et specs phase 2 **préservés**.
- `apps/mobile-android` touché seulement après OK de **ce** lot.
