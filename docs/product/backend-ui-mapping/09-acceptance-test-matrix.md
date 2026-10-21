# Matrice de tests d’acceptation

Les AC détaillés restent dans leurs dossiers. Ici : **traçabilité** capacité → scénario.

| Famille | Où | Couvre |
|---|---|---|
| AC-AUTH-* | [ux-audit/10](../mobile-ux-audit/10-acceptance-criteria.md) | E-A01–16 |
| AC-FEED / UPLOAD / SOC | idem | E-F*, E-V*, E-SO* membre |
| AC-MX-* | [phase-2/08](../mobile-ux-phase-2/08-acceptance-tests.md) | E-SH*, E-O* |
| AC-MOD / AC-LEG | phase-2 | E-M01, légal |
| NR-01–06 | phase-2 / design | Non-régression social+shop |
| AC-GST / AC-401 | [guest-mode/08](../guest-mode/08-acceptance-tests.md) | Visiteur, B-FEED |
| AC-SOC / AC-MX-IMG / AC-SAV | [mobile-media/08](../mobile-media/08-acceptance-tests.md) | Médias |
| AC-MAP-* | ci-dessous | Trous non couverts ailleurs |

## AC-MAP (compléments)

| ID | Scénario | Attendu | Endpoint |
|---|---|---|---|
| AC-MAP-01 | Bio 161 car. | Refus local ou 422, pas succès | E-U01 |
| AC-MAP-02 | Desc vidéo 501 runes | Refus avant init | E-V01 |
| AC-MAP-03 | Modifier commentaire <15 min | Cible P1 ; aujourd’hui pas d’UI | E-SO09 |
| AC-MAP-04 | Ouvrir followers | Cible P1 | E-SO15 |
| AC-MAP-05 | Prefs push sans Firebase | Copy « non actif », pas de devices | E-N05, E-N07 |
| AC-MAP-06 | Share sans SHARE_BASE_URL | Texte sans lien, pas de crash | E-SO18 |
| AC-MAP-07 | JWT ADMIN dans l’app | 0 écran cases/admin | E-M02, E-SH20 |
| AC-MAP-08 | Visiteur 0 Authorization | Interceptor | tous GET publics |
| AC-MAP-09 | Confirm checkout réseau | Vérif / GET orders, pas « échec » seul | E-O07 |
| AC-MAP-10 | HEIC produit | Format, 0 POST | E-SH14 |
| AC-MAP-11 | 11e photo | too_many_images local | E-SH14 |
| AC-MAP-12 | Session IP | Décision Q-09 | E-A12 |

## Acteurs à jouer

Visiteur (après B-FEED), membre, owner vidéo, vendeur, acheteur, session expirée. **Pas** de scénario staff dans l’APK grand public.

## Définition d’échec

Message anglais brut ; succès sans `data` ; action interdite parce que le bouton est visible ; notif / favoris / Wave / image sociale / save HLS présentés comme prêts.
