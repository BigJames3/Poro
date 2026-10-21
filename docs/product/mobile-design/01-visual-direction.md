# Poro — Direction visuelle

- **Date** : 21 octobre 2026.
- **Nature** : proposition de design. **Aucun fichier de `apps/mobile-android` n’est modifié.**
- **Base** : `Theme.kt`, inventaire UX, phase 2, OpenAPI, écrans Compose réels.
- **Charte marketing hors dépôt** : Q-10 reste ouvert. Cette direction **fige l’existant** et comble les trous, elle ne remplace pas une charte officielle.

## 1. Ce que le produit est vraiment

Poro est une application sociale vidéo pour l’Afrique de l’Ouest et du Centre, avec une marketplace en paiement à la livraison. Le social est le cœur (ADR-0004). La boutique est déjà un onglet principal (décision phase 2 : la garder).

Ce que l’interface doit exprimer en 3 secondes :

| Surface | Promesse | Preuve visuelle |
|---|---|---|
| Fil | « Je regarde des vidéos d’ici » | Vidéo plein écran, chrome sombre, actions à droite, auteur en bas |
| Boutique | « Je peux acheter chez quelqu’un que je connais » | Prix lisible, handle vendeur, paiement à la livraison annoncé |
| Auth | « Je peux entrer sans friction » | OTP d’abord, orange unique, une action principale |
| Profil | « C’est moi, et je contrôle mon compte » | Avatar, @pseudo, actions secondaires clairement secondaires |

Ce que l’interface **ne doit pas** exprimer :

- Un template Material générique (fuite lavande M3 actuelle).
- Une app de paiement Mobile Money (Wave / GeniusPay absents).
- Un réseau de messages (pas de chat).
- Un dashboard analytics (pas de service).
- Un catalogue Amazon (pas de recherche produits, PORO-026).

## 2. Problèmes visuels observés (audit)

Ces écarts justifient la refonte. Ce ne sont pas des goûts : ils cassent l’identité ou l’usage.

| # | Constat | Preuve | Impact |
|---|---|---|---|
| V-01 | Palette incomplète : 6 tokens, le reste est violet Material 3 | `Theme.kt` ; `Avatar` sur `primaryContainer` | Avatars et non-lus paraissent d’une autre app |
| V-02 | Aucune typo, forme, ni échelle d’espacement | Pas de `Type.kt` / `Dimens.kt` | 12 / 16 / 24 dp mélangés, double padding Scaffold boutique |
| V-03 | Fil : pas de scrim haut, bas à 60 % seulement | `FeedScreen.kt` L194 | Onglets et cloche illisibles sur vidéo claire |
| V-04 | Accueil boutique = pile de boutons outlined | `ShopHomeScreen.kt` | Pas une vitrine ; icône onglet = panier |
| V-05 | Profil public sans contenu, commentaires sans nom | `PublicProfileScreen`, comments `user_id` | Expérience sociale anonyme |
| V-06 | Chip Créateur cliquable sans action | `AssistChip(onClick = {})` | Contrôle mort TalkBack |
| V-07 | Voix tu / vous mélangée | « Pour toi » vs « Abonnez-vous » | Marque instable |
| V-08 | Images non clipées, empty states texte seul | Cart, éditeur, listes | Finition amateur |
| V-09 | Fenêtre XML toujours claire, Compose suit le système | `themes.xml` vs `PoroTheme` | Barre de statut désaccordée du fil noir |
| V-10 | Aucun signalement ni écran légal | Routes absentes | Stores + confiance |

## 3. Concept : chaleur + clarté

**Poro n’est pas froid.** L’orange `#FF6A00` est déjà la marque : énergie de marché, action, soleil. Le vert `#1B7F5A` est déjà la confiance : confirmation, succès, argent reçu plus tard — jamais un second CTA.

La direction artistique combine trois registres **déjà présents dans le produit** :

1. **Immersion sociale** — le fil reste un cinéma vertical. Pas de cartes autour du player.
2. **Clarté commerciale** — prix, statut de commande, paiement à la livraison sont des faits, pas du décor.
3. **Simplicité grand public** — une action principale par écran, 48 dp, français, formulaires courts.

Identité africaine contemporaine, **subtile** :

- Chaleur des fonds (`#FFFBF8`, pas un blanc clinique).
- Orange saturé, jamais pastel.
- Densité tactile généreuse (usage une main, soleil, 3G).
- Typographie de plateforme (Roboto / police système) : lisible, pas une police « ethnique ».

Interdit : motifs wax partout, silhouettes safari, dégradés sunset sur chaque carte, illustrations clip-art, ornements « afrofuturistes » génériques.

## 4. Palette de travail (extension de PoroTheme)

Valeurs **déjà dans le code** conservées. Valeurs nouvelles = comblement des slots M3, pas une nouvelle marque.

| Rôle | Clair | Sombre | Fonction |
|---|---|---|---|
| Primary | `#FF6A00` | `#FF8A3D` | CTA, onglet actif, focus |
| On primary | `#FFFFFF` | `#000000` | Texte sur CTA |
| Primary container | `#FFE4D1` | `#4A2800` | Avatar fallback, chips sélection |
| On primary container | `#3D1600` | `#FFE4D1` | Initiale avatar ≥ 4,5:1 |
| Secondary | `#1B7F5A` | `#4CC497` | Succès, renvoyé, bannière utile |
| Secondary container | `#D4EFE4` | `#0F3D2C` | Non-lus, email à vérifier |
| Background | `#FFFBF8` | `#121212` | Écrans hors fil |
| Surface | `#FFFFFF` | `#1E1E1E` | Champs, barres, cartes |
| Surface variant | `#F4EBE4` | `#2A2A2A` | Fonds de liste, chips |
| Error | `#B3261E` | `#F2B8B5` | Erreurs métier |
| Outline | `#85746B` | `#9A8A82` | Bordures champs |
| Like (hors scheme) | `#FF3B5C` | `#FF3B5C` | Cœur social — convention, documenté |
| Overlay ink | `#FFFFFF` | — | Texte sur vidéo |
| Overlay scrim | noir 0 % → 72 % | — | Bas du fil |
| Overlay top | noir 40 % | — | Onglets + cloche |

Le fil **ignore** le thème clair : chrome toujours sombre. C’est déjà le cas (`Color.Black` / `White`). On le officialise.

## 5. Hiérarchie visuelle (règle d’écran)

Chaque écran a **un seul** niveau 1.

```
1. Action principale     — Button / SubmitButton, orange, pleine largeur si formulaire
2. Contenu essentiel     — vidéo, prix, code OTP, liste
3. Actions secondaires   — Outlined / TextButton
4. Méta                  — dates, @handle, request id, mentions légales
```

Exemples :

| Écran | Niveau 1 | Interdit en niveau 1 |
|---|---|---|
| Fil | La vidéo | Un bandeau boutique |
| Produit | Prix + Ajouter au panier | Follow du vendeur |
| Checkout récap | Confirmer la commande | Un sélecteur Wave |
| OTP | Valider | Créer une boutique |
| Shop home | Trouver `@handle` | Un carrousel inventé |

## 6. Voix et libellés

Langue : **français uniquement** (ADR-0004). SMS Nigeria en anglais côté serveur, pas dans l’UI.

Proposition de voix (à appliquer progressivement, sans casser les strings stables métier) :

- Tutoiement **sur le fil seulement** (« Pour toi ») — déjà là, conserver.
- **Vouvoiement** partout ailleurs (auth, boutique, paramètres) — déjà dominant, conserver.
- Ne pas « moderniser » les copies COD honnêtes (`checkout_placed`, `cart_payment_notice`).
- `action_unfollow` aujourd’hui = « Abonné·e » (état). Le garder comme **état du bouton**, pas le remplacer par « Se désabonner » en primaire (évite un tap accidentel). Long-press ou second tap confirme le unfollow — comportement actuel à documenter, pas à inverser sans test.

## 7. Motion

| Autorisé | Interdit |
|---|---|
| Pager vertical natif | Lottie, confettis, bounce de like |
| Ripple M3 | Dégradés animés |
| Spinner existant | Skeleton complexes sur le fil (coût GPU) |
| Crossfade miniature → HLS (déjà Coil/player) | Double-tap like (non demandé, conflit accessibilité) |
| Durées système (`animatorDurationScale`) | Animations qui ignorent reduced motion |

Le like peut rester un changement de teinte immédiat (déjà optimistic). Pas de particules.

## 8. Iconographie

Conserver Material filled pour la barre (Home, Search, AddCircle, **Store** à la place de ShoppingCart, Person).

| Aujourd’hui | Proposition | Pourquoi |
|---|---|---|
| Onglet Boutique = `ShoppingCart` | `Icons.Filled.Store` | Le panier est une destination secondaire, pas l’onglet |
| Actions vidéo = Favorite, Comment custom, Share | Conserver + `MoreVert` | Report P0, pas une 6e tab |
| Cloche blanche | Conserver, badge `secondary` | Déjà compris |

Pas de set d’icônes custom. `PoroIcons.Comment` reste l’exception déjà livrée.

## 9. Surfaces et profondeur

| Surface | Élévation | Rayon |
|---|---|---|
| Écran | 0 | 0 |
| Carte produit / commande | 0–1 dp, bordure `outline` 1 dp | 12 dp (M3 medium) |
| Bottom sheet | M3 défaut | Coins hauts 28 dp |
| Dialogue | M3 défaut | 28 dp |
| Avatar | 0 | Cercle |
| Miniature vidéo / produit | 0 | 8 dp (clip obligatoire) |
| CTA | 0 (pas d’ombre orange) | Full / 12 dp M3 |

Pas d’ombres portées « premium ». Le contraste vient de la couleur et de l’espace.

## 10. Références d’ambition (pas à copier)

| Qualité à viser | Référence de métier | Ce qu’on prend | Ce qu’on refuse |
|---|---|---|---|
| Immersion vidéo | TikTok / Instagram Reels | Plein écran, actions droites, scrims | Live, duets, shop overlays inventés |
| Commerce de confiance | Jumia / marchés locaux | Prix fort, COD explicite, statuts | Faux « payé », vitrine infinie |
| Auth simple | WhatsApp OTP | 6 cases, pays, renvoyer | Social login absent du backend |
| Profil | Instagram léger | Avatar, bio, grille **si API** | Grille publique tant que `GET /videos` est owner-only |

## 11. Maquettes

Visuels de direction (images, pas un prototype interactif) :

| Fichier | Sujet |
|---|---|
| [`mockups/poro-design-system.jpg`](mockups/poro-design-system.jpg) | Tokens |
| [`mockups/poro-feed-home.jpg`](mockups/poro-feed-home.jpg) | Fil |
| [`mockups/poro-shop-home.jpg`](mockups/poro-shop-home.jpg) | Hub boutique |
| [`mockups/poro-product.jpg`](mockups/poro-product.jpg) | Fiche produit |
| [`mockups/poro-checkout-review.jpg`](mockups/poro-checkout-review.jpg) | Récap COD |
| [`mockups/poro-auth-phone.jpg`](mockups/poro-auth-phone.jpg) | Connexion |

Les écarts décoratifs des images (photo marché, recherche produit, frais offerts) sont listés dans [`mockups/README.md`](mockups/README.md). **Le code et l’OpenAPI l’emportent.**

## 12. Décisions figées ici

1. Étendre `PoroTheme`, ne pas créer un second système.
2. Orange primaire, vert succès, rose like documenté.
3. Fil toujours chrome sombre.
4. Boutique visible, découverte par `@handle` jusqu’à PORO-026.
5. Pas de Mobile Money à l’écran.
6. Mode invité spécifié, **non activable** tant que `/feed/trending` exige un Bearer.
7. Textes légaux = « non validé / indisponible » jusqu’à Q-03.
8. Aucune police custom tant que Q-10 n’est pas livré.
