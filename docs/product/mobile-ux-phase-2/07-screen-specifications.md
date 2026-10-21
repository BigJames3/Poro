# Spécifications d’écrans (phase 2)

Écrans **existants à stabiliser** et **nouveaux spécifiés**. Les parcours sociaux non listés restent tels quels (non-régression). Format unique par écran.

---

## UI-SHP-001 — Accueil boutique (existant)

| | |
|---|---|
| Objectif | Trouver une boutique par `@handle`, aller au panier / commandes / vente |
| Entrée / sortie | Onglet ; `ShopRoute`, `CartRoute`, `OrdersRoute`, `MyShopRoute` |
| Données | `handle`, `cartCount`, `hasShop` |
| Actions | find, nav |
| États | Idle ; finding ; erreur handle ; `hasShop` null = ne pas afficher « Ouvrir » comme certitude |
| Validation | ≥ 3 caractères |
| API | GET shop, cart, me |
| Accès | Connecté (invité : lecture handle **après** Q-02 ; panier/me → login) |
| A11y | Titres heading ; CTA 48 dp |
| AC | MX-01 ; hasShop réseau non trompeur |

## UI-SHP-002 — Vitrine (existant)

Loadable + grille + vide + retry + pagination. API publiques. Invité : OK lecture. A11y cartes : titre + prix.

## UI-SHP-003 — Fiche produit (existant)

| | |
|---|---|
| Actions | Variante, qty 1–99, add, ouvrir shop |
| Validation | Variante requise |
| API | GET product ; PUT cart **somme** (correctif) |
| Accès | Lecture pub ; add **Auth** |
| États | Loading, 404 `product_not_found`, rupture, adding, erreur own_shop |
| AC | MX-03/04 ; 2× add 1 → qty 2 |

## UI-SHP-004 — Panier (existant)

Serveur = source. `unavailable` bloque checkout. Hors ligne : retry, pas de cache métier. Retour recharge. AC MX-05/06.

## UI-SHP-005 — Checkout (existant, deux états d’un écran)

| | |
|---|---|
| Objectif | Livraison puis **récap à confirmer** ; COD seulement |
| Données | `DeliveryForm`, adresses, `PreviewDto`, `working` |
| Validation | Nom 1–80, tél `+\d`, ville 1–80, adresse 1–300 |
| API | prefill, addresses, preview, confirm |
| Succès confirm | Snackbar `checkout_placed` ; `orders[].status` typiquement `confirmed` ; **interdit** « Payée » |
| Réseau confirm | Retry idempotent / GET orders |
| Accès | Auth |
| A11y | Deux titres distincts livraison / récap |
| AC | MX-07/08/09 ; AC-CHK-* |

Hors ligne : pas de file d’attente checkout.

## UI-SHP-006/007 — Commandes (existant)

Liste + détail acheteur/vendeur. Ship / receive / cancel selon statut réel. `paid` / `awaiting_payment` : afficher seulement si l’API les envoie. Contact vendeur = données personnelles : écran détail, pas la liste.

## UI-SHP-008/009 — Ma boutique / éditeur (existant)

Pays/devise immuables. Photos max **10** (aligner UI). Publier exige photo. Delete + confirm. Suspendue : message, pas d’admin.

---

## UI-MOD-001 — ReportSheet (nouveau)

Voir [04](04-reporting-ux-spec.md). Pas de route. Auth. Motifs enum. Détail 0–500. 201 vs 200 doublon. Accès : pas staff queue.

## UI-LEG-001/002 — Légal (nouveau)

Voir [05](05-legal-screens-spec.md). Accessible déconnecté. Contenu « À valider » / indisponible. Pas de consentement bloquant tant que non exigé.

---

## UI-FEED-001 — Accueil visiteur (nouveau comportement, **bloqué**)

| | |
|---|---|
| Objectif | Fil public = Tendances |
| API | `GET /feed/trending` **aujourd’hui 401** |
| Règle | Ne pas coder tant que [03](03-guest-mode-feasibility.md) §8 n’est pas livré côté backend |
| Actions protégées | Like/comment/follow/share/report → login + returnTo |

Connecté : ne pas retirer Pour toi / Abonnements.

## UI-AUTH-G1 — Challenge login (évolution)

Objectif : depuis une action protégée, ouvrir le graphe auth **sans perdre** la destination (vidéo, produit, report). Aujourd’hui `goToSignIn()` vide tout le graphe. Spec : `SavedState` / argument `returnRoute` **À confirmer à l’implémentation**, sans casser logout → Phone.

---

## Hors ligne / reprise

| Parcours | Stratégie |
|---|---|
| Lecture feed/shop GET | Retry manuel, pas d’offline-first |
| Panier | Re-GET à l’entrée |
| Preview | TTL 15 min serveur |
| Confirm | Idempotence preview_id |
| Upload | P1 WorkManager (lot 5), pas phase 2 lots 1–3 |
| Légal embarqué | Disponible hors ligne une fois validé et packagé |

## Accessibilité transversale

`ErrorMessageText` liveRegion ; spinner « Chargement… » ; destructive = `AlertDialog` existant (delete, cancel order). Report et légal : mêmes patterns.
