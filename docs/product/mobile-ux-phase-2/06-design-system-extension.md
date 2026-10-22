# Axe D — Extension de PoroTheme

Référence : `app/src/main/java/com/poro/app/ui/theme/Theme.kt`, `ui/common/*`, `ui/auth/AuthForm.kt`,
`ui/auth/ErrorMessageText.kt`, `res/values/themes.xml`. Aucune bibliothèque UI supplémentaire n'est
proposée : Material 3 (BOM Compose 2025.06.00) et `material-icons-core` suffisent.

Légende : **R** = existant à réutiliser, **A** = amélioration nécessaire, **N** = nouveau composant justifié.

> **DS1 tranchée le 2026-10-22 : piste A, texte foncé sur l'orange.** Appliquée ainsi dans
> `Theme.kt` : boutons pleins en orange de marque `#FF6A00` avec texte `#1A0A00` (6,73:1), via
> `primaryContainer` et `brandButtonColors()` ; textes, liens, icônes et contours orange en
> `primary` = `#A84100` (5,09:1 au pire, sur `surfaceVariant`), car l'orange de marque reste
> trop pâle pour du texte sur fond clair (2,79:1) même avec la piste A. Gris `surfaceVariant`
> réchauffés, fenêtre clair/sombre sans flash au démarrage. `ThemeContrastTest` vérifie chaque
> paire texte/fond des deux thèmes (≥ 4,5:1).

## 1. Inventaire de l'existant

### 1.1 Couleurs (`Theme.kt`, marquées « provisoires » dans le code)

| Rôle M3 | Clair | Sombre |
|---|---|---|
| `primary` | `#FF6A00` (PoroOrange) | `#FF8A3D` |
| `onPrimary` | `#FFFFFF` | `#000000` |
| `secondary` | `#1B7F5A` | `#4CC497` |
| `background` | `#FFFBF8` | `#121212` |
| `surface` | `#FFFFFF` | `#1E1E1E` |
| Autres rôles (`error`, `surfaceVariant`, `outline`, conteneurs…) | défauts Material 3 | défauts Material 3 |

Thème système : clair/sombre suit `isSystemInDarkTheme()`. Pas de couleurs dynamiques (Android 12+), ce qui est cohérent avec une identité de marque.

**Points à vérifier (A)** :
- Contrastes mesurés (formule WCAG 2.x) :

  | Paire | Ratio | AA texte normal (4,5:1) |
  |---|---|---|
  | `#FFFFFF` sur `#FF6A00` (bouton plein, clair) | 2,87:1 | ❌ |
  | `#FF6A00` sur `#FFFBF8` (bouton texte, lien, clair) | 2,79:1 | ❌ |
  | `#000000` sur `#FF8A3D` (bouton plein, sombre) | 8,95:1 | ✅ |
  | `#FFFFFF` sur `#1B7F5A` (secondaire, clair) | 4,96:1 | ✅ |
  | `#1A0A00` sur `#FF6A00` (piste A) | 6,73:1 | ✅ |
  | `#FFFFFF` sur `#C24E00` (piste B) | 4,79:1 | ✅ |

  En thème clair, **les boutons pleins (`onPrimary` sur `primary`) et tous les textes en `primary`
  (`TextButton`, liens) sont sous le seuil AA**. Pistes : A — texte foncé sur l'orange de marque ;
  B — `primary` plus soutenu (`#C24E00`) pour les fonds et les textes, l'orange vif restant en
  accent. Choix de marque à valider (décision DS1).
- `primaryContainer`, `secondaryContainer`, `tertiary`, `error*`, `surfaceVariant`, `outline` non définis : générés par défaut et sans lien avec la marque → définir une palette complète (rôles M3) à partir de l'orange et du vert.
- `res/values/themes.xml` hérite de `Theme.Material.Light.NoActionBar` : éclair blanc au démarrage en mode sombre → thème de fenêtre `DayNight` avec `windowBackground` = `background`.

### 1.2 Typographie
Aucune `Typography` personnalisée : échelle Material 3 par défaut (police système). Styles utilisés : `headlineMedium`, `headlineSmall`, `titleLarge/Medium/Small`, `bodyMedium/Small`.
**A** : définir `PoroTypography` (police système conservée, graisses des titres affirmées) ; pas de police téléchargée sans décision (poids de l'APK, réseaux lents).

### 1.3 Espacements et dimensions (relevés dans le code)

| Valeur | Usage observé | Occurrences |
|---|---|---|
| 4 dp | espacement serré | 6 |
| 8 dp | espacement entre éléments liés | 16 |
| 12 dp | espacement de formulaire | 13 |
| 16 dp | marge d'écran secondaire, espacement de section | 33 |
| 24 dp | marge des formulaires | 19 |
| 48 dp | hauteur minimale des cibles tactiles | 23 |

**A** : objet `PoroSpacing` (xs 4, s 8, m 12, l 16, xl 24, xxl 32) pour remplacer les valeurs en dur au fil des lots, sans réécriture massive.

### 1.4 Formes, élévations, surfaces
Une seule forme explicite (`RoundedCornerShape(8.dp)`) ; le reste suit `Shapes` par défaut. **A** : `PoroShapes` (small 8, medium 12, large 16) ; élévations Material 3 tonales, sans ombres personnalisées.

### 1.5 Composants

| Composant | Fichier | Statut | Remarque |
|---|---|---|---|
| `SubmitButton` (chargement, compte à rebours 429) | `ui/auth/AuthForm.kt` | R | À déplacer dans `ui/common` (utilisé hors auth) |
| `AuthForm`, `AuthLink` | `ui/auth/AuthForm.kt` | R | |
| `ErrorMessageText` (+ référence de requête) | `ui/auth/ErrorMessageText.kt` | R | À déplacer dans `ui/common` |
| `LoadableContent` (chargement / erreur + Réessayer) | `ui/common/Loadable.kt` | R | Base de tous les écrans de lecture |
| `Avatar` (initiale de repli) | `ui/common/Components.kt` | R | |
| `BackTopBar` | `ui/common/Components.kt` | R | |
| `QuantityStepper` | `ui/shop/ProductScreen.kt` | R → A | Boutons « − / + » textuels : remplacer par des icônes avec `contentDescription` |
| `ProductCard` | `ui/shop/ShopScreen.kt` | R → A | Ajouter l'état « rupture » |
| `VideoActions`, `CommentsSheet`, `ShareSheet` | `ui/social/` | R | Point d'entrée du signalement |
| Feuille de connexion invité | — | N (Lot 4) | Unique, réutilisée par toutes les actions protégées |
| Feuille de signalement | — | N (Lot 2) | Voir `04` |
| État vide illustré (`EmptyState`) | — | N | Aujourd'hui des `Text` isolés, inégaux d'un écran à l'autre |
| Bandeau d'information (`InfoBanner`) | — | N | Textes « À valider », « Total estimé », session invité |
| Badge de statut de commande (`OrderStatusChip`) | — | N | Couleur + libellé, jamais la couleur seule |

### 1.6 États interactifs
Désactivé, sélectionné, pressé : comportements Material 3 par défaut (`FilterChip`, `Button`, `ListItem.clickable`). **R**. Règle : tout état transmis par la couleur l'est aussi par le texte ou l'icône.

### 1.7 Navigation et icônes
`NavigationBar` à 5 onglets (Accueil, Rechercher, Publier, Boutique, Profil), icônes `Icons.Filled` de `material-icons-core`, libellés toujours visibles. **R**. Icônes manquantes dans `core` (signalement, drapeau) : utiliser `Icons.Filled.Warning` / `Info` ou un `ImageVector` local dans `PoroIcons` (déjà le cas pour `Comment`), sans ajouter `material-icons-extended` (≈ +10 Mo).

## 2. Proposition de jetons (à valider, DS1)

```kotlin
// ui/theme/Theme.kt — proposition, non appliquée
object PoroSpacing { val xs = 4.dp; val s = 8.dp; val m = 12.dp; val l = 16.dp; val xl = 24.dp; val xxl = 32.dp }
val PoroShapes = Shapes(small = RoundedCornerShape(8.dp), medium = RoundedCornerShape(12.dp), large = RoundedCornerShape(16.dp))
// Couleurs : palette M3 complète (primary, onPrimary, primaryContainer, …) générée depuis #FF6A00 et #1B7F5A,
// vérifiée au contraste AA pour chaque paire on*/fond.
```

## 3. Critères d'acceptation du design system

1. Toutes les paires texte/fond utilisées atteignent 4,5:1 (texte normal) ou 3:1 (grand texte, icônes).
2. Aucun écran ne redéfinit une couleur en dur hors `Theme.kt`.
3. Le démarrage en mode sombre n'affiche pas de flash blanc.
4. À 200 % de taille de police, aucun libellé de bouton ni d'onglet n'est tronqué au point d'être illisible.
5. Chaque composant N dispose d'un `@Preview` clair et sombre.
