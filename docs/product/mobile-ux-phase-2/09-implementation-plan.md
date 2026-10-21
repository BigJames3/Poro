# Plan d’implémentation

Lots 1–3 **livrés** dans `apps/mobile-android` (non commités). Statut : **À valider**
(appareil + gateway + textes juridiques). Lots 4–6 inchangés.

L’ordre reste : marketplace visible → report → légal → invité **après** backend → P1 → futurs.

## Lot 0 — Contrats et docs

| | |
|---|---|
| But | Lever les écarts documentaires ; **pas de Kotlin** |
| Prérequis | — |
| Travaux | Tamponner `ANDROID-INTEGRATION.md` / état des lieux (PORO-032) : client existant ; noter `login_throttled` hors enum auth |
| Fichiers | `docs/api/*.yaml`, `docs/api/ANDROID-INTEGRATION.md` — **pas** Android |
| Tests | Relire OpenAPI vs handlers (déjà faits pour cette spec) |
| Risques | Docs encore contradictoires si non faits |
| Terminé | Revue produit de ce dossier phase 2 ; Q-03 owner nommé |

## Lot 1 — Stabilisation marketplace

| | |
|---|---|
| But | Parcours COD honnêtes et robustes ; onglet Boutique conservé |
| Prérequis | Autorisation **explicite** de modifier `apps/mobile-android` |
| Travaux | Somme panier ; max 10 photos ; confirm réseau → retry/GET orders ; `hasShop` null ; prefill erreur visible |
| Fichiers potentiels | `ProductViewModel.kt`, `ProductEditorScreen.kt`, `CheckoutViewModel.kt`, `ShopHomeViewModel.kt`, tests `*Shop*`, `*Checkout*` |
| Interdit | Wave, search catalogue, GPS, changer les strings « Payée » en succès COD |
| Tests | AC-MX-*, NR-06, tests unitaires ViewModel existants étendus |
| Risques | PUT quantité = sémantique serveur (absolu) : bien relire le panier |
| Terminé | Code + tests unitaires (somme panier, 10 photos, retry confirm, hasShop, prefill). **À valider** sur appareil : AC-MX-03, 08, 09, 11 ; pas de libellé Payée après COD |

## Lot 2 — Signalement

| | |
|---|---|
| Prérequis | Lot 1 mergeable ; OpenAPI moderation inchangé |
| Travaux | `ModerationApi` + `ReportSheet` + ⋮ sur vidéo, commentaire, profil ; mapping erreurs |
| Fichiers | **nouveaux** sous `ui/moderation/`, `data/moderation/` ; accroches `SocialVideoActions`, `CommentsSheet`, `PublicProfileScreen` ; `AuthErrorMessages.kt`, `strings.xml` |
| Tests | AC-MOD-* ; ViewModel fake 201/200/422/429 |
| Risques | Self report ; ne pas exposer la file |
| Terminé | Code + tests ViewModel 201/200/422/429. **À valider** sur appareil : 7 motifs ; doublon non-erreur ; ⋮ vidéo/commentaire/profil |

## Lot 3 — Légal

| | |
|---|---|
| Prérequis | Textes **ou** décision d’afficher « indisponible » (défaut) |
| Travaux | `LegalRoute` / document ; liens Settings + auth ; session observer n’éjecte pas |
| Fichiers | `ui/legal/`, `Routes.kt`, `PoroNavHost` / `profileGraph` / `authGraph`, `SettingsScreen.kt`, `strings.xml` |
| Tests | AC-LEG-* |
| Risques | Publier un placeholder comme politique Play |
| Terminé | 3 entrées + routes déconnectées. Corps opposable **absent**. **À valider** : textes juridiques (Q-03) ; AC-LEG-03 sur appareil |

## Lot 4 — Mode invité

| | |
|---|---|
| Prérequis | **Backend** : trending optionnel + quota IP + OpenAPI/ADR (voir [03](03-guest-mode-feasibility.md) §8). Autorisation Android séparée. |
| Travaux | Start `HomeRoute` si guest ; tabs sans Publier ou Publier→login ; Tendances seules ; gates like/cart ; `returnTo` |
| Fichiers | `PoroNavHost.kt`, `FeedViewModel.kt`, `FeedScreen.kt`, `AppNav.kt`, shop/social graphs |
| Tests | AC-GST-* **et** NR-01 à 06 (connecté intact) |
| Risques | Casser le start authentifié ; envoyer un Bearer vide ; For You anonyme |
| Terminé | Visiteur voit trending 200 ; connecté inchangé ; 0 appel `/users/me` visiteur |
| Si backend refusé | Lot **annulé** ; rester auth-wall |

## Lot 5 — P1 lecture / upload / FCM

WorkManager upload ; mute + erreur HLS ; Firebase + `PUT /notifications/devices` ; labelliser ou cacher `prefs_push` tant que pas de projet. Hors lots 1–3. Prérequis : projet Firebase (Q-21).

## Lot 6 — Futurs conditionnés backend

Wave (PORO-015), search shop (PORO-026), followers UI, edit comment, App Links, `values-en`. **Un service = un lot**, pas d’écran spéculatif.

## Ordre exact le jour J (après OK Android)

1. Lot 1 (marketplace) + tests unitaires shop/checkout.
2. Lot 2 (report) + tests moderation.
3. Lot 3 (légal indisponible).
4. Stop. Backend trending. Revue sécu.
5. Lot 4 si ADR feed acceptée.
6. Lot 5 selon Firebase.
7. Lot 6 à la carte.

Social existant : pas de refactor navigation globale dans le lot 1. Design tokens (`Theme.kt`) : opportuniste dans lot 2 (sheet) / 1 (chip créateur no-op si touché), pas un lot isolé bloquant.

## Définition de terminé globale d’un lot

- Critères AC du lot verts en local.
- Aucune route staff ajoutée.
- Aucun secret commité.
- `apps/mobile-android` modifié **uniquement** après autorisation de ce lot.
- Documentation audit phase 1 **préservée**.
