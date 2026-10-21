# Inventaire écrans et actions

Navigation : `Routes.kt`, `PoroNavHost.kt`. Cinq tabs : Accueil, Rechercher, Publier, Boutique, Profil. PoroTheme. Pas de second graphe.

## 1. Écrans complets

| Route | Fichier | Représentation | Accès actuel | Cible |
|---|---|---|---|---|
| Phone, Otp, Email*, Forgot, Reset | `ui/auth/` | Formulaires | Pub | + returnTo |
| EmailVerify | auth | Formulaire skippable | Auth | idem |
| Home | `FeedScreen` | Pager vertical | Auth | Visiteur = Tendances (B-FEED) |
| Video | `VideoScreen` | Player + overlay | Auth | Pub API |
| Search | `SearchScreen` | 3 onglets + suggest | Auth | Pub API |
| Notifications | `NotificationsScreen` | Liste | Auth | Prompt si visiteur |
| NotificationPreferences | prefs | Switches | Auth | Push labellisé |
| Upload | `UploadScreen` | Formulaire + % | Auth | Prompt si visiteur |
| PublicProfile | social | Header + follow | Auth | Pub API |
| Profile | `ProfileScreen` | me + mes vidéos | Auth | Écran invitation visiteur |
| EditProfile | edit | Form + avatar | Auth | — |
| Settings | settings | Sessions, logout, légal | Auth | — |
| Legal, LegalDocument | `ui/legal/` | Placeholder | Déconnecté OK | Textes Q-03 |
| DeleteAccount | profile | Preuve + confirm | Auth | — |
| ShopHome | shop | Handle + tuiles | Auth | Lecture + prompts |
| Shop | vitrine | Grille images | Auth | Pub API |
| Product | PDP | Pager images | Auth | Pub API |
| Cart | panier | Lignes | Auth | Prompt |
| Checkout | 2 étapes | Livraison / récap COD | Auth | Jamais auto-confirm |
| Orders / Order | listes/détail | Acheteur ou vendeur | Auth | — |
| MyShop | dashboard + open | Owner | Auth | Prompt |
| ProductEditor | form + photos | Owner | Auth | ImageOnly |

## 2. Actions hors écran plein

| Surface | Type | Fichier | API |
|---|---|---|---|
| Like, comment, share, ⋮ | Colonne overlay | `VideoActions` | social + report |
| Comments | Sheet 85 % | `CommentsSheet` | GET/POST/DELETE/like |
| Share | Sheet | `ShareSheet` | Intent ± POST shares |
| Report | Sheet | `ReportSheet` | POST reports |
| Overflow | Menu | `OverflowMenu` | report aujourd’hui |
| Upload status | Carte | `UploadScreen` | multipart |
| Cloche | IconButton | `FeedNotificationBell` | unread-count |
| Guest prompt | Sheet **cible** | — | aucun write |

## 3. Dialogues

| Dialogue | Où | API |
|---|---|---|
| Supprimer vidéo | Mes vidéos | DELETE /videos/{id} |
| Supprimer produit | Éditeur | DELETE product (`has_reservations`) |
| Annuler commande | Détail | POST cancel |
| Supprimer compte | DeleteAccount | POST me/delete |

## 4. Sans UI directe (volontaire)

Worker, outbox, sharp/FFmpeg, buckets, health, JWKS, erasure, staff cases, `DELETE /cart`, `DELETE logo`, listes likes/followers (P1/P2), devices FCM, PATCH comment (P1).

## 5. Représentation recommandée (si API prête, UI absente)

| Capacité | Meilleure UI | Pas |
|---|---|---|
| Followers / following | Écran liste P1 | Overlay dense |
| Éditer commentaire | Item menu 15 min | Nouvel écran |
| DELETE logo | TextButton dashboard | — |
| Carnet adresses | Écran Settings/checkout | GPS |
| Devices FCM | Invisible après login | Toggle mensonger |
| Staff | App séparée | Tab caché |
| Trending public | Même FeedScreen | Second fil WebView |

## 6. Interdits de représentation

- Wave / chat / favoris / catalogue / post image / save HLS / offline : **pas de bouton**.
- Staff : pas d’entrée.
- Créateur : chip **non** cliquable comme un droit publish.
- COD : pas « Payée ».
- Légal : pas de clauses inventées.
