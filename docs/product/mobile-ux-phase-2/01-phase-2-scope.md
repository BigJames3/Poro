# Phase 2 — Périmètre, décisions et exclusions

- **Date** : 21 octobre 2026.
- **Nature** : conception et spécification. **Aucun fichier de `apps/mobile-android` n’est modifié.**
- **Base** : code et OpenAPI en vigueur, audit [mobile-ux-audit](../mobile-ux-audit/01-product-overview.md).
- **Branche observée** : `claude/gifted-ritchie-t5ej2z`. Fichiers non suivis : `docs/product/` uniquement. Aucun diff Android.

## 1. Décisions produit (cette phase)

| # | Décision | Conséquence pour la conception |
|---|---|---|
| 1 | La boutique **reste visible**. | Scénario B de l’audit : stabiliser COD, panier, vendeur. Ne pas masquer l’onglet. |
| 2 | Mode invité **souhaité** pour un fil public, sous réserve backend et sécurité. | Faisabilité : [03](03-guest-mode-feasibility.md). **Le fil est bloqué** tant que `/feed/*` exige un Bearer. |
| 3 | Écrans légaux **préparés**, textes non publiés sans validation. | Placeholders « À valider ». Pas de clauses inventées. |
| 4 | `PoroTheme` reste la référence. | Extension documentée, pas de second système. |
| 5 | Interfaces natives Compose. | Pas de WebView, pas de graphe alternatif hors spec d’évolution du `NavHost` existant. |
| 6 | Pas d’écran sur une API absente. | Pas de Wave, chat, recherche catalogue, GPS, FCM devices. |
| 7 | Parcours sociaux opérationnels **préservés**. | Feed, like, commentaires, follow, partage, recherche, profil : non-régression. |

Q-01 de l’audit est **tranché** (boutique visible). Q-02 est **souhaité mais non activable** sur le contrat feed actuel. Q-03 reste ouvert (textes).

## 2. Module Android (lecture seule)

| Élément | Valeur |
|---|---|
| Chemin | `apps/mobile-android/` (`:app`, `com.poro.app`) |
| Navigation | `ui/Routes.kt`, `ui/PoroNavHost.kt`, `ui/shop/ShopGraph.kt` |
| Thème | `ui/theme/Theme.kt` |
| Règle | Toute implémentation ultérieure : Android Studio, après autorisation explicite |

## 3. Périmètre inclus

1. Diagnostic et matrice de stabilisation marketplace (COD uniquement).
2. Étude du mode invité + matrice d’accès + changements backend **nécessaires** (non réalisés ici).
3. Spec signalement (`POST /moderation/reports`).
4. Spec écrans légaux (contenu non validé).
5. Extension du design system existant.
6. Specs d’écrans, tests d’acceptation, plan d’implémentation par lots.

## 4. Exclusions explicites

| Exclu | Preuve d’absence |
|---|---|
| Wave / GeniusPay / Mobile Money | Pas de `services/payment` |
| Recherche produits / boutiques | PORO-026 ; accueil = `@handle` |
| Chat, live, favoris, blocage | Pas d’API |
| File modérateur / admin boutique dans l’app | JWT staff ≠ produit grand public |
| Firebase / WorkManager / mute player | P1, lot 5, hors lots 1–3 |
| Modification gateway, services, OpenAPI | Cette phase documentation seulement |
| Commit, merge, déploiement | Interdit |

## 5. Contradiction documentation vs code (non corrigée silencieusement)

| Source | Affirme | Code / contrat actuel |
|---|---|---|
| ADR-0004 | Marketplace P1 après le social | Onglet Boutique livré ; décision phase 2 = **garder** |
| `ANDROID-INTEGRATION.md` | Gabarit vide `com.example.myporo` | App `com.poro.app` branchée API |
| `.cursor/rules/project.md` | Marketplace hors MVP | Shop + order en production locale |
| Audit Q-01 | Deux scénarios | Tranché : boutique visible |

La vérification se fait toujours sur OpenAPI + handlers + Kotlin, pas sur les docs périmés.

## 6. Livrables

| Fichier | Contenu |
|---|---|
| [02-marketplace-audit.md](02-marketplace-audit.md) | Parcours, risques, matrice écran→API |
| [03-guest-mode-feasibility.md](03-guest-mode-feasibility.md) | Accès public, blocage feed, proposition UX |
| [04-reporting-ux-spec.md](04-reporting-ux-spec.md) | Signalement |
| [05-legal-screens-spec.md](05-legal-screens-spec.md) | Légal |
| [06-design-system-extension.md](06-design-system-extension.md) | PoroTheme |
| [07-screen-specifications.md](07-screen-specifications.md) | États d’écrans |
| [08-acceptance-tests.md](08-acceptance-tests.md) | QA |
| [09-implementation-plan.md](09-implementation-plan.md) | Lots 0–6 |
| [10-blockers-and-decisions.md](10-blockers-and-decisions.md) | Décisions restantes |

Rien de ce dossier n’est **implémenté**. C’est une spec prête pour revue.
