# Poro — Critères d’acceptation design

Critères **objectifs**, testables sur émulateur ou appareil, sans goût personnel. Ils s’ajoutent aux AC métier [phase-2/08](../mobile-ux-phase-2/08-acceptance-tests.md) et à l’audit [10](../mobile-ux-audit/10-acceptance-criteria.md).

Légende : **Visuel** (capture / revue) · **Fonctionnel** (parcours) · **A11y** · **Non-régression**.

## 1. Identité et thème

| ID | Critère | Type | Lot |
|---|---|---|---|
| DA-TH-01 | `primary` clair = `#FF6A00`, sombre = `#FF8A3D` | Visuel | D0 |
| DA-TH-02 | `secondary` clair = `#1B7F5A`, sombre = `#4CC497` | Visuel | D0 |
| DA-TH-03 | Aucun `primaryContainer` / `secondaryContainer` lavande M3 sur avatar, banner email, unread | Visuel | D0 |
| DA-TH-04 | Like actif = `#FF3B5C`, pas l’orange primaire | Visuel | D0 |
| DA-TH-05 | Fond hors fil clair = `#FFFBF8` ; fil = noir | Visuel | D0 |
| DA-TH-06 | Thème suit le système ; pas de toggle in-app | Fonctionnel | D0 |
| DA-TH-07 | Status bar du fil : icônes claires | Visuel | D0 |
| DA-TH-08 | Aucune nouvelle dépendance Gradle UI | Fonctionnel | tous |
| DA-TH-09 | Un seul `PoroTheme` / `MaterialTheme` produit | Fonctionnel | D0 |

## 2. Navigation

| ID | Critère | Type |
|---|---|---|
| DA-NAV-01 | 5 tabs : Accueil, Rechercher, Publier, Boutique, Profil | Fonctionnel |
| DA-NAV-02 | Pas de tab Messages / Analytics / Notifications | Fonctionnel |
| DA-NAV-03 | `saveState` / `restoreState` inchangés | NR |
| DA-NAV-04 | Barre cachée sur Video, Cart, Settings, PublicProfile | Fonctionnel |
| DA-NAV-05 | Legal accessible SignedOut sans éjection | Fonctionnel |
| DA-NAV-06 | Logout vide le graphe vers Phone | NR |

## 3. Fil

| ID | Critère | Type | Lot |
|---|---|---|---|
| DA-FD-01 | Trois kinds Pour toi / Abonnements / Tendances | NR | D2 |
| DA-FD-02 | Scrim haut visible : tabs + cloche lisibles sur vidéo blanche | Visuel | D2 |
| DA-FD-03 | Scrim bas ≥ 70 % : titre lisible sur vidéo blanche | Visuel | D2 |
| DA-FD-04 | Play/pause tap ; pas de double-tap like | Fonctionnel | D2 |
| DA-FD-05 | Actions like / comment / share fonctionnent | NR | D2 |
| DA-FD-06 | ⋮ ouvre Signaler (après lot report) | Fonctionnel | D2 |
| DA-FD-07 | Aucun overlay produit / Wave | Visuel | D2 |
| DA-FD-08 | Empty following propose Search | Fonctionnel | D5 |
| DA-FD-09 | Une page settled = un HLS play | NR | D2 |
| DA-FD-10 | Invité absent tant que trending 401 | Fonctionnel | D7 off |

## 4. Auth et profil

| ID | Critère | Type |
|---|---|---|
| DA-AU-01 | Un CTA primaire orange par écran auth | Visuel |
| DA-AU-02 | OTP 6 cases, touch ≥ 48 dp | A11y |
| DA-AU-03 | Pays hors +225/221/237/234 refusé | NR |
| DA-AU-04 | Chip Créateur non cliquable / non focusable | A11y |
| DA-AU-05 | display_name ≤ 50, bio ≤ 160 côté UI | Fonctionnel |
| DA-AU-06 | Delete account inchangé (J+30, password ou OTP) | NR |
| DA-AU-07 | Settings : 3 liens légaux même indisponibles | Fonctionnel |

## 5. Marketplace

| ID | Critère | Type |
|---|---|---|
| DA-MX-01 | Hub : 1 carte find + accès cart/orders/sell distincts | Visuel |
| DA-MX-02 | Pas de grille catalogue globale | Fonctionnel |
| DA-MX-03 | Prix via `Formats.money` partout | Fonctionnel |
| DA-MX-04 | Thumbs produit clipées | Visuel |
| DA-MX-05 | Aucun sélecteur Mobile Money | Visuel |
| DA-MX-06 | Confirm → `checkout_placed` + commandes `confirmed` | NR / phase 2 |
| DA-MX-07 | Aucun libellé Payée après COD | Fonctionnel |
| DA-MX-08 | `hasShop` null ≠ « Ouvrir » | Fonctionnel |
| DA-MX-09 | 2× add qty 1 → 2 (somme) | Fonctionnel / phase 2 |
| DA-MX-10 | Photos éditeur max 10 | Fonctionnel / phase 2 |

## 6. Signalement et légal

| ID | Critère | Type |
|---|---|---|
| DA-MD-01 | 7 motifs enum seulement | Fonctionnel |
| DA-MD-02 | 201 snackbar envoyé ; 200 doublon non-erreur | Fonctionnel |
| DA-MD-03 | 422 self message métier | Fonctionnel |
| DA-MD-04 | Pas de `case_id` affiché | Fonctionnel |
| DA-LG-01 | Aucun paragraphe présenté comme opposable si non validé | Visuel |
| DA-LG-02 | Version/date absentes si non fournies | Visuel |
| DA-LG-03 | Pas de WebView produit | Fonctionnel |

## 7. États

| ID | Critère | Type |
|---|---|---|
| DA-ST-01 | Chaque liste P0 a un empty | Visuel |
| DA-ST-02 | Chaque POST P0 a un working | Fonctionnel |
| DA-ST-03 | 401 → Phone | NR |
| DA-ST-04 | 429 auth/report : countdown | Fonctionnel |
| DA-ST-05 | Erreur = code mappé FR, pas `error.message` | Fonctionnel |

## 8. Accessibilité et responsive

| ID | Critère | Type |
|---|---|---|
| DA-A11Y-01 | 0 cible P0 < 48 dp | A11y |
| DA-A11Y-02 | Titres `heading`, erreurs liveRegion | A11y |
| DA-A11Y-03 | Font scale 2.0 : checkout et OTP utilisables | A11y |
| DA-A11Y-04 | Largeur 320 dp : hub et fil utilisables | Visuel |
| DA-A11Y-05 | TalkBack parcours OTP → like → add cart | A11y |

## 9. Non-régression produit

| ID | Critère |
|---|---|
| DA-NR-01 | Login OTP + email + reset |
| DA-NR-02 | Feed 3 kinds + like + comments + share + follow |
| DA-NR-03 | Upload picker + limites |
| DA-NR-04 | Search videos/users/hashtags |
| DA-NR-05 | Notif liste + prefs |
| DA-NR-06 | COD preview ≠ commande ; confirm crée |
| DA-NR-07 | Seller ship / buyer receipt / cancel + dialogue |
| DA-NR-08 | Pas d’appel `/users/me` si un jour visiteur |

## 10. Revue visuelle (checklist appareil)

À cocher sur **un device clair et un device sombre**, police défaut :

- [ ] Avatar profil : fond pêche/vert Poro, pas violet
- [ ] Notification non-lue : fond vert pâle / vert sombre, pas violet
- [ ] Fil : tabs lisibles sur clip blanc
- [ ] Fil : titre lisible sur clip blanc
- [ ] Shop home : carte find évidente, pas 4 boutons identiques
- [ ] PDP : prix plus grand que le titre
- [ ] Récap : « Paiement à la livraison » sans logo wallet
- [ ] Commande confirmée : chip « Confirmée », pas « Payée »
- [ ] Report sheet : 7 lignes, CTA off sans motif
- [ ] Légal : état indisponible honnête

## 11. Tests à exécuter avant de déclarer le redesign « intégré »

**Visuels** : checklist §10 + captures light/dark des 5 tabs.

**Fonctionnels** : DA-NR-01…07 + MX phase 2 + MOD + LEG.

**A11y** : DA-A11Y-01…05.

**Perf (smoke)** : fil 10 vidéos, une lecture, pas de jank évident après ajout du scrim haut (sujetif mais bloquant si drop visible).

## 12. Hors critères (ne pas échouer le lot)

- Beauté des illustrations empty (absentes).
- Parité pixel avec les PNG `mockups/`.
- Police custom (Q-10).
- Guest mode.
- Wave.
- Grille profil public.
- FCM.

Les maquettes `mockups/` sont une **direction**. L’acceptation se juge sur l’app Compose et ces critères, pas sur une superposition pixel.
