# Parcours visiteur

Tous les parcours ci-dessous sont **cibles**. Aujourd’hui le parcours réel est : splash → `PhoneRoute`. Les parcours 1–4 exigent B-FEED + lot Android G1.

## 1. Première ouverture — regarder

```
Splash (Session Unknown, quelques dizaines de ms)
  → SignedOut
  → HomeRoute, onglet Tendances
  → GET /feed/trending (sans Authorization)
  → Pager vertical, autoplay de la page settled, une décode à la fois
```

Arrêt : pause tap ; app en arrière-plan (déjà). Pas de splash marketing. Pas de wall « Créez un compte pour continuer » au-dessus du premier clip.

Erreur trending (réseau) : message + Réessayer. 401 après B-FEED : bug backend, pas un prompt login.

## 2. Lire les commentaires sans compte

```
Tap 💬 → sheet 85 %
  → GET /videos/{id}/comments (Opt, sans header)
  → Liste, réponses 1 niveau
  → Tap champ « Écrire un commentaire » → Prompt (commenter)
  → Fermer le prompt → sheet et vidéo toujours là
```

Ne pas vider le pager.

## 3. Like → compte → retour

```
Tap ♥ → Prompt « Connectez-vous pour aimer »
  → Parcours auth existant (téléphone ou email)
  → Succès → returnTo = Home + videoId + kind=trending
  → Replay PUT like (idempotent)
  → Cœur plein + compteur +1 si 200
```

Annuler le prompt : rien n’est envoyé, compteur inchangé.

## 4. Profil public → follow

```
Tap auteur → PublicProfile (GET by-username)
  → Bio, avatar, compteurs si branchés
  → Tap Suivre → Prompt
  → Login → retour profil → PUT follow
```

Pas de grille de médias (API absente).

## 5. Recherche publique

```
Onglet Rechercher → saisie
  → GET /search?type=videos|users|hashtags (quota IP)
  → Tap résultat vidéo → VideoRoute (GET public)
  → Like sur cette vidéo → Prompt + returnTo VideoRoute
```

Cold start search : OK **même avant** B-FEED, mais seulement si le mur Android (`goToSignIn` sur SignedOut) est levé. Sans trending, ce n’est **pas** le mode visiteur promis (pas de fil). Ne pas livrer « Search only » comme v1.

## 6. Boutique

```
Onglet Boutique → saisie @handle
  → GET /shops/{handle} + products
  → Fiche produit, images, prix, variantes
  → Ajouter au panier → Prompt « Connectez-vous pour commander »
  → Login → returnTo ProductRoute
  → L’utilisateur retape « Ajouter » (pas de PUT silencieux)
```

Checkout / confirm : jamais rejoués. COD : succès = « Commande passée », pas « Payée ».

Handle 404 : introuvable, pas login.

## 7. Publier

```
Onglet + → Prompt « Connectez-vous pour publier »
  → Login → UploadRoute (formulaire vide, pas un fichier en attente)
```

Ne pas mémoriser un URI vidéo visiteur.

## 8. Cloche

```
Tap cloche → Prompt « Connectez-vous pour voir vos notifications »
  → Login → NotificationsRoute → GET réel
```

Visiteur : pas de badge. Pas de liste placeholder « 3 nouvelles ».

## 9. Fermer le login / échec auth

```
Prompt ouvert → Retour / Fermer
  → Contenu public inchangé (pager, recherche, PDP)
```

OTP invalide, réseau auth : restent sur le parcours auth. **Ne pas** éjecter vers un wall plein écran qui empêche de revenir au clip.

Session expirée **en membre** : message `session_expired` + login. Après fermeture sans reconnecter : **revenir visiteur** sur le dernier contenu public (Tendances / PDP), pas un blocage. Aujourd’hui `goToSignIn` vide la pile — à changer (G1).

## 10. Membre déjà connecté (non-régression)

Splash → Home Pour toi. Trois onglets de fil. Like/comment/share/report/panier inchangés. Logout → aujourd’hui Phone ; cible : visiteur sur Tendances (après B-FEED), pas un piège.

## 11. Interdits dans les parcours

- Envoyer un Bearer vide.
- Appeler `/users/me`, `/cart`, `/notifications` au démarrage visiteur.
- Afficher Pour toi avec un mix inventé côté client.
- Présenter une miniature ou une photo boutique comme un post image social.
- Auto-confirm checkout après login.
