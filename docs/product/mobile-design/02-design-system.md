# Poro — Design system (extension de PoroTheme)

Référence actuelle : `apps/mobile-android/app/src/main/java/com/poro/app/ui/theme/Theme.kt`.

**Règle** : on complète `ui/theme/` et les composables déjà dans `ui/common/` et `ui/auth/`. On n’ajoute aucune bibliothèque UI. On n’introduit pas un module Compose parallèle.

Implémentation **interdite** tant que l’écriture dans `apps/mobile-android` n’est pas autorisée.

## 1. Fichiers cibles (futurs)

| Fichier | Rôle | Statut actuel |
|---|---|---|
| `ui/theme/Theme.kt` | `PoroTheme` + ColorScheme complet | 6 tokens seulement |
| `ui/theme/Color.kt` | Constantes nommées (`PoroOrange`, `PoroLike`, scrims) | Absent (inline) |
| `ui/theme/Dimens.kt` | `PoroSpace`, touch, avatars | Absent |
| `ui/theme/Type.kt` | **Ne pas créer** tant que Q-10 | M3 défaut OK |
| `res/values/themes.xml` | Fenêtre NoActionBar ; **suivre le système** pour status bar | Toujours Light |
| `ui/common/Components.kt` | Avatar, BackTopBar | Existant |
| `ui/common/Loadable.kt` | Loading / error / content | Existant |
| `ui/common/EmptyState.kt` | Titre + corps + CTA optionnel | **Nouveau justifié** |
| `ui/common/StatusChip.kt` | Statut commande / boutique / vidéo | **Nouveau justifié** |
| `ui/common/PriceText.kt` | Wrapper `Formats.money` | **Nouveau léger** |
| `ui/common/OverflowMenu.kt` | ⋮ 48 dp | **Nouveau justifié** (report) |
| `ui/auth/AuthForm.kt` | SubmitButton, AuthLink | Existant — réutiliser hors auth |

Interdit : Lottie, Compose BOM parallèle, thème XML concurrent produit, WebView UI.

## 2. Couleurs sémantiques

Voir [01-visual-direction.md](01-visual-direction.md) §4 pour les hex. Mapping Material 3 à appliquer dans `lightColorScheme` / `darkColorScheme` :

| Slot M3 | À définir | Usage Poro |
|---|---|---|
| `primary` / `onPrimary` | Déjà | CTA, Tab selected, cursor OTP |
| `primaryContainer` / `onPrimaryContainer` | **Oui** | Avatar sans photo |
| `secondary` / `onSecondary` | Déjà / à compléter | Succès, « code renvoyé » |
| `secondaryContainer` / `onSecondaryContainer` | **Oui** | Non-lus, bannière email |
| `tertiary` | Badge Créateur seulement | Texte/chip non cliquable, **pas un CTA** |
| `error` / `onError` / `errorContainer` | **Oui** | `ErrorMessageText`, bouton supprimer |
| `background` / `onBackground` | Déjà / à compléter | Écrans |
| `surface` / `onSurface` / `onSurfaceVariant` | Partiel | Champs, méta |
| `outline` / `outlineVariant` | **Oui** | Bordures, séparateurs |
| `surfaceVariant` | **Oui** | Fonds de liste |

Tokens hors scheme (constantes `Color.kt`) :

```
PoroLike          = 0xFFFF3B5C
PoroScrimTop      = Black.copy(0.40f)
PoroScrimBottomStart = Transparent
PoroScrimBottomEnd   = Black.copy(0.72f)
PoroOverlayInk    = White
PoroOverlayMuted  = White.copy(0.85f)
```

### Règles d’emploi

- Un écran a **au plus un** bouton `primary`.
- Vert = confirmation déjà arrivée (OTP renvoyé, commande passée, vidéo `ready`), jamais « cliquez ici ».
- Rose like = état `likedByMe` uniquement, pas un branding de page.
- Ne jamais coder un statut commande par la seule couleur : le texte `order_status_*` reste obligatoire.

## 3. Typographie

Scale Material 3, police plateforme. Usages figés :

| Rôle | Style | Contraintes |
|---|---|---|
| Titre d’écran | `headlineSmall` ou titre `TopAppBar` | 1 ligne, ellipsis |
| Titre auth | `headlineMedium` (`AuthForm`) | Conserver |
| Titre overlay vidéo | `titleMedium` + Bold | 2 lignes max |
| Nom auteur overlay | `titleSmall` | 1 ligne |
| Corps | `bodyLarge` / `bodyMedium` | Erreurs = `bodyMedium` + `error` |
| Méta / @handle | `bodyMedium` + `onSurfaceVariant` | 1 ligne |
| Prix | `titleLarge` + Bold | Toujours `Formats.money` |
| Prix « à partir de » | `titleMedium` | Carte produit |
| Compteur social | `labelMedium` | Overlay blanc |
| Hint / légal | `bodySmall` | Pas de clauses inventées |
| Chip statut | `labelLarge` | Texte + couleur |

Troncatures : titre vidéo 2 ; nom produit 2 ; handle 1 ; bio profil **sans** troncature (scroll). Hashtags overlay 1 ligne.

## 4. Espacements, touch, rayons

`PoroSpace` (dp) :

| Token | Valeur | Usage |
|---|---|---|
| `xxs` | 4 | interne chip, badge |
| `xs` | 8 | gap onglets feed, icône + texte |
| `sm` | 12 | colonnes profil, gap actions vidéo |
| `md` | 16 | padding listes, cartes, overlay bas |
| `lg` | 24 | padding écrans auth / shop / settings |
| `xl` | 32 | empty states |

| Dimension | Valeur | Règle |
|---|---|---|
| Touch min | 48 dp | Tout contrôle actionnable |
| Champ | hauteur M3 Outlined | Pleine largeur formulaires |
| OTP cell | 48×56 dp (**aujourd’hui 44×56**) | Passer à 48 de large pour touch |
| Avatar S | 40 dp | Listes, commentaires (aujourd’hui 40 notif) |
| Avatar M | 48 dp | Overlay (aujourd’hui 36 — trop petit) |
| Avatar L | 96 dp | Profils |
| Nav bar | M3 | 5 items, labels visibles |
| Icône | 24 dp barre ; 32 dp actions vidéo | |
| Miniature produit | 72 dp liste ; 1:1 grille | **clip 8 dp** |
| Miniature vidéo | 9:16 | clip 8 dp |
| Radius card | 12 dp | |
| Radius sheet | 28 dp haut | |
| Stroke | 1 dp `outline` | Cartes commerce |

Corriger le **double Scaffold padding** observé sur shop/cart/product : une seule source d’insets.

## 5. Composants

### 5.1 Boutons

| Variante | Composable | Quand | États |
|---|---|---|---|
| Primaire | `Button` / `SubmitButton` | Une par écran | enabled, loading (spinner + disable), retry-after |
| Secondaire | `OutlinedButton` | Alternative (panier, modifier) | enabled / disabled |
| Tertiaire | `TextButton` / `AuthLink` | Navigation faible, « Plus tard » | min 48 dp |
| Destructif | `Button` `error` **ou** `TextButton` error dans dialogue | Supprimer compte, produit, commande | Dialogue d’abord |

Pendant une requête : le CTA primaire passe en `loading` et ignore les taps. Pas de second spinner de page si le bouton suffit.

### 5.2 Champs

Réutiliser `OutlinedTextField`, `EmailField`, `PasswordField`, `CodeField`.

| État | Visuel |
|---|---|
| Repos | outline |
| Focus | primary |
| Erreur | error + `ErrorMessageText` liveRegion |
| Disabled | onSurface 38 % |
| Afficher mot de passe | `Role.ToggleButton` + « masqué / affiché » |

Clavier : OTP number, téléphone phone, email email, handle ascii, quantité number.

### 5.3 Cartes et listes

| Type | Structure | Clip |
|---|---|---|
| `ProductCard` | image 1:1, titre 2 lignes, `PriceText` « À partir de » | image 8 dp, carte 12 dp |
| Ligne commande | statut (`StatusChip`) · date · total | — |
| Ligne notification | Avatar 40, titre, relatif, unread container | — |
| Ligne session | appareil, date, révoquer | — |
| Ligne panier | thumb 72 clip, titre, variante, stepper, retirer | **clip manquant aujourd’hui** |

Pas d’élévation marketing. Séparateurs `outlineVariant` si besoin, pas les deux (carte + divider).

### 5.4 Avatar

États : photo Coil / initiale sur `primaryContainer` / « ? » si rien.

- Décoratif dans une ligne déjà labellisée : `contentDescription = null`.
- Bouton (ouvrir profil) : label = nom affiché.
- Tailles : S 40, M 48, L 96. Overlay passe de 36 → 48.

### 5.5 Navigation

`NavigationBar` M3, 5 items, labels FR existants. Item selected = `primary`. Icônes filled. Boutique : proposer `Store` (changement d’icône seulement).

`BackTopBar` inchangé. Sheets : poignée M3 + titre `heading()`.

### 5.6 Dialogues et feuilles

| Usage | Composant | Titre |
|---|---|---|
| Commentaires | `ModalBottomSheet` 85 % | `comments_title` |
| Partage | `ModalBottomSheet` | Partager |
| Signalement | `ModalBottomSheet` **nouveau** | Signaler |
| Delete vidéo / produit / commande | `AlertDialog` | Action + conséquence |
| Delete compte | **écran dédié** déjà là | Ne pas réduire à un dialogue |

Dismiss = aucun POST. Focus piégé le temps de l’ouverture (M3).

### 5.7 Chargement

| Contexte | Pattern |
|---|---|
| Premier chargement écran | `LoadableContent` spinner centré « Chargement… » |
| CTA | `SubmitButton` spinner |
| Page suivante liste | petit spinner bas / `action_load_more` |
| Fil | spinner blanc sur noir |
| Player HLS | P1 : indicateur discret centre ; pas un overlay opaque |

Pas de skeleton sur le fil (une miniature Coil suffit). Skeleton optionnel P2 sur grille boutique seulement.

### 5.8 Succès et erreurs

| Type | Où |
|---|---|
| Succès court | Snackbar (`checkout_placed`, `product_added`, report envoyé) |
| Succès d’écran | État Finished upload, récap commandes |
| Erreur bloquante | `ErrorMessageText` + Réessayer |
| Erreur champ | supportingText |
| 429 | countdown sur le bouton, comme l’auth |
| 401 session | `goToSignIn()`, pas un bandeau |

Jamais `error.message` anglais. Jamais un succès animé à la place d’une erreur métier.

### 5.9 Empty et indisponible

`EmptyState(title, body?, actionLabel?, onAction?)` — texte d’abord, illustration **optionnelle et absente en P0**.

Copies déjà dans `strings.xml` à réutiliser : `feed_empty`, `feed_following_empty`, `cart_empty`, `shop_no_products`, `comments_empty`, `orders_empty`.

Contenu retiré : `video_removed` / 404 — EmptyState sans CTA inventé (pas « explorer le catalogue »).

Document légal non validé : titre + « Document non encore validé. » — **pas** de faux CGU.

### 5.10 StatusChip

| Domaine | Valeurs | Couleur d’appoint |
|---|---|---|
| Commande | pending, confirmed, shipped, completed, cancelled, awaiting_payment, paid | neutre / vert confirmed / ambre shipped / error cancelled |
| Boutique | active, closed, suspended | vert / neutre / error |
| Vidéo owner | processing, ready, failed, removed | neutre / vert / error / neutre |

`paid` et `awaiting_payment` : n’afficher que si l’API les envoie. COD → `confirmed` après confirm.

### 5.11 PriceText

Un seul point d’affichage monétaire : `Formats.money`. Interdit d’interpoler un float. XOF/XAF sans décimales ; NGN 2 max.

## 6. Clair / sombre

| Surface | Règle |
|---|---|
| Auth, shop, profil, settings, search, legal | ColorScheme système |
| Fil et `VideoPage` | Chrome sombre **toujours** |
| Sheets | surface thème |
| Status bar / nav system | `enableEdgeToEdge` + icônes adaptées au fond **réel** (fil = icônes claires) |

XML `Theme.Poro` : cesser d’imposer Light. Suivre `uiMode` pour éviter le flash blanc au-dessus du fil.

Pas de toggle in-app (non demandé, pas d’API préférence).

## 7. Navigation bar — 5 destinations

| Tab | Label | Icône proposée | Route |
|---|---|---|---|
| Accueil | Accueil | Home | `HomeRoute` |
| Rechercher | Rechercher | Search | `SearchRoute` |
| Publier | Publier | AddCircle | `UploadRoute` |
| Boutique | Boutique | Store (aujourd’hui Cart) | `ShopHomeRoute` |
| Profil | Profil | Person | `ProfileRoute` |

Visible seulement sur ces cinq racines. Enfant = `BackTopBar`, barre masquée (comportement actuel conservé).

Pas d’onglet Messages, Analytics, Notifications.

## 8. Accessibilité des composants (résumé)

Détail dans [08-accessibility-responsive.md](08-accessibility-responsive.md).

Minimum par composant nouveau :

- Touch 48 dp.
- Nom accessible FR dans `strings.xml`.
- `heading()` sur les titres d’écran et de sheet.
- Erreurs `LiveRegionMode.Polite`.
- Couleur jamais seul vecteur.

## 9. Ce qu’on ne crée pas

| Tentation | Pourquoi non |
|---|---|
| `PoroButton` wrapper de tout Button | `SubmitButton` + M3 suffisent |
| Palette « African sunset » 12 couleurs | Sémantique déjà saturée |
| Police display custom | Q-10 absent ; APK + perf |
| Illustrations empty Lottie | Poids, motion |
| Carte produit style shop Shopify | Pas le métier |
| Dark-only brand | Boutique et auth doivent rester clairs au soleil |

## 10. Critères d’acceptation du système

1. Aucun `primaryContainer` / `secondaryContainer` lavande M3 n’apparaît (avatar, non-lus).
2. Tous les CTA primaires sont `#FF6A00` / `#FF8A3D`.
3. Tous les prix passent par `PriceText` / `Formats.money`.
4. Toute image produit/vidéo en liste est clipée.
5. Tout bouton a une hauteur min 48 dp.
6. Aucune nouvelle dépendance Gradle UI.
7. `PoroTheme` reste le seul `MaterialTheme` produit.
