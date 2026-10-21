# Poro — Accessibilité et adaptation aux écrans

Complète [l’audit a11y](../mobile-ux-audit/09-accessibility-and-mobile.md). Ici : règles de **design** à respecter à l’implémentation.

minSdk 24 · target 37 · `supportsRtl=true` · `enableEdgeToEdge` · `adjustResize`.

## 1. Contraste

| Paire | Cible WCAG AA | Comment |
|---|---|---|
| Texte `onSurface` / `background` | ≥ 4,5:1 | Tokens complets |
| Texte `onPrimary` / `primary` | ≥ 4,5:1 | Blanc sur `#FF6A00` OK ; noir sur `#FF8A3D` sombre : vérifier |
| Initiale avatar / `primaryContainer` | ≥ 4,5:1 | `#3D1600` sur `#FFE4D1` |
| Overlay ink / scrim 72 % | ≥ 4,5:1 | **Revue appareil** vidéo blanche |
| Tabs feed unselected 70 % blanc | ≥ 3:1 large | Si échec, passer à 80 % |
| Error / fond | ≥ 4,5:1 | `#B3261E` |
| Prix | ≥ 4,5:1 | Pas de gris clair |

Couleur ≠ seul vecteur : statut commande = texte ; like = icône remplie + teinte ; unread = container + (option) point.

## 2. Touch et focus

- Minimum **48×48 dp** sur tout contrôle (OTP cells 44 → 48, overlay avatar 36 → 48, stepper, send commentaire, ⋮).
- Espacement actions vidéo 12 dp (déjà).
- Ordre focus : top bar / tabs → contenu → nav / composer.
- Sheets : focus piégé (M3).
- Chip Créateur : **non focusable**.

## 3. TalkBack / sémantique

| Contrôle | Attendu |
|---|---|
| Titres écran | `heading()` |
| Erreurs | `LiveRegionMode.Polite` |
| Spinner | « Chargement… » |
| Back | `action_back` |
| Tabs fil | `Role.Tab` + `selected` |
| Tabs search | `TabRow` |
| Like / comment / share | strings avec compteur |
| ⋮ | « Plus d’actions » |
| Signaler motifs | Radio / selected |
| Stepper | « Quantité : N » |
| Cloche | pluriel unread |
| Mot de passe | « masqué / affiché » |
| Switch prefs | `Role.Switch` |
| Images décoratives | `contentDescription = null` |
| Miniature mes vidéos | label titre + statut |
| ProductCard | titre + prix |
| Play overlay | `feed_play_pause` |
| Mute P1 | état annoncé |
| Badge panier | nombre dans le nom |

Ne pas doubler (compteur déjà dans la description → `clearAndSetSemantics` sur le texte, déjà like).

## 4. Taille de police système

Tester Default, Large, **Largest**.

| Risque | Adaptation |
|---|---|
| Tabs fil débordent | Scroll horizontal, pas de wrap sous la cloche |
| Titre overlay | 2 lignes max, ellipsis |
| Shop hub tuiles | Stack vertical |
| TopAppBar long | ellipsis |
| Checkout champs | 1 colonne toujours |
| NavigationBar | labels M3, 5 items — ne pas passer à 6 |

Ne pas figer `sp` en `dp`. Ne pas mettre `softWrap = false` sur un prix.

## 5. Densités et largeurs

| Largeur utile | Règle |
|---|---|
| 320–360 dp | OTP gap 4 ; shop 1 col tuiles ; grille produit 2 col serrée ou 1 |
| 360–411 | Cible design (la plupart des devices CI/SN/CM/NG) |
| Tablette | **Ne pas** concevoir un layout 2 volets. Centrer contenu max ~600 dp, vidéo toujours plein écran |

Orientation : non verrouillée. Fil paysage = crop ZOOM. **À confirmer** lock portrait plus tard — pas un prérequis design.

## 6. Insets et clavier

- Une seule application de padding Scaffold (corriger le double shop).
- Overlay fil : au-dessus de la nav bar + gesture inset.
- Video isolée : back sous status bar.
- Sheets : `imePadding` (commentaires, report).
- Auth : scroll + CTA au-dessus de l’IME.

Status bar : icônes claires sur le fil, adaptées au thème ailleurs. XML Light-only à abandonner.

## 7. RTL

`AutoMirrored` sur back. Colonne actions vidéo : rester à **droite** (convention pouce, même en RTL arabe futur). L’app est FR seulement : RTL peu probable en v1, mais ne pas hardcoder left/right sur le back.

## 8. Motion et photosensibilité

Pas de flash, pas de strobe like. Respecter `animatorDurationScale = 0`. Pager = geste utilisateur. Boucle vidéo : pas d’animation UI supplémentaire.

## 9. Localisation inclusive

- UI FR.
- Compteurs et dates `Locale.FRANCE`.
- Devises selon boutique.
- « Abonné·e » conservé.
- Pas de wolof / haoussa / pidgin inventés.
- SMS +234 EN : hors UI.

## 10. Permissions

Aucune nouvelle permission pour ce redesign. Picker photo système. Pas de CAMERA, LOCATION, MICRO. `POST_NOTIFICATIONS` seulement avec Firebase (P1).

## 11. Clavier physique / TV

Hors cible. Focus D-pad non spécifié.

## 12. Tests a11y recommandés

1. TalkBack : auth OTP → Home → like → comments → shop handle → checkout.
2. Contrast Checker sur overlay (captures vidéo blanche / noire).
3. Font scale 1.3 et 2.0 : feed tabs, PDP, checkout.
4. Largeur 320 dp emulator.
5. Dark + light : avatar, unread, checkout prix.
6. Switch access : 5 tabs + CTA auth.

## 13. AC accessibilité

1. 0 contrôle actionnable < 48 dp sur les parcours P0.
2. 0 chip no-op focusable.
3. Overlay AA sur 2 extraits vidéo extrêmes.
4. Toutes les nouvelles actions ont une string FR.
5. Legal headings corrects.
6. Report motifs sélectionnables sans couleur seule.
