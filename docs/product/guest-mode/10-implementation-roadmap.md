# Feuille de route — mode visiteur

Aucun lot G* dans `apps/mobile-android` tant qu’il n’y a pas **autorisation explicite** **et** B-FEED (sauf G0 docs).

Pas de commit, branche, push dans cette phase.

Complète phase-2 lot 4 et [mobile-media](../mobile-media/09-implementation-plan.md) sans les fusionner.

## Lots

### G0 — Cette conception

| | |
|---|---|
| But | Spec + audit |
| Travaux | `docs/product/guest-mode/*` |
| Terminé | Ce dossier. **Rien n’est implémenté.** |

### B-FEED — Backend (autre équipe / autre autorisation)

Voir [09](09-backend-dependencies.md). Trending Opt + quota IP + OpenAPI + tests + ADR.

**Stop Android guest tant que ce lot n’est pas vert.**

### G1 — Mur Android + Tendances visiteur

| | |
|---|---|
| Prérequis | B-FEED vert ; autorisation Android |
| Travaux | `start` = `HomeRoute` si SignedOut ; `isPublicScreen()` (Home, Search, Shop, Video, PublicProfile, Legal, Product…) ; `goToSignIn` limité logout/session ; **ne plus** éjecter le visiteur ; FeedViewModel : visiteur = trending only, 0 `/users/me` ; tabs Pour toi/Following → prompt |
| Fichiers | `PoroNavHost.kt`, `AppNav.kt`, `FeedViewModel.kt`, `FeedScreen.kt` |
| Interdit | For You anonyme ; Bearer vide ; second NavHost |
| Tests | AC-GST-01…05, 18, 19 ; AC-401-01…04 ; NR-01 |
| Terminé | Visiteur voit des clips Tendances ; membre inchangé |

### G2 — Prompts et returnTo

| | |
|---|---|
| Prérequis | G1 |
| Travaux | `GuestAuthSheet` ; raisons ; empiler `authGraph` sans destroy ; returnTo ; replay like/follow seulement |
| Accroches | `VideoActions`, Comments composer, `PublicProfile` follow, cloche, tab Upload, cart CTA, Profile tab |
| Tests | AC-GST-08…12, 14…17 ; AC-AUTH-* |
| Interdit | Auto checkout / upload |

### G3 — Recherche, boutique lecture, profil visiteur

| | |
|---|---|
| Prérequis | G1 (mur levé) ; G2 pour les CTA protégés |
| Travaux | Search déjà public une fois le mur levé ; écran Profil visiteur ; shop sans GET me/cart ; badge panier absent |
| Tests | AC-GST-13, 14, 16 ; NR-04 |
| Interdit | Catalogue ; player vidéo shop |

### G4 — Durcissement session

| | |
|---|---|
| Prérequis | G2 |
| Travaux | Session expirée → visiteur, pas Phone piège ; logout → Tendances ; tests 401 |
| Terminé | AC-AUTH-05, 06 |

### Hors route guest

M5 image sociale, FCM, WorkManager, favoris, Wave, PORO-026.

## Ordre

```
G0 (fait, docs)
  → décision humaine B-FEED
       → non : stop, auth-wall
       → oui : backend B-FEED
            → autorisation Android
                 → G1 fil
                 → G2 prompts
                 → G3 shop/search/profil
                 → G4 session
```

## Définition de terminé d’un lot G

- AC du lot verts (unitaires a minima ; appareil pour G1 lecture).
- NR-01…06.
- 0 route staff.
- 0 secret.
- Visiteur : 0 header Authorization sur les GET publics.
- Kotlin seulement après OK **de ce lot**.

## Rapport final (cette phase)

| Question | Réponse |
|---|---|
| Faisable avec le code **API** actuel, fil compris ? | **Non** — `/feed/*` Auth |
| Faisable lecture isolée (vidéo, search, shop, comments GET) ? | **Oui côté API** ; **non côté app** (mur Phone) |
| Changements backend nécessaires ? | **Oui** : B-FEED uniquement pour le fil promis |
| Que reste bloqué ? | Fil image (B-IMG-POST), FCM, favoris, catalogue, enregistrer une vidéo, guest Android avant B-FEED |
| Quoi implémenté aujourd’hui ? | **Rien** du mode visiteur — docs seulement |
