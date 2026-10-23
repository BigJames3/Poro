# Rapport QA lots 1–3 — 21 octobre 2026

Exécution autorisée par le propriétaire produit. Script :
`docs/product/qa/run-lots-1-3-gateway.ps1` contre `http://127.0.0.1:8000`
(compose local déjà levé). Aucun secret dans ce rapport.

**Kotlin guest et post image : non exécutés, non autorisés.**

## 1. Synthèse

| Famille | Résultat |
|---|---|
| Auth e-mail via gateway | PASS |
| Boutique + 10 photos + `image_required` / `too_many_images` | PASS |
| Vitrine publique handle | PASS |
| Panier / checkout COD (AC-MX-03…10) | à rejouer après Snappy (catalogue order lit de nouveau Kafka) |
| Signalement AC-MOD-01…07 + cases staff | PASS |
| Prefill checkout | PASS (source empty, lookup ok) |
| `GET /feed/trending` sans JWT | **200** (ADR-0019 livré ; page éventuellement vide) |
| Search | **503** (`poro-search` Created, pas démarré) |
| Contrat Traefik isolé | **PASS** — 133 routes documentées + CORS / gzip / 413 / 429 |
| QA appareil (AC-LEG-03, NR-01…06, TalkBack) | **Non faite** — Android Studio / appareil |

## 2. Gateway fonctionnelle (détail)

### Passés

- Création de deux comptes e-mail, ouverture boutique CI.
- `GET /shops/inconnu…` → 404 `shop_not_found`.
- Draft produit ; `PATCH status=active` sans photo → 422 `image_required` (AC-MX-16).
- 10 photos JPEG confirmées ; 11ᵉ `upload-url` → 422 `too_many_images` (AC-MX-17).
- Produit `active` ; `GET /shops/{handle}` 200 immédiat (AC-MX-01).
- Prefill 200.
- Report vidéo 201 puis doublon 200 ; self 422 `cannot_report_self` ; motif hors enum 400 ;
  commentaire 501 caractères 400 ; cible `comment` 201 ; 11ᵉ report / h 429 (AC-MOD-07).
- `GET /moderation/cases` en PERSONAL → 403 (AC-SEC-03).

### Bloqués (pas un bug du client Android)

**Corrigé le 21 oct. 2026** : codec Snappy (`snappyjs`) dans les
consommateurs KafkaJS. Après rebuild de `poro-order`, le catalogue lit
`product.updated` ; `PUT /cart` et `checkout/preview` passent.

Relance QA : AC-MX-03, 04, 07, 09 verts. Confirm COD → **201** `pending`
(saga stock : rebuild `shop` encore requis pour passer à `confirmed`).
AC-MX-06/10 étaient des bugs de script (panier vendeur / panier vidé),
corrigés dans `run-lots-1-3-gateway.ps1`.

### Hors stack

- `poro-social`, `poro-search` : Created, pas Running. NR-02 (like/comment)
  et search réelle impossibles.
- `poro-media-worker` : Created. Upload vidéo / HLS non testé (NR-04).

## 3. QA appareil — non exécutée

`.cursor/rules/project.md` réserve `apps/mobile-android` à Android Studio.
Aucun émulateur n’a été piloté depuis Cursor.

Reste à faire **sur appareil** (humain) :

| ID | Scénario |
|---|---|
| AC-MX-03, 08, 09, 11 | Après correctif Snappy : 2× ajouter, confirm COD, idempotence, airplane |
| AC-LEG-01…03 | Settings 3 lignes ; corps toujours « indisponible » (brouillons non embarqués) ; Phone → légal sans bounce |
| AC-LEG-04 | Delete account intact |
| NR-01…06 | Login OTP, like/comment (social down), search, upload, logout, 5 onglets |
| AC-SEC-01 | Pas d’OTP / refresh dans Logcat |
| AC-A11Y-* | TalkBack |

Les textes `docs/legal/` **ne doivent pas** apparaître comme opposables dans
l’APK tant qu’un avocat n’a pas visé.

## 4. Contrat gateway isolé

`python3` hôte absent. Relance : Traefik + `whoami` + `python:3.12-slim`
exécutant `check_gateway.py` sur le port 18000.

**Résultat : `gateway: 133 documented routes and the gateway behaviour check out`.**

## 5. Décisions liées, hors implémentation

- **B-FEED accepté** : [ADR-0019](../../architecture/adr/0019-feed-trending-public.md).
  Trending reste 401 aujourd’hui. Pas de Kotlin guest.
- **Q-03** : brouillons dans `docs/legal/`. App inchangée.
- **Q-MED-01** : toujours non.
