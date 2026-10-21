# Rôles et permissions réels

Sources : ADR-0001, JWT `roles[]`, guards shop/order/moderation, `PoroNavHost`.  
Les noms de rôles **ne suffisent pas** : chaque service revérifie.

## 1. Rôles JWT (auth)

| Rôle | Comment il est obtenu | Droit **effectif** côté API métier |
|---|---|---|
| `PERSONAL` | Création de compte | Socle. Upload vidéo **sans** CREATOR (ADR-0007) |
| `CREATOR` | `POST /users/me/creator` | Badge. **Pas** de droit publish supplémentaire |
| `BUSINESS` | Event `shop.created` puis refresh | Attribué **après** ouverture boutique. `POST /shops` ne l’exige pas |
| `ENTERPRISE` | Enum seulement | **Aucune** permission, aucun grant |
| `MODERATOR` / `ADMIN` | Attribution ops | ` /moderation/cases*`, ` /admin/shops/*` |
| `SUPPORT` | Enum | **Jamais contrôlé** |

Permissions fines (`shop:write`) : **non stockées** dans le token (ADR-0001). Dérivées dans le service (`owner_id`, `seller_id`, auteur commentaire).

## 2. Matrice d’acteurs (produit mobile)

| Action | Visiteur | Membre | Owner / auteur | Vendeur | Staff | Vérif serveur | Bouton visiteur |
|---|---|---|---|---|---|---|---|
| GET vidéo ready | API oui | oui | + autres statuts | — | — | Opt + statut | Pub (mur Android) |
| GET feed * | 401 | oui | — | — | — | JWT | B-FEED / prompt FY |
| Like / comment write / follow / share count / report | 401 | oui | — | — | — | JWT | **Visible → prompt** |
| GET comments / stats | API oui | oui | — | — | — | Opt | Pub |
| Publier vidéo | 401 | oui (PERSONAL) | — | — | — | JWT | Onglet → prompt |
| `GET /videos` liste | 401 | ses vidéos | — | — | — | owner | Profil membre |
| DELETE vidéo | 401 | non | oui | — | — | owner | Mes vidéos |
| PATCH commentaire | 401 | non | auteur 15 min | — | — | auteur | Cible P1 |
| DELETE commentaire | 401 | non | auteur **ou** owner vidéo | — | — | handler | Sheet |
| Profil public | API oui | oui | me = plus de champs | — | — | Pub vs Auth | Pub |
| `GET /users/me` | 401 | oui | — | — | — | JWT | **Ne pas appeler** |
| Search | API oui | oui | — | — | — | Opt + quota | Pub |
| Inbox notif | 401 | oui | — | — | — | JWT | Cloche → prompt |
| Vitrine / PDP | API oui | oui | — | owner CRUD | suspend | @Public vs me | Pub |
| Panier / checkout | 401 | acheteur | pas sa boutique (`own_shop`) | — | — | JWT + règles | Prompt |
| Commandes acheteur | 401 | buyer_id | — | — | — | JWT | Prompt |
| Ship / cancel vendeur | 401 | non | — | seller_id | — | JWT + shop | MyShop |
| File cases | 401/403 | 403 | — | — | oui | rôles MOD/ADMIN | **Absent** |
| Admin shop | 403 | 403 | 403 | 403 | oui | `MODERATION_ROLES` | **Absent** |

## 3. Règles « owner » (pas un rôle)

- Vidéo : `user_id` = `sub`.
- Boutique : `owner_id` = compte qui a `POST /shops` (une boutique / compte).
- Commentaire : auteur **ou** propriétaire de la vidéo.
- Commande acheteur : `buyer_id` ; vendeur : `seller_id`.
- Upload key image : préfixe `uploads/{ownerId}/`.

Le client **ne fait pas confiance** à l’UI pour ça. Un 403/404/409 s’affiche via `error.code`.

## 4. Données à masquer

| Donnée | Visiteur / tiers | Owner |
|---|---|---|
| email, téléphone, pays | Jamais (profil public sans) | me / auth |
| IP session | — | Settings (Q-09) |
| Contact livraison | — | Détail **vendeur** seulement |
| `hls_url` si removed | 404 | Owner sans URL média |
| Upload présigné | Jamais logué | Session d’envoi seulement |
| File modération | Jamais | Jamais dans cette app |

## 5. Refus — comportement UI

| Code / HTTP | UI |
|---|---|
| 401 sans session | Prompt connexion (cible guest) ; aujourd’hui Phone wall |
| 401 session morte | Snackbar + login ; cible : retour visiteur si abandon |
| 403 `shop_suspended` | Message vendeur, pas d’admin |
| 403 `forbidden` staff | Ne pas montrer l’entrée |
| 404 `*_not_found` / removed | Copy introuvable, pas retry infini |
| 409 conflict / reservations | Dialogue / message métier |
| 422 validation | Champ ou `ErrorMessageText` |
| 429 | `Retry-After`, pas login |

Le bouton visible n’autorise rien. Staff : **pas d’écran**, même si le JWT du testeur a `ADMIN`.
