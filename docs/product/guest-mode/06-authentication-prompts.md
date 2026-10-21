# Prompts d’authentification

Un clic protégé = une feuille claire, puis le graphe auth **existant** (téléphone / e-mail). Pas de nouvel OTP, pas de login social tiers.

## 1. Contrat du prompt

| Champ | Règle |
|---|---|
| Déclencheur | Tap d’une action Auth, ou tab Pour toi / Abonnements / Publier / cloche / panier |
| Ton | Une phrase, pourquoi **cette** action. Pas « Rejoignez des millions de Poristes ». |
| Sortie | Continuer téléphone, Continuer e-mail, Plus tard |
| Auth | `authGraph` actuel ; `device_name` à l’verify / login |
| Annulation | `pop` ; aucune écriture |
| Succès | Restaurer `returnTo` ; replay seulement si [03](03-guest-permissions-matrix.md) le permet |
| Légal | Liens déjà branchés |

Strings cibles (nouvelles clés, FR) :

| Clé | Texte |
|---|---|
| `guest_why_like` | Connectez-vous pour aimer cette vidéo. |
| `guest_why_comment` | Connectez-vous pour commenter. |
| `guest_why_follow` | Connectez-vous pour suivre ce compte. |
| `guest_why_share_count` | (si on choisit de compter) Connectez-vous pour partager. |
| `guest_why_report` | Connectez-vous pour signaler ce contenu. |
| `guest_why_publish` | Connectez-vous pour publier. |
| `guest_why_foryou` | Connectez-vous pour un fil personnalisé. |
| `guest_why_following` | Connectez-vous pour voir vos abonnements. |
| `guest_why_notify` | Connectez-vous pour voir vos notifications. |
| `guest_why_cart` | Connectez-vous pour ajouter au panier. |
| `guest_why_orders` | Connectez-vous pour voir vos commandes. |
| `guest_why_shop` | Connectez-vous pour gérer votre boutique. |
| `guest_why_profile` | Connectez-vous pour gérer votre profil. |
| `guest_continue_phone` | Continuer avec le téléphone |
| `guest_continue_email` | Continuer avec l’e-mail |
| `guest_later` | Plus tard |

Share **lien** : pas de prompt si on n’appelle pas `POST /shares`. Intent local + URL = Pub.

## 2. `returnTo`

Structure indicative (SavedState / args) :

```
route: Home | Video | PublicProfile | Product | Shop | Search | Upload | Notifications | …
id / handle / query
feedKind: trending | (après login : forYou possible)
pending: like | follow | none
pendingTargetId
```

`goToSignIn()` actuel (`popUpTo` graphe, `PhoneRoute`) **reste** pour : logout explicite depuis Settings, compte bloqué, suppression. Il **ne sert plus** au challenge visiteur.

`goHome()` après login depuis un challenge : **ne pas** l’utiliser. Il casse le retour.

## 3. Replay

| Action | Replay auto | Condition |
|---|---|---|
| Like / unlike | Oui | Idempotent ; 404 → snackbar, pas de 2ᵉ prompt |
| Follow / unfollow | Oui | Idempotent |
| Commentaire | Non | Rouvrir sheet + texte conservé (SavedState) ; l’utilisateur envoie |
| Report | Non | Rouvrir `ReportSheet` |
| Share count | Optionnel une fois | Seulement si l’Intent a déjà été fait ou si on compte au login |
| Add to cart | Non | Retour PDP, tap à nouveau |
| Checkout confirm | **Interdit** | Conséquence commande |
| Upload | **Interdit** | Fichier / titre non mémorisés hors session |
| Prefs / delete account | Interdit | — |

## 4. Erreurs pendant le challenge

| Cas | UI |
|---|---|
| OTP / mot de passe faux | Copy auth actuelle, rester sur l’écran |
| `account_blocked` | Message, pas de retry SMS (déjà) |
| Réseau auth | Réessayer sur l’écran auth ; back = contenu public |
| Utilisateur tue l’app au milieu | Au relancement : visiteur, pas de pending rejoué |
| Login OK mais `returnTo` perdu | Home membre (Pour toi) — repli acceptable |

## 5. Continuité vs session morte

Deux portes :

1. **Challenge visiteur** : modal + auth empilé + retour contenu.
2. **Session membre morte** : snackbar `session_expired` + même modal/auth ; si abandon → état visiteur, ** Accueil Tendances** (après B-FEED), pas un Phone plein écran inescapable.

Ne pas rejouer un like d’avant l’expiration sans tap.

## 6. Ce qu’on ne fait pas

- Prefetch `/auth/me` sous le prompt.
- Stocker le mot de passe / OTP pour replay.
- Deep link login qui saute le « pourquoi ».
- Captcha inventé.
- Forcer l’e-mail si l’utilisateur a choisi le téléphone.
