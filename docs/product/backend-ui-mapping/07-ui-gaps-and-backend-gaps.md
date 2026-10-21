# Écarts UI vs écarts backend

Preuves : matrice [02](02-endpoint-ui-traceability.md). Recommandation ≠ implémentation.

## 1. Backend prêt, UI manquante ou partielle

| Écart | Preuve | Reco | P |
|---|---|---|---|
| PATCH commentaire | `SocialRepository.editComment` ; pas d’item menu | Menu 15 min dans la sheet | P1 |
| GET followers/following | routes Opt ; pas de `FollowListRoute` | Écran liste | P1 |
| GET likes listes | social OpenAPI | P2 | P2 |
| DELETE logo | `shops.controller` DELETE | TextButton MyShop | P2 |
| DELETE /cart | order API | P2 vider | P2 |
| CRUD adresses | API complète ; chips checkout seulement | Écran carnet | P1 |
| PUT/DELETE devices | notif API | Après Firebase | P1 |
| POST reports | API + ReportSheet **non commité** | Valider lots 1–3 | P0 |
| returnTo / guest | APIs publiques | Après B-FEED | P0 |
| Aperçu photo local | shop upload immédiat | Lot M2 | P1 |
| Desc vidéo 500 runes | model `MaxDescriptionRunes` | Aligner champ | P1 |
| display_name/bio trop larges | user 50/160 | Aligner EditProfile | P0 |
| Switch push | prefs API, pas FCM | Labelliser | P0 |
| `SHARE_BASE_URL` vide | BuildConfig | Config + share lien | P1 |
| Mute / HLS error | player | P1 lecture | P1 |

## 2. UI ou promesse sans backend

| Surface | Manque | Reco |
|---|---|---|
| Fil / publish image | Pas de `kind` image | B-IMG-POST ; pas de picker mixte |
| Enregistrer vidéo | Pas de MP4 | Pas de bouton |
| Auth-wall vs « découvrir » | Feed Auth + Phone start | B-FEED ou rester wall |
| Catalogue boutique | PORO-026 | Handle only |
| Favoris, block, chat, live | Absents | Pas de bouton |
| Wave / « Payée » | payment absent | COD only |
| Grille profil public | `GET /videos` owner-only | Pas de scrape |
| Légal opposable | Pas de textes | Placeholder Q-03 |
| Push réel | Pas de `google-services.json` | Q-21 |
| WorkManager upload | Absent | Copie honnête |

## 3. Internes — pas d’UI directe

Outbox, worker, sharp, erasure, JWKS, health, metrics, DLQ, seaweed IAM.  
Indirect : `processing`, notif `video_ready`, 404 après delete, latence feed.

## 4. Staff — documenté, hors app

`GET/POST /moderation/cases*`, `POST /admin/shops/{id}/suspend|reinstate`. Reco Q-16 : app séparée.

## 5. Risques d’honnêteté (P0)

1. Switch push qui écrit le serveur sans jeton appareil.
2. Chip « Devenir créateur » lu comme un droit de publier.
3. `hasShop` null → « Ouvrir » (lot 1 vise à corriger).
4. Add-to-cart qui remplaçait la qty (lot 1 : somme).
5. Description / bio plus larges que le serveur (422 surprise).
6. Guest impossible alors que des GET sont publics (attente vs réalité).
7. IP affichée dans Settings (Q-09).

Lots 1–3 Android **corrigent** une partie (panier, report, légal, hasShop) mais **non commités / non QA appareil**.
