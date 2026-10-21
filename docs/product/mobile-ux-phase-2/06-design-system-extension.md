# Axe D — Extension de PoroTheme

Référence actuelle : `apps/mobile-android/.../ui/theme/Theme.kt`. Commentaire du fichier : palette provisoire. **On étend ce fichier / ce package, on n’en crée pas un second.**

Charte hors dépôt : **À confirmer** (Q-10). En attendant, figer l’existant et combler les trous Material.

## 1. Constat palette

| Token | Clair | Sombre | Usage |
|---|---|---|---|
| `primary` | `#FF6A00` | `#FF8A3D` | CTA |
| `onPrimary` | `#FFFFFF` | `#000000` | Texte CTA |
| `secondary` | `#1B7F5A` | `#4CC497` | Accents, bannière |
| `background` | `#FFFBF8` | `#121212` | Écrans |
| `surface` | `#FFFFFF` | `#1E1E1E` | Champs, barres |
| Fil | `Color.Black` / `White` hors scheme | Overlay vidéo |
| Like | `#FF3B5C` en dur | Cœur social |

Slots `error`, `outline`, `primaryContainer`, etc. : **défauts M3** (non maîtrisés). Dark = `isSystemInDarkTheme()`, pas de toggle in-app.

## 2. Recommandations (classées)

| Item | Type | Détail |
|---|---|---|
| `PoroTheme` | **Réutiliser** | Ne pas remplacer MaterialTheme |
| Définir `error` / `onError` | **Améliorer** | Ex. `#B3261E` / clair ; texte erreurs déjà `colorScheme.error` |
| `primaryContainer` avatar | **Améliorer** | Contraste initiale ≥ 4,5:1 |
| Like `#FF3B5C` | **Réutiliser** | Convention sociale ; documenter comme `PoroLike` dans Theme.kt |
| Scrim overlay | **Améliorer** | Token `Color.Black.copy(alpha=0.55)` unique |
| Typo `Type.kt` custom | **Ne pas ajouter** | Tant que Q-10 n’a pas de police |
| Scale M3 | **Réutiliser** | `headlineSmall`, `titleMedium`, `bodyMedium` déjà en place |
| `PoroSpace` 4/8/12/16/24 | **Nouveau justifié** | Constantes dans `theme/Dimens.kt`, pas une lib |
| Touch 48 dp | **Réutiliser** | Auth, nav, stepper |
| Rayons / élévation | **Réutiliser** | Défauts M3 ; pas de Card custom parallèle |
| `SubmitButton` / `AuthForm` | **Réutiliser** | Report CTA, checkout |
| `LoadableContent` | **Réutiliser** | Vitrine, produit, commande |
| `ErrorMessageText` | **Réutiliser** | Tous les codes |
| `Avatar` | **Réutiliser** | Chip créateur : **améliorer** (non cliquable) |
| `ModalBottomSheet` | **Réutiliser** | Comments, Report, Share |
| `OverflowMenu` ⋮ | **Nouveau justifié** | Report ; 48 dp ; `Icons.Default.MoreVert` |
| `StatusChip` commande | **Améliorer** | Texte déjà là ; ne pas coder le statut par la seule couleur |
| `PriceText` | **Nouveau léger** | Wrapper `Formats.money` pour ne pas afficher de float XOF |
| `EmptyState` illustré | **Pas P0** | Textes existants suffisent |
| Lottie / lib UI | **Interdit** | Coût, dépendance non justifiée |
| Web components | **Interdit** | |

## 3. Composants — états

| Composant | Disabled | Selected | Pressed | Loading |
|---|---|---|---|---|
| `SubmitButton` | `enabled=false` | — | ripple M3 | spinner + disable |
| Champ | — | focus outline | — | — |
| `NavigationBarItem` | — | selected | ripple | — |
| Switch prefs | — | checked | — | — |
| Stepper | bornes 1 / 99 | — | — | ligne `busy` panier |
| Chip créateur | **non interactif** | — | aujourd’hui ripple mort | — |

## 4. Navigation et icônes

Conserver les 5 `Icons.Filled` actuelles. Report : `MoreVert`, pas une 6e tab. Légal : pas d’icône tab. Boutique : `ShoppingCart` déjà sur l’onglet — acceptable.

## 5. Clair / sombre

| Surface | Règle |
|---|---|
| Auth, shop, settings | ColorScheme |
| Fil / player | Toujours chrome sombre (lisibilité vidéo), **indépendant** du thème système |
| Sheets | Surface thème |
| Snackbar | M3 défaut |

Tester les deux thèmes sur checkout (prix) et report.

## 6. Implémentation future (après autorisation)

Fichiers cibles uniquement : `Theme.kt`, éventuellement `Color.kt` / `Dimens.kt` **dans `ui/theme/`**, plus `strings.xml` pour report/légal. Interdit : module Compose nouveau, thème XML concurrent (rester `Theme.Poro` NoActionBar fenêtre).
