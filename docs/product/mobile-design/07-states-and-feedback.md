# Poro — États et feedback

Règle : chaque écran chargé a un chemin visible pour **chargement, vide, erreur, succès, hors-ligne, session morte, rate limit, contenu disparu**. Le paiement en ligne n’a pas d’état (service absent).

Composants : `LoadableContent`, `SubmitButton`, `ErrorMessageText`, `EmptyState` (à créer), Snackbar activité.

## 1. Langage d’état

| Nom | Quand | UI type |
|---|---|---|
| Initial | Premier frame | Fond thème / noir fil |
| Loading | Attente premier GET | Spinner nommé |
| Refreshing | Re-tap / retour panier | Spinner local ou pull — **pas de pull obligatoire** |
| Empty | 200 + liste 0 | `EmptyState` + copy existante |
| Content | Données | Hiérarchie écran |
| Working | POST/PUT en cours | CTA loading, pas de double tap |
| Success brief | Action courte | Snackbar 4 s |
| Success screen | Upload ready, checkout done | État d’écran + CTA suivant |
| Field error | Validation locale | supportingText |
| Business error | `error.code` | `ErrorMessageText` |
| Network | Timeout / IO | `error_network` + Réessayer |
| Rate limit | 429 | Countdown `Retry-After` |
| Gone | 404 / removed | Copy dédiée, pas un retry infini |
| Session | 401 refresh KO | Redirect Phone, graphe vidé |
| Unavailable legal | Texte non validé | « Document non encore validé. » |

## 2. Chargement

| Contexte | Pattern | Interdit |
|---|---|---|
| Session splash | Spinner centre | Splash marketing 3 s |
| Feed premier | Spinner blanc | Skeleton 10 cellules |
| Shop / produit / profil / order | `LoadableContent` | Écran blanc sans spinner |
| Liste page 2 | Spinner bas ou `Afficher plus` | Remplacer toute la liste |
| CTA | `SubmitButton` | Spinner + bouton encore cliquable |
| Upload | % + « Gardez l’application ouverte » | Barre indéterminée seule |
| Player (P1) | Petit spinner centre | Overlay opaque qui cache la miniature |

`contentDescription = Chargement…` obligatoire.

## 3. Vides

Réutiliser les strings. CTA seulement s’il mène à une **route réelle**.

| Surface | Copy | CTA proposé |
|---|---|---|
| Feed ForYou / Tendances | `feed_empty` | Aucun |
| Feed Abonnements | `feed_following_empty` | Rechercher → `SearchRoute` |
| Search hint | placeholder | Aucun |
| Search 0 | Aucun résultat | Aucun |
| Comments | `comments_empty` | Focus composer |
| Cart | `cart_empty` | Retour boutique (back) |
| Shop products | `shop_no_products` | Aucun (acheteur) |
| Orders | `orders_empty` | Aucun |
| Mes vidéos | Texte existant | Publier → `UploadRoute` |
| Notifications | Texte existant | Aucun |

Pas d’illustration P0.

## 4. Erreurs métier (codes déjà mappés)

Toujours `error.code` → string FR. Jamais `error.message`.

Familles à conserver / étendre :

| Famille | Exemples | UI |
|---|---|---|
| Auth | `otp_*`, `invalid_credentials`, `phone_country_not_supported` | Sous le formulaire |
| Compte | `account_blocked` | Message + pas de retry SMS |
| Social | like/follow réseau | Snackbar revert |
| Shop | `shop_not_found`, `own_shop`, `image_required` | Inline |
| Order | `preview_outdated`, `order_status_conflict`, `cart_unavailable` | Inline + retour d’étape |
| Report **Δ** | `cannot_report_self`, doublon 200 | Inline / snackbar non-erreur |
| Inattendu | + `request_id` | `ErrorMessageText` |

## 5. Succès

| Action | Feedback | Navigation |
|---|---|---|
| OTP / email login | Aucun toast | Home |
| `deletion_cancelled` | Snackbar | Home |
| Profil sauvé | `profile_saved` | Back |
| Like | Couleur | Reste |
| Comment posté | Ligne apparaît | Reste sheet |
| Share enregistré | Sheet dismiss | Reste |
| Report 201 | « Signalement envoyé. Notre équipe va l’examiner. » | Dismiss |
| Report 200 doublon | « Vous avez déjà signalé ce contenu. » | Dismiss |
| Produit ajouté | `product_added` | Reste PDP |
| Checkout | `checkout_placed` | `OrdersRoute` |
| Upload ready | Carte Finished + Voir | `VideoRoute` |
| Shop ouverte | Dashboard | Remplace le form |
| Commande annulée / reçue / expédiée | Détail rafraîchi | Reste |

Interdit : succès vert plein écran, confettis, « Paiement réussi ».

## 6. Hors ligne

Pas d’offline-first. GET échoue → retry manuel. Pas de file checkout. Upload : perte si process tué (P1 WorkManager). Légal embarqué validé : lisible hors ligne.

## 7. Rate limit

Auth / OTP / report : `SubmitButton` countdown. Ailleurs : message `rate_limited` + retry plus tard. Ne pas spammer l’API.

## 8. Contenu disparu

| Cas | UI |
|---|---|
| Vidéo removed / 404 | Texte, back |
| Boutique 404 | `shop_not_found` |
| Produit 404 | `product_not_found` |
| Profil 404 | Erreur + back |
| Commande 404 | Erreur + back |
| Ligne panier unavailable | Bandeau, bloque checkout |

## 9. Boutons pendant la requête

```
enabled && !working  → tap OK
working              → spinner, ignore tap
retryAfter > 0       → label « Réessayer dans Ns », enabled false
success              → navigate or snackbar
error                → enabled true, message visible
```

Appliquer à : OTP, login, find shop, add cart, preview, confirm, report, delete account, publish product, become creator.

## 10. Confirmation destructive

Toujours `AlertDialog` (ou écran Delete account) :

- Supprimer une vidéo
- Supprimer un produit
- Annuler une commande
- Révoquer une session (si déjà dialogue — conserver)
- Delete account = écran dédié

Dismiss / Annuler = aucun appel.

## 11. Mapping visuel couleur

| Feedback | Couleur |
|---|---|
| Info / unread | `secondaryContainer` |
| Succès durable | `secondary` texte |
| Erreur | `error` |
| Like | `PoroLike` |
| Warning boutique fermée | `onSurfaceVariant` + texte, pas un orange décoratif |

## 12. AC

1. Aucun écran liste sans empty.
2. Aucun POST sans working.
3. Aucun succès COD « payé ».
4. 401 → Phone, pas un empty.
5. Doublon report ≠ erreur rouge.
6. Request id seulement sur erreur inattendue.
