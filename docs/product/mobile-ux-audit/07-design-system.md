# Design system mobile

**Fait** : un thème Compose minimal existe. **Recommandation** : le compléter dans `ui/theme/` sans casser `PoroTheme`, sans second système, en Material 3.

Source actuelle : `apps/mobile-android/app/src/main/java/com/poro/app/ui/theme/Theme.kt`. Commentaire du fichier : *« Palette provisoire, à remplacer par la charte graphique de Poro. »* Charte de marque hors dépôt : **À confirmer**.

## A. Principes visuels (critères, pas slogans)

| Principe | Critère mesurable déjà ou à viser |
|---|---|
| Priorité au contenu | Fil : vidéo plein écran, chrome réduit (onglets + overlay). Ne pas ajouter de cartes autour du player. |
| Lisibilité | Overlay sur dégradé sombre (`FeedScreen`) ; texte métier sur `onSurface` ; erreurs `colorScheme.error`. |
| Densité | Espacements 8 / 12 / 16 / 24 dp observés. Zones tactiles ≥ 48 dp sur CTA auth, onglets, stepper. |
| Feedback | `SubmitButton` + spinner ; `LoadableContent` retry ; snackbar session / succès checkout ; `liveRegion` erreurs. |
| Cohérence | Un `BackTopBar`, un `Avatar`, un mapping `error.code` → string FR. |
| Dynamique | Compteurs `Formats.count` ; dates `Formats.relative` / `dd/MM/yyyy` ; prix `Formats.money`. |
| Adaptation | `enableEdgeToEdge` ; `adjustResize` clavier ; `supportsRtl=true` ; dark = système. |

## B. Couleurs

### Constatées

| Token Material | Clair | Sombre | Usage actuel |
|---|---|---|---|
| `primary` | `#FF6A00` | `#FF8A3D` | CTA, avatar fallback (`primaryContainer` défaut M3) |
| `onPrimary` | `#FFFFFF` | `#000000` | Texte sur CTA |
| `secondary` | `#1B7F5A` | `#4CC497` | Accents (bannière email : `secondaryContainer`) |
| `background` | `#FFFBF8` | `#121212` | Écrans hors fil |
| `surface` | `#FFFFFF` | `#1E1E1E` | Champs, barres |
| Fil | n/a | n/a | `Color.Black` / `Color.White` **hors** ColorScheme |
| Cœur like | n/a | n/a | `#FF3B5C` en dur dans les actions sociales |

Slots non définis (`error`, `outline`, `tertiary`, containers) : **défauts Material 3**. Risque de contraste non maîtrisé sur `primaryContainer` (avatar).

### Recommandation (à valider charte)

Conserver l’orange `#FF6A00` comme primaire s’il est la marque. Compléter **dans le même fichier** :

| Rôle | Proposition de travail | Contraste cible |
|---|---|---|
| `error` | `#B3261E` / `#F2B8B5` sombre | Texte d’erreur ≥ 4,5:1 |
| `tertiary` | réserver au badge Créateur, pas un 2e CTA | — |
| Overlay scrim | noir 40–70 % déjà via `Brush` | Texte blanc ≥ 4,5:1 sur la zone de titre |
| Success | s’appuyer sur `secondary` vert, pas un 4e brand | Confirmation commande, vidéo prête |
| Warning | défaut M3 `tertiary` ou ambre **À confirmer** | Boutique suspendue |

Ne pas figer une palette marketing ici : le fichier dit déjà « provisoire ».

## C. Typographie

**Constat** : aucune `Type.kt`. `MaterialTheme.typography` par défaut (Roboto / police plateforme).

Usages observés :

| Rôle | Style appelé |
|---|---|
| Titre écran | `headlineSmall` (profil), `TopAppBar` title |
| Titre vidéo overlay | `titleMedium` + `FontWeight.Bold` + `maxLines` + ellipsis |
| Corps | `bodyLarge` / `bodyMedium` |
| Meta / @username | `bodyLarge` + `onSurfaceVariant` |
| Erreur | `bodyMedium` + `error` |
| Compteur | texte compact `Formats.count` |

**Recommandation** : garder la scale M3 (pas une police custom tant que la charte n’est pas livrée). Règles de troncature : titre vidéo 2 lignes max ; nom produit 2 lignes ; handle 1 ligne. Bio profil : pas de troncature (scroll).

SMS Nigeria en anglais, UI Android **français seulement** (`values/strings.xml`). Pas de `values-en`.

## D. Espacements et dimensions

Échelle observée / à officialiser en tokens `PoroSpace` :

| Token | Valeur | Usage |
|---|---|---|
| space-4 | 4 dp | interne chips |
| space-8 | 8 dp | padding onglets feed |
| space-12 | 12 dp | colonnes profil |
| space-16 | 16 dp | listes, cartes |
| space-24 | 24 dp | marges d’écran auth/profil |
| touch | 48 dp | min hauteur boutons / tabs |
| field | défaut `OutlinedTextField` M3 | formulaires |
| radius | défaut M3 (cards ~12 dp) | ne pas mixer des rayons ad hoc |
| avatar-s | 48 dp | listes, overlay |
| avatar-l | 96 dp | profil |
| nav bar | hauteur M3 `NavigationBar` | 5 items max |
| icône | 24 dp Material | |

Feed : pas de padding bas supplémentaire sous l’overlay au-delà de la nav bar (la nav est masquée ? Non : le fil **est** un onglet, la barre reste visible). Contrainte : la colonne d’actions doit rester au-dessus de la `NavigationBar`.

## E. Composants

### Existants — variantes et états

| Composant | Variantes | États | Accessibilité | Contrainte |
|---|---|---|---|---|
| Bouton primaire | `Button` / `SubmitButton` | enabled, loading, retry-after | label string | Un seul CTA principal par écran auth |
| Bouton secondaire | `OutlinedButton`, `TextButton` | — | 48 dp `AuthLink` | |
| Champ | `OutlinedTextField` | error, autofill | labels `strings.xml` | Mot de passe show/hide en TextButton |
| OTP | `CodeField` | filled, error | description 6 chiffres, liveRegion renvoi | |
| Avatar | photo / initiale / « ? » | — | décoratif `contentDescription=null` | Ne pas le rendre bouton sans label |
| Loadable | loading / failed / loaded | retry | spinner nommé « Chargement… » | Pas d’empty ici (géré par l’écran) |
| Empty | `Text` centré | — | — | Pas d’illustration |
| Snackbar | `SnackbarHost` activité | — | — | Succès courts, pas d’erreurs bloquantes |
| NavigationBar | 5 tabs | selected | label ; icône `contentDescription=null` | OK si label lu |
| ModalBottomSheet | commentaires, share | — | titre `comments_title` | |
| AlertDialog | delete vidéo, delete produit, cancel order | — | titre + corps | Action destructive claire |
| Switch | préférences | on/off | Role.Switch | `orders` hint obligatoire |
| Badge | unread cloche | 0 = pas de badge | pluriel unread | |
| ProductCard | image + titre + prix from | rupture via fiche | — | Prix toujours via `Formats.money` |
| QuantityStepper | − / + | 1–99, disabled bornes | « Quantité : N » | |
| VideoSurface | playing / paused (icône play) | pas d’error UI HLS | play/pause onClickLabel | Boucle ; crop ZOOM |

### Manquants à spécifier (même famille M3)

| Composant | Pourquoi | Règle |
|---|---|---|
| `OverflowMenu` (3 points) | Report, signaler, partager secondaire | 48 dp ; ne pas cacher une autorisation |
| `EmptyState` | Image optionnelle + titre + CTA | Réutiliser les copies déjà traduites |
| `StatusChip` | draft / active / archived / order status | Couleur ≠ seul vecteur (texte déjà là) |
| `PriceText` | Centraliser devise | Interdit d’afficher un float XOF |
| `MuteButton` | P1 lecture | État annoncé TalkBack |
| `ReportSheet` | UI-MOD-001 | Motifs = enum API |

Pas de composant Web, pas de carte paiement Wave.

## F. Motion

Pas de spec d’animation dédiée. Pager vertical = geste natif. Réduire les animations : suivre `android:animatorDurationScale` système (par défaut Compose). **Recommandation** : ne pas ajouter de Lottie sur le fil (coût GPU / données).

## G. Implémentation

- Fichiers cibles : `Theme.kt` + éventuellement `Color.kt` / `Type.kt` **dans le module existant**.
- Interdit : module Compose parallèle, thème XML concurrent (le XML `Theme.Poro` reste NoActionBar fenêtre seulement).
- Toutes les copies UI : `strings.xml` (FR), jamais `error.message` anglais.

## H. Incohérences à corriger (recommandations)

1. Like `#FF3B5C` vs primary orange : choisir (cœur rouge = convention sociale, acceptable si documenté).
2. Chip Créateur cliquable sans action : `AssistChip` → `SuggestionChip` non cliquable ou badge texte.
3. `AccountRules` display_name/bio plus larges que le serveur : aligner l’UI sur 50 / 160.
4. Contraste overlay : vérifier titre + compteurs sur vidéo claire (scrim déjà présent, **À confirmer** par revue visuelle appareil).
