# Critères d’acceptation testables

Scénarios pour QA et développeurs Android. Un critère est **échoué** si le message anglais brut du serveur s’affiche, si un succès est annoncé sans réponse `data`, ou si une action interdite passe uniquement parce que le bouton est visible.

Prérequis d’environnement : gateway `8000`, comptes de test CI/SN/CM/NG, **pas** de secrets dans les rapports.

---

## AC-AUTH

| ID | Scénario | Attendu |
|---|---|---|
| AC-AUTH-01 | Numéro CI valide, code correct | Session, `HomeRoute`, jetons absents des logs |
| AC-AUTH-02 | Numéro +33 | Refus local ou `phone_country_not_supported` |
| AC-AUTH-03 | 3 codes faux | `otp_too_many_attempts`, demande d’un nouveau code |
| AC-AUTH-04 | Renvoi avant 60 s | Bouton inactif / countdown |
| AC-AUTH-05 | Email + mot de passe < 8 | CTA inactif ou refus local |
| AC-AUTH-06 | Email déjà pris | `email_taken`, pas de session |
| AC-AUTH-07 | Forgot d’un email inconnu | 202, même copie que connu |
| AC-AUTH-08 | Reset | Login avec nouveau mdp ; anciennes sessions 401 |
| AC-AUTH-09 | Kill app, relance | Toujours connecté si refresh valide |
| AC-AUTH-10 | `revoke-others` depuis un autre appareil | Cet appareil → Phone + snackbar session |
| AC-AUTH-11 | Compte `SUSPENDED` | `account_blocked`, pas de fuite d’existence au-delà du contrat |
| AC-AUTH-12 | Skip vérif email | Home ; bannière profil présente |

## AC-FEED-VIDEO

| ID | Scénario | Attendu |
|---|---|---|
| AC-FEED-01 | Pour toi avec contenu | Pager, une lecture à la fois, boucle |
| AC-FEED-02 | Abonnements vide | Copie `feed_following_empty` |
| AC-FEED-03 | Airplane mode | `error_network` + Réessayer, pas de crash |
| AC-FEED-04 | Tap vidéo | Pause ; retap play ; icône play visible si pause |
| AC-FEED-05 | Background | Pause ; retour : reprise sauf pause user |
| AC-FEED-06 | Vidéo `processing` via VideoRoute | Texte traitement, pas de player |
| AC-FEED-07 | Vidéo retirée | `video_removed` ou 404 traduit |
| AC-FEED-08 | Scroll rapide | Pas de 2 players simultanés |

## AC-UPLOAD

| ID | Scénario | Attendu |
|---|---|---|
| AC-UPL-01 | MP4 < 3 min < 256 Mo + titre | Progression, puis processing, puis ready |
| AC-UPL-02 | Fichier 4 min | `upload_error_duration` avant l’API |
| AC-UPL-03 | AVI | `upload_error_format` |
| AC-UPL-04 | Titre vide | Publier inactif |
| AC-UPL-05 | Tuer l’app à 40 % | Envoi perdu (limite connue) ; pas d’état « en ligne » mensonger au relance |
| AC-UPL-06 | Delete depuis Mes vidéos | Dialogue ; disparition liste ; 404 public |

## AC-SOCIAL-SEARCH

| ID | Scénario | Attendu |
|---|---|---|
| AC-SOC-01 | Like | Compteur ±1 ; persisté après re-GET stats |
| AC-SOC-02 | Commentaire vide | Envoi impossible |
| AC-SOC-03 | Réponse à une réponse | Refus `comment_reply_depth` ou UI qui n’offre pas le 3e niveau |
| AC-SOC-04 | Delete commentaire d’autrui sans être owner | 403 traduit |
| AC-SOC-05 | Follow soi-même | Impossible |
| AC-SOC-06 | Share sans SHARE_BASE_URL | Sheet s’ouvre, texte sans URL, compteur shares +1 |
| AC-SRC-01 | `q` 1 caractère après debounce | Suggest ou search selon UI ; pas de crash |
| AC-SRC-02 | Aucun hit | `search_no_results` |
| AC-SRC-03 | Tap compte | PublicProfile du bon userId |

## AC-PROFILE-ACCOUNT

| ID | Scénario | Attendu |
|---|---|---|
| AC-PRF-01 | Username `admin` | `username_reserved` |
| AC-PRF-02 | Username pris | `username_taken` |
| AC-PRF-03 | Devenir créateur sans username | Bouton inactif / `username_required` |
| AC-PRF-04 | Devenir créateur avec username | Badge ; publication déjà possible avant |
| AC-PRF-05 | Bio 200 caractères | Aujourd’hui : risque `bio_invalid` — **échec de cohérence** à corriger (limite 160) |
| AC-PRF-06 | Avatar > 5 Mo | `avatar_too_large` |
| AC-DEL-01 | Delete sans case | CTA inactif |
| AC-DEL-02 | Delete + preuve | Déconnexion ; `deletion_scheduled_at` |
| AC-DEL-03 | Reconnexion J+1 | Snackbar annulation ; compte utilisable |

## AC-SHOP-ORDER

| ID | Scénario | Attendu |
|---|---|---|
| AC-SHP-01 | Handle 2 caractères | Ouverture boutique refusée |
| AC-SHP-02 | Pays CI | Prix FCFA sans décimales |
| AC-SHP-03 | Publier sans photo | `image_required` |
| AC-SHP-04 | Achat dans sa boutique | `own_shop` |
| AC-SHP-05 | Qty 0 | Stepper min 1 |
| AC-CART-01 | Article dépublié | Ligne unavailable, checkout bloqué tant que non retiré |
| AC-CHK-01 | Preview puis modifier le panier ailleurs | `preview_outdated`, nouveau récap, **pas** de commande |
| AC-CHK-02 | Confirm | Statut `confirmed` (COD), message « Commande passée », **pas** « Payée » |
| AC-CHK-03 | Back sur récap | Formulaire livraison conservé |
| AC-ORD-01 | Cancel avant ship | Stock libéré, statut cancelled |
| AC-ORD-02 | Cancel après ship | Action absente ou `order_status_conflict` |
| AC-ORD-03 | Ship vendeur | Statut shipped ; acheteur voit confirm-receipt |
| AC-ORD-04 | Confirm receipt | completed |
| AC-PAY-01 | Aucun sélecteur Wave | Seulement COD affiché |

## AC-NOTIF-MOD

| ID | Scénario | Attendu |
|---|---|---|
| AC-NTF-01 | Like reçu | Item liste ; tap → vidéo ; unread diminue |
| AC-NTF-02 | `orders=false` | In-app commande encore visible (contrat) |
| AC-NTF-03 | Switch push sans Firebase | PATCH ok ; **aucune** notification OS (limite connue) |
| AC-MOD-01 | *(après UI-MOD-001)* Report sa vidéo | `cannot_report_self` |
| AC-MOD-02 | Report motif `spam` | 201/200, confirmation discrète, pas de détail de file |

## AC-NAV-SESSION

| ID | Scénario | Attendu |
|---|---|---|
| AC-NAV-01 | Onglet Shop puis Accueil puis Shop | État / scroll restaurés |
| AC-NAV-02 | Logout | Phone ; back quitte l’app, pas un écran connecté |
| AC-NAV-03 | Deep link `/v/id` | Aujourd’hui : **non géré** (échec attendu jusqu’à implémentation) |

## AC-A11Y-PERF

| ID | Scénario | Attendu |
|---|---|---|
| AC-A11Y-01 | TalkBack sur Phone | Titre, champ, CTA, erreur annoncée |
| AC-A11Y-02 | TalkBack like | « J’aime, N » / « Retirer le j’aime » |
| AC-A11Y-03 | Cible nav | 5 onglets labellisés |
| AC-I18N-01 | Prix NGN | ₦ avec séparateurs ; kobo si besoin |
| AC-SEC-01 | Logcat debug | Pas de refresh token, pas de mot de passe, pas de code OTP |

---

## Matrices d’états par famille d’écrans

| État | Auth | Feed | Upload | Shop lecture | Checkout | Commande |
|---|---|---|---|---|---|---|
| Initial | formulaire vide | — | Idle | — | form loading | — |
| Chargement | bouton spinner | spinner noir | % + processing | Loadable | working | Loadable |
| Succès | navigation | pager | CTA voir | grille | placed snackbar | détail |
| Vide | — | copies dédiées | — | `shop_no_products` | — | `orders_empty` |
| Erreur métier | code FR | retry | code / format | shop_not_found | preview_outdated | status_conflict |
| Hors ligne | error_network | idem | idem | idem | ne pas marquer placé | retry GET |
| Session expirée | — | → Phone | → Phone | → Phone | → Phone | → Phone |
| Permission refusée | n/a | n/a | picker annulé = stay | n/a | n/a GPS | n/a |
| Contenu supprimé | — | 404 | — | product_not_found | — | order_not_found |
| Rate limit | countdown | message | message | message | message | message |
| Inconnu post-interruption | — | reload page | poll GET video | — | GET orders | GET order |

Paiement en ligne : **hors scope actuel**. Quand PAY-001 existera, ajouter : pending, confirmed-by-server, refused, cancelled, unknown (re-fetch). Interdiction d’utiliser le tap comme source de vérité.
