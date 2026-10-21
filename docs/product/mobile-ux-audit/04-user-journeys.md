# Parcours utilisateurs

Parcours fondés sur le code Android et les API. Les étapes « le produit devrait » sont séparées et marquées **Recommandation** ou **À confirmer**.

Convention : point d’entrée → écrans → succès / échec / réseau / retour.

---

## PJ-AUTH-OTP — Se connecter avec un numéro

1. Lancement : jetons absents → `PhoneRoute`.
2. L’utilisateur choisit le pays (CI par défaut) et saisit le numéro national. Validation locale `PhoneNumber.parse`.
3. `POST /otp/request`. Succès → `OtpRoute(phone, expiresInSeconds)`.
4. Saisie du code 6 chiffres (autofill SMS si disponible). `POST /otp/verify` + `device_name`.
5. Jetons chiffrés. `goHome()`. Si `deletion_cancelled` : snackbar d’annulation de suppression.

**Échecs**

| Cas | Code | Comportement UI |
|---|---|---|
| Pays hors lancement | `phone_country_not_supported` | Message dédié |
| Numéro injoignable | `phone_unreachable` | Vérifier le numéro |
| Throttle / budget SMS | `otp_throttled`, `429`, `sms_unavailable` | Countdown `Retry-After` ; proposition email si SMS down |
| Code faux / expiré / trop d’essais | `otp_invalid`, `otp_expired`, `otp_too_many_attempts` | Nouveau code si besoin |
| Compte suspendu | `account_blocked` | Support |
| Réseau | `NetworkError` | `error_network` + réessayer |

**Annulation** : retour système sur Phone ; « Modifier le numéro » depuis OTP. Pas d’inscription distincte : le premier verify **crée** le compte.

**Critères** : un numéro CI valide reçoit un code ; un +33 est refusé avant ou par l’API ; un code faux n’ouvre pas de session.

---

## PJ-AUTH-EMAIL — Créer un compte email

1. Depuis Phone → « Se connecter avec un email » → Login → « Créer un compte ».
2. Nom, email, mot de passe. Validations `AccountRules`.
3. `POST /email/register` → session + `EmailVerifyRoute(codeAlreadySent=true)` (pile auth effacée).
4. Vérifier, renvoyer, ou « Plus tard » → Home.

**Échecs** : `email_taken` ; throttle ; mot de passe hors 8–128. Le `full_name` disparaît : le profil s’ouvre sans nom affiché (**lacune**).

---

## PJ-AUTH-RESET — Mot de passe oublié

1. Login → mot de passe oublié (email prérempli).
2. `POST /password/forgot` toujours 202.
3. Reset : code + nouveau mot de passe → Login avec message de succès.
4. Toutes les autres sessions sont mortes.

Ne jamais afficher « aucun compte pour cet email ».

---

## PJ-FEED — Consommer le fil

1. Home après session. Onglet **Pour toi** par défaut.
2. `GET /feed/for-you` page 10. Pager vertical : une vidéo visible lue en HLS, les autres en miniature.
3. Tap : pause / reprise. Overlay : auteur, titre, like, commentaires, partage. Cloche en haut à droite.
4. Changer d’onglet (Abonnements / Tendances) recharge ce fil. Re-tap sur l’onglet courant : refresh.
5. Fin de liste : prefetch curseur.

**Vides** : Pour toi / Tendances « Aucune vidéo… » ; Abonnements « Abonnez-vous à des créateurs… ».

**Échecs** : `invalid_cursor` → Android recommence sans curseur ; réseau → retry.

**Retour** : back depuis Home quitte l’application (pile racine).

**Hors ligne** : erreur réseau, pas de cache local des pages.

---

## PJ-VIDEO-OPEN — Ouvrir une vidéo hors fil

Entrées : recherche, notification, « Voir la vidéo » après upload, (futurs liens `/v/{id}` non branchés).

`VideoRoute` → `GET /videos/{id}`.

| Statut | UI |
|---|---|
| `ready` + approved | Lecteur + actions |
| `processing` | Texte d’attente, pas de player |
| `failed` | Échec de traitement |
| `removed` / 404 | Retirée ou introuvable |

---

## PJ-UPLOAD — Publier une vidéo

1. Onglet Publier.
2. Photo Picker (pas de caméra in-app, pas de permission stockage).
3. Contrôles locaux : MIME, 256 Mo, 3 min.
4. Titre obligatoire 1–100 ; description / hashtags facultatifs.
5. Publier → init upload → PUT parties (retry par partie) → complete → poll jusqu’à `ready`/`failed`.
6. Message « Gardez l’application ouverte ». Succès : CTA « Voir la vidéo ».

**Échecs** : format, taille, durée, `invalid_media`, `payload_too_large`, réseau milieu d’upload (reprise partie, pas fichier entier), process tué (**perte**).

**Annulation** : quitter l’écran pendant l’envoi — **À confirmer** le comportement exact d’abort UI ; l’API `DELETE /uploads/{id}` existe.

**Permissions système** : aucune au-delà d’INTERNET. Pas d’enregistrement caméra.

---

## PJ-SOCIAL-LIKE-COMMENT-SHARE

Depuis overlay vidéo :

- Like : `PUT/DELETE .../like`, compteur immédiat (optimistic **À confirmer** dans le ViewModel — les tests existent).
- Commentaires : bottom sheet, liste, répondre (1 niveau), liker, supprimer (auteur ou propriétaire). **Pas d’édition** malgré l’API 15 min.
- Partage : WhatsApp ou autre ; `POST /shares`. Sans `SHARE_BASE_URL`, texte titre seulement.

Rate limit 429 + Retry-After : bouton submit compte à rebours (`SubmitButton` auth ; social **À confirmer** le countdown vs message générique).

---

## PJ-FOLLOW — S’abonner

1. Tap auteur → `PublicProfileRoute`.
2. `GET` profil + `GET /users/{id}/stats`.
3. S’abonner / se désabonner. Compteurs mis à jour.
4. Le fil Abonnements se peuplera au prochain chargement (cache 60 s page 1).

Pas de liste d’abonnés. Impossible de s’abonner à soi-même.

---

## PJ-SEARCH

1. Onglet Rechercher. Placeholder « Vidéos, comptes, #hashtags ».
2. Suggestions pendant la saisie.
3. Soumission : onglet Vidéos par défaut, puis Comptes / Hashtags.
4. Tap vidéo → `VideoRoute` ; tap compte → profil public ; tap hashtag → relance une recherche (pas de feed hashtag dédié).

Vide initial = hint ; vide résultats = « Aucun résultat. »

---

## PJ-NOTIF

1. Cloche (badge unread) → liste. Resume : rafraîchit le compteur.
2. Tap : mark read + navigation (vidéo, profil, commande acheteur/vendeur via `data.role`).
3. Tout lire. Préférences depuis Paramètres.

Push réel : **non parcouru** (Firebase absent). Le switch « Notifications sur le téléphone » est un drapeau serveur inopérant sans jeton.

---

## PJ-PROFILE — Gérer son profil

1. Onglet Profil : `GET /users/me` + `GET /auth/me`.
2. Sans username : incitation. CTA créateur désactivé.
3. Modifier : username (disponibilité live), nom, bio, avatar.
4. Vérifier l’email si `needsEmailVerification`.
5. Mes vidéos + compteurs d’abonnés (composants extra).

**Écart** : une bio de 200 caractères passe l’UI et échoue en `bio_invalid` (limite serveur 160).

---

## PJ-CREATOR — Devenir créateur

1. Username enregistré.
2. « Devenir créateur » → `POST /me/creator`.
3. Badge affiché. Le JWT `CREATOR` arrive au refresh suivant — l’UI n’attend pas le rôle.
4. **Aucun droit nouveau** pour publier.

---

## PJ-SETTINGS — Sessions et sortie

Paramètres : liste d’appareils, déconnecter un appareil, les autres, préférences notif, logout, supprimer le compte.

Logout → Phone. Session révoquée ailleurs → prochaine API 401 → refresh fail → SignedOut.

---

## PJ-DELETE — Supprimer le compte

1. Explication J+30 + case à cocher.
2. Preuve : mot de passe ou OTP masqué (`PhoneNumber.masked`).
3. `POST /me/delete` → jetons effacés → Phone.
4. Reconnexion avant J+30 : snackbar d’annulation. `POST /me/delete/cancel` existe, **pas d’écran dédié** une fois déconnecté (la reconnexion suffit).

Irréversible après purge. Pas de restauration support.

---

## PJ-SHOP-OPEN — Ouvrir une boutique

1. Boutique → « Ouvrir ma boutique ».
2. Nom, handle (définitif), pays (devise définitive), description facultative.
3. `POST /shops` → tableau de bord. BUSINESS au prochain refresh (non lu par l’UI).

409 `shop_exists` / `handle_taken`. Pays hors CI/SN/CM/NG : non proposés.

---

## PJ-SHOP-PRODUCT — Publier un produit

1. Ma boutique → nouveau produit.
2. Nom, description, ≥1 variante (titre, prix, stock), ≥1 photo pour publier.
3. Brouillon enregistrable ; publier ; archiver ; supprimer (bloqué si réservations).

Prix : XOF/XAF entiers FCFA ; NGN kobo via `Formats.parseMoney`.

---

## PJ-BUY — Acheter (paiement à la livraison)

1. Connaître le `@handle` → Shop home → vitrine → fiche.
2. Variante + quantité 1–99 → panier (`PUT /cart/items/{variantId}`).
3. Panier : lignes par boutique, sous-total, articles indisponibles à retirer, notice COD.
4. Commander → livraison (préremplie, adresses sauvées, case « enregistrer »). **Pas de GPS.**
5. Continuer → preview 15 min. Vérifier total, adresse, « Paiement à la livraison ».
6. Confirmer explicitement (`confirmed: true`). Une commande **par boutique**.
7. Succès : « Commande passée. Le vendeur va la préparer. » → liste commandes (preview retiré de la pile).

**Échecs** : `own_shop`, `cart_full`, `variant_unavailable`, `preview_outdated` (recharger le récap), `shop_suspended`, réseau après confirm : **ne pas** réafficher un succès ; réessayer confirm (idempotent) ou aller aux commandes pour vérifier.

**Annulation checkout** : back sur le récap revient à la livraison (`BackHandler`), pas au panier.

---

## PJ-ORDER-BUYER / SELLER

Acheteur : suivre, annuler avant expédition (dialogue), confirmer réception si `shipped`. Sans confirmation : `completed` automatique à 7 jours (serveur).

Vendeur : commandes reçues, marquer expédiée (tracking facultatif ≤200), annuler avant ship. Contact acheteur visible côté vendeur (API). Contacts effacés 90 jours après la fin.

**Pas de litige, pas de retour, pas d’annulation post-expédition (PORO-036).**

---

## PJ-MOD-REPORT — Signaler (API sans UI)

**Parcours non pris en charge par Android.** L’API existe. Recommandation d’écran : [05](05-mobile-screen-inventory.md) UI-MOD-001. Ne pas inventer de motifs hors contrat.

---

## PJ-GUEST / PJ-CHAT / PJ-PAY-WAVE

Absents. Un lien de partage collé dans le navigateur n’ouvre pas l’app (pas d’App Links). Wave : pas de parcours à décrire comme réel.

---

## Interruptions transverses

| Événement | Comportement constaté |
|---|---|
| App en arrière-plan | Vidéo en pause ; reprise sauf pause utilisateur |
| Process tué pendant upload | Envoi perdu |
| 401 métier | Refresh unique puis replay ; échec → login |
| 429 | Message + éventuellement `Retry-After` |
| Compte bloqué | Impossible de rafraîchir / agir |
| Contenu retiré pendant lecture | Prochaine ouverture : 404 / « retirée » |
| Preview checkout expiré | Nouveau récap, pas de commande fantôme |
