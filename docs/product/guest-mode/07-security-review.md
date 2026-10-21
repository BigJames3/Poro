# Revue de sécurité — mode visiteur

Conception seulement. Ne pas affaiblir les contrôles serveur pour « faire passer » l’app.

## 1. Principes

1. Le client ne décide pas qu’une ressource est publique. Le service le fait.
2. Visiteur = absence de jeton, pas un JWT `GUEST` inventé.
3. Un Bearer invalide sur une route Opt vaut 401 : le client n’envoie **aucun** header.
4. Les buckets privés et la quarantaine restent inaccessibles (ADR-0015).
5. Staff hors de l’APK grand public.

## 2. Surfaces déjà sûres pour un GET anonyme

| Surface | Risque résiduel | Mitigation déjà là / à garder |
|---|---|---|
| HLS + thumbs `poro-videos` | URL devinable si ids connus | Pas de listing ; `removed` sans URL ; originaux privés |
| `GET /videos/{id}` | Enumération UUID | 404 uniforme si non ready / foreign |
| Search | Scraping | 60/min IP ; pas de PII dans les hits |
| Profil public | Énumération username | Champs déjà réduits (pas pays/tel/email) |
| Shop / produit | Énumération handle | closed/suspended = 404 |
| Comments GET | Volume | Rate social général ; pas d’e-mail auteur |

## 3. Ce que le visiteur ne doit jamais recevoir

- `GET /users/me`, `/auth/me`, sessions, IP des appareils
- Panier, adresses, commandes, contact livraison vendeur
- Inbox notifications, tokens FCM
- `upload_url`, `fields`, `upload_key`, sources vidéo
- File ` /moderation/cases`, ` /admin/shops/*`
- Pays / e-mail / téléphone d’autrui

Le client visiteur **n’appelle pas** ces routes. Un 401/403 n’est pas une excuse pour retry avec un header vide.

## 4. Feed public (B-FEED) — conditions de sécurité

Ouvrir `GET /feed/trending` sans JWT **uniquement si** :

1. Middleware Opt (ou absence de JWT acceptée).
2. Quota **par IP** du même ordre que search (60/min) + plafond gateway.
3. Un Bearer **invalide** → 401 (ne pas l’ignorer).
4. Items = `ready`+`approved` seulement (déjà le contrat feed).
5. Pas de PII auteur au-delà du profil public déjà exposé.
6. For You / Following **restent** Auth (mix et follows = identité).
7. ADR complément 0009 écrite et testée (trending 200 sans header ; For You 401).

Sans 2 : scraping trivial du catalogue vidéo. Sans 3 : un token volé mal formé « passe anonyme » et masque les bugs client.

## 5. Abus et coût

| Vecteur | Contrôle |
|---|---|
| HLS leech | Quota objet / CDN (PORO-029, futur). Hors guest v1. |
| Search flood | 60/min IP déjà |
| Report sans compte | Impossible (Auth) — prompt |
| Like flood | Auth + 60/min |
| SMS pumping via prompt | ADR-0017 déjà (cooldown, budget). Le prompt n’ajoute pas de route OTP. |
| Compte jetable pour like | Même qu’aujourd’hui |

## 6. Android

| Risque | Règle |
|---|---|
| Header `Authorization: Bearer ` vide | Interdit — interceptor actuel déjà OK |
| Refresh sur 401 anonyme | Interdit — authenticator déjà OK |
| `returnTo` qui rejoue un confirm | Interdit ([06](06-authentication-prompts.md)) |
| Logs d’OTP / mot de passe | Interdit (déjà) |
| Deep link vers `/admin` | Pas de route |
| WebView URL média | Interdit |
| Backup jetons | `allowBackup=false` déjà |

`LaunchedEffect` SignedOut → `goToSignIn` : aujourd’hui il **protège** par excès (auth-wall). Le relâcher sans B-FEED ouvre Accueil en 401 et des retries. Ordre : backend trending **puis** relâchement du mur, ou Home qui n’appelle trending tant que 401 connu — mais alors l’onglet Accueil est un leurre. Reco : ne pas relâcher le mur avant B-FEED.

## 7. Notifications

- Inbox = Auth. Pas de cache « dernières notifs » sur disque pour le visiteur.
- FCM absent : ne pas enregistrer de device, ne pas demander `POST_NOTIFICATIONS` au visiteur.
- Compteurs de likes sur une vidéo ≠ notification personnelle.

## 8. Marketplace

Visiteur : GET publics seulement. Un `productId` d’un draft n’est pas public (404). Ne pas précharger le panier « au cas où ». COD : aucune implication que c’est payé.

## 9. Résiduels acceptés

- Une URL HLS fuitée reste GET-able (déjà vrai pour un membre qui partage le fichier).
- Trending public révèle l’algo d’engagement 72 h (déjà visible à tout membre).
- Commentaires GET exposent `user_id` (déjà ; l’UI affiche « Membre Poro » si pas de username).

## 10. Non-objectifs

Contourner le JWT feed en agrégeant `GET /videos/{id}` depuis la search. Ce serait un fil maison, non contractuel, et un abus de quota. Interdit.
