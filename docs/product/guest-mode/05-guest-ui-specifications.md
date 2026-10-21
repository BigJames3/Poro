# Spécifications d’écrans — visiteur

PoroTheme uniquement. Chrome du fil : [mobile-design/05](../mobile-design/05-social-feed-design.md). Cette page ne décrit que les **écarts visiteur**.

Aucun Kotlin dans cette phase.

## 1. Accueil visiteur

Même page immersive : vidéo (ou image de post si contrat) pleine surface, scrims, tabs overlay.

| Élément | Visiteur |
|---|---|
| Tabs | Tendances **sélectionné**. Pour toi et Abonnements **visibles**, non « disabled » gris morts : tap → prompt (fil personnalisé). |
| Indicateur | Underline orange sur Tendances. |
| Cloche | Icône visible, **pas de badge**. Tap → prompt notifications. |
| Overlay auteur / titre / # | Identique |
| Colonne ♥ 💬 ↗ ⋮ | Identique, 48 dp, compteurs publics |
| Follow overlay | Toujours pas (densité) — follow au profil |
| Empty Tendances | `feed_empty` + pas de CTA login obligatoire |
| Empty For You / Following | N/A (pas chargés) |
| Erreur trending | `ErrorMessageText` + Réessayer (pas « Connectez-vous ») |
| Player | Une page settled = un play ; tap pause ; miniature Coil ; P1 HLS fail déjà prévu |

Image sociale : page sans triangle play ([mobile-media/04](../mobile-media/04-social-media-spec.md)). Tant que B-IMG-POST : clips seulement.

Petit écran : mêmes règles que le design fil (< 360 dp, tabs scroll).

## 2. Prompt de connexion (composant unique)

Feuille (`ModalBottomSheet`) 50–70 %, pas un dialog bloquant opaque.

```
Titre          {raison courte}
Corps          Une ligne. Pas de marketing.
[ Continuer avec le téléphone ]   primary 48 dp
[ Continuer avec l’e-mail ]       outlined
[ Plus tard ]                     text — dismiss
Légal          liens déjà existants (indisponible / À valider)
```

Raisons : [06](06-authentication-prompts.md). Une instance, paramètres `reason` + `returnTo`.

Après « Continuer » : empiler le graphe auth **sans** `popUpTo` du graphe entier. Back depuis Phone = dismiss prompt + contenu.

Succès : `goHome()` actuel **interdit** ici (il détruit la pile). Naviguer `returnTo` ou `pop` jusqu’à l’écran d’origine, puis replay éventuel.

## 3. Onglet Profil visiteur

Pas un `ProfileScreen` vide qui spinner `/users/me`.

```
TopBar « Profil »
Carte
  Rejoindre Poro
  Aimez, commentez et publiez en un compte.
  [ Se connecter ]  primary
  [ Créer un compte ]
Groupe À propos
  Documents légaux (routes déjà déconnectées)
  Version
```

Pas de chip créateur, pas de mes vidéos, pas de sessions.

## 4. Publier (tap onglet)

Ne pas montrer le formulaire d’upload. Soit naviguer vers `UploadRoute` qui, en visiteur, n’affiche **que** le prompt (recommandé : prompt immédiat au tap onglet, rester sur l’onglet précédent visuellement). Éviter un écran upload trompeur (« Choisir une vidéo » puis échec).

## 5. Recherche

Inchangée. Empty / 0 résultats : copies actuelles, pas de login. 429 : throttle.

## 6. Boutique / PDP

Hub, vitrine, PDP : lecture. CTA « Ajouter au panier » **reste enabled** visiteur (pas `enabled=false`). Tap → prompt commande.

Icône panier : visible, badge **absent** (pas de GET cart). Tap → prompt.

« Mes commandes » / « Ma boutique » : visibles comme aujourd’hui, tap → prompt (raison dédiée).

`hasShop` : ne pas appeler `/shops/me` en visiteur ; label neutre « Ma boutique ».

## 7. Commentaires

Sheet identique. Composer visible, placeholder « Connectez-vous pour commenter » ; focus → prompt. Liste déjà là.

## 8. Notifications

Pas d’écran liste visiteur. Prompt seulement. Après login : écran actuel.

## 9. Légal

Inchangé, déjà hors session (`isSignInScreen`).

## 10. Accessibilité

- Tabs Pour toi / Abonnements : `contentDescription` « Connectez-vous pour un fil personnalisé » en visiteur.
- Like : « J’aime, 1,2 k. Connexion requise » (visiteur) vs état liked (membre).
- Cloche : « Notifications. Connexion requise » ; pas « 0 non lues ».
- Prompt : titre = heading ; Fermer nommé ; focus initial sur le titre.
- Contraste overlay : scrim 72 % inchangé.

## 11. États session

| État | UI |
|---|---|
| Unknown | Spinner splash (déjà) |
| SignedOut | Graphes publics + prompts |
| SignedIn | App actuelle |
| SignedOut pendant un écran public (logout) | Rester / revenir Tendances, snackbar optionnel « Déconnecté » |
| Session expirée | Snackbar existant + prompt ; fermer → visiteur, contenu public |

## 12. Interdits visuels

- Bannière permanente « Connectez-vous » qui mange 80 dp du clip.
- Boutons like/comment en `enabled=false` sans explication.
- Illustration onboarding 5 pages avant le fil.
- WebView TikTok.
- Second thème.
