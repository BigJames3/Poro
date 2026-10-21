# Feuille de route priorisée

Aucun Kotlin / backend dans **cette** phase. Lots existants (phase-2, design, media, guest) **non fusionnés** : on les ordonne.

## P0 — Cohérence et sécurité

| Lot | But | Dépendance | Android ? |
|---|---|---|---|
| Valider lots 1–3 | COD, report, légal déjà codés | QA appareil + Q-03 textes | déjà touché, non commité |
| Alignement limites | bio 50/160, desc vidéo 500 | — | oui après OK |
| Push honnête | Labelliser / cacher prefs_push | Q-21 | oui |
| B-FEED décision | Trending public **ou** garder le wall | Produit + sécu | non (d’abord backend) |
| Staff hors app | Confirmer 0 route cases | Q-16 | revue seulement |
| Pas de bouton fantôme | Image sociale, save vidéo, Wave, favoris, catalogue | — | ne pas ajouter |

## P1 — Complétude UX (API déjà là)

| Lot | Écart | Prérequis |
|---|---|---|
| G1–G2 guest | Fil Tendances + prompts + returnTo | **B-FEED vert** + OK Android |
| M1–M2 médias | Desc 500 ; aperçu photo boutique | OK Android |
| Follow lists | E-SO15/16 | OK Android |
| Edit comment | E-SO09 | OK Android |
| Carnet adresses | E-O08–12 | OK Android |
| Mute + HLS fail | player | OK Android |
| SHARE_BASE_URL | config | Produit |
| DELETE logo | P2 acceptable si P1 saturé | — |

## P2 — Futur / bloqué

| Sujet | Bloqué par |
|---|---|
| M5 post image | B-IMG-POST / Q-MED-06 |
| M3 enregistrer photo | Q-MED-01/02 |
| M4 enregistrer vidéo | B-VID-SAVE |
| FCM devices | Firebase |
| WorkManager upload | lot 5 phase-2 |
| Search catalogue | PORO-026 |
| Wave | PORO-015 / payment |
| Grille profil | API liste publique |
| Chat / favoris / block | Absent |
| App staff | Q-16 |
| values-en | Q-11 |

## Ordre recommandé

```
QA lots 1–3 + aligner limites + push honnête
  → décision B-FEED
       non → stop guest ; P1 social listes / comment / médias boutique
       oui → backend trending → G1–G2 → G3–G4
  → P1 restant
  → P2 uniquement après contrat
```

Ne pas ouvrir guest Android et un gros lot shop le même jour sur `PoroNavHost`.

## Définition de terminé

AC de la famille verts ; NR-01–06 ; 0 staff ; 0 secret ; pas de feature documentée présentée comme livrée.
