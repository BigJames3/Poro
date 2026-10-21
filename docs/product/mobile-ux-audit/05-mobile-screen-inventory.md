# Inventaire des écrans mobiles

Deux ensembles : **existants** (composables routés ou embarqués dans `apps/mobile-android`) et **nécessaires** aux fonctionnalités réellement identifiées mais sans UI.

Priorités : justification dans [11-prioritized-roadmap.md](11-prioritized-roadmap.md). P0 = parcours de base déjà supportés par l’API et l’app, plus les trous bloquants stores/sécurité. Ne pas créer un écran distinct pour un simple état.

**Compte** : 27 destinations typées + 1 splash + 3 feuilles embarquées existantes. 6 écrans à créer pour des API déjà présentes ou des obligations stores. Les parcours sans backend (chat, Wave, Enterprise) n’ont **pas** d’écran prescrit ici.

---

## A. Authentification

### UI-AUTH-000 — Démarrage session (`LoadingScreen`)

| | |
|---|---|
| Objectif | Attendre la lecture des jetons |
| Fichier | `PoroNavHost.kt` (privé) |
| Utilisateurs | Tous |
| Structure | Indicateur centré, fond thème |
| États | Unique : chargement |
| Priorité | P0 existant |

### UI-AUTH-001 — Connexion téléphone (`PhoneEntryScreen`)

Point d’entrée start si déconnecté. Pays + numéro, CTA SMS, lien email. Validation locale. Sortie : OTP ou Email login.

### UI-AUTH-002 — Code SMS (`OtpVerifyScreen`)

6 cases, expiration, renvoi, changer de numéro. Succès → Home.

### UI-AUTH-003 — Connexion email (`EmailLoginScreen`)

Email, mot de passe, forgot, créer un compte, retour téléphone.

### UI-AUTH-004 — Inscription email (`EmailRegisterScreen`)

Nom, email, mot de passe. Sortie : UI-AUTH-005.

### UI-AUTH-005 — Vérification email (`EmailVerifyScreen`)

**Authentifié.** Skip autorisé. Aussi depuis le profil.

### UI-AUTH-006 — Mot de passe oublié (`ForgotPasswordScreen`)

### UI-AUTH-007 — Nouveau mot de passe (`ResetPasswordScreen`)

Tous : `ui/auth/**`, composant `AuthForm` + `SubmitButton`. États : idle, sending, error (code traduit), retry countdown. Priorité **P0 existant**.

---

## B. Accueil et vidéo

### UI-FEED-001 — Fil (`FeedScreen` / `HomeRoute`)

| | |
|---|---|
| Objectif | Consommer Pour toi / Abonnements / Tendances |
| Structure | Fond noir plein écran, pager vertical, onglets haut, cloche, overlay auteur + titre + actions |
| Composants | `VideoPage`, `VideoSurface`, `Avatar`, `SocialVideoActions`, `FeedNotificationBell` |
| Données | `FeedItem` : video_id, title, hashtags, duration, urls, stats, author |
| Actions | Changer de fil, play/pause, ouvrir auteur, like, commentaires, partage |
| États | Loading, empty (2 copies), error+retry, items |
| API | `GET /feed/{for-you,following,trending}` + stats social |
| Priorité | P0 existant |

### UI-FEED-002 — Vidéo isolée (`VideoScreen` / `VideoRoute`)

Même lecteur. États processing / failed / removed. Entrée recherche, notif, upload.

### UI-FEED-003 — Contrôles lecture avancés

**À créer (état, pas nouvel écran)** : mute, indicateur de chargement média, échec HLS distinct, barre de progression optionnelle. Aujourd’hui : tap play/pause seulement. Priorité **P1**.

---

## C. Création

### UI-UPL-001 — Publier (`UploadScreen` / `UploadRoute`)

Picker, titre, description, règles 3 min / 256 Mo, progression %, consigne garder l’app ouverte, succès + CTA. États Idle / Uploading / Processing / Finished / Failed.

### UI-UPL-002 — Mes vidéos

**Pas une route** : `MyVideosSection` sur le profil. Grille, statut, suppression confirmée, pagination.

Pas d’écran caméra / enregistrement in-app (absent, non prescrit tant que le picker suffit).

---

## D. Recherche

### UI-SRC-001 — Recherche (`SearchScreen`)

Champ, suggestions, onglets Vidéos / Comptes / Hashtags, listes infinies. `is_creator` DTO non affiché.

Pas d’écran hashtag dédié : un tap relance la requête.

---

## E. Social et profils

### UI-SOC-001 — Profil public (`PublicProfileScreen`)

Avatar, nom, @, badge créateur texte, bio, follow, compteurs. **Pas de grille vidéo.** P0 existant ; grille = P1 (dépend de l’API `GET /videos` publique **absente** : la liste est propriétaire seulement). **Lacune** : pas d’API « vidéos d’un user public ». Ne pas inventer l’écran grille tant que l’API n’existe pas — sauf à n’afficher que le profil.

### UI-SOC-002 — Commentaires (`CommentsSheet`)

Feuille modale, pas une route. Liste, réponses, like, delete, composer. Pas d’édition.

### UI-SOC-003 — Partage (`ShareSheet`)

WhatsApp / autres.

### UI-SOC-004 — Liste d’abonnés / abonnements

**À créer.** API `GET /users/{id}/followers|following` existante, non branchée. Deux états d’un même écran `FollowListRoute(kind)`. Priorité **P1**.

### UI-PRF-001 — Profil moi (`ProfileScreen`)

Avatar 96 dp, username, bio, badge (chip morte), CTA créateur, bannière email, extra (compteurs + mes vidéos), modifier, paramètres.

### UI-PRF-002 — Éditer (`EditProfileScreen`)

### UI-PRF-003 — Paramètres (`SettingsScreen`)

Appareils, logout, notif, supprimer.

### UI-PRF-004 — Supprimer le compte (`DeleteAccountScreen`)

---

## F. Notifications

### UI-NTF-001 — Liste (`NotificationsScreen`)

### UI-NTF-002 — Préférences (`NotificationPreferencesScreen`)

Switches alignés sur le contrat. Hint : push coupe l’OS, in-app demeure ; `orders` idem.

---

## G. Marketplace

### UI-SHP-001 — Accueil boutique (`ShopHomeScreen`)

Cart badge, mes commandes, trouver par handle (≥3), CTA ma boutique. **Pas de catalogue global.**

### UI-SHP-002 — Vitrine (`ShopScreen`) Grille 2 colonnes, `ProductCard`.

### UI-SHP-003 — Fiche produit (`ProductScreen`) Photos, vendeur, variantes, stepper 1–99, rupture, ajouter au panier.

### UI-SHP-004 — Panier (`CartScreen`)

### UI-SHP-005 — Checkout (`CheckoutScreen`) Deux étapes livraison / récap COD. `BackHandler` interne.

### UI-SHP-006 — Liste commandes (`OrdersScreen`) `asSeller`.

### UI-SHP-007 — Détail commande (`OrderScreen`) Statuts, ship, receive, cancel + dialogue.

### UI-SHP-008 — Ma boutique (`MyShopScreen`) Formulaire d’ouverture **ou** dashboard (statut, logo, produits).

### UI-SHP-009 — Éditeur produit (`ProductEditorScreen`)

Tous **P0 existants si la marketplace reste dans la barre** ; sinon masquage de l’onglet jusqu’au P1 produit (Q-01).

### UI-SHP-010 — Recherche catalogue

**Non prescrit** : backend PORO-026 absent. Ne pas maquetter un résultat produit sans API.

---

## H. Écrans à créer (API ou obligation existante)

### UI-MOD-001 — Signaler un contenu

| | |
|---|---|
| Objectif | Permettre `POST /moderation/reports` depuis une vidéo, un commentaire ou un profil |
| Utilisateurs | Membre authentifié, pas la cible |
| Entrée | Menu « … » sur overlay, commentaire, profil public |
| Structure | Bottom sheet : motifs du contrat (`spam`, `nudity`, `violence`, `harassment`, `hate`, `fraud`, `other`) + commentaire facultatif 500 |
| États | idle, sending, succès (accusé non détaillé), `cannot_report_self`, 429 10/h |
| API | `POST /api/v1/moderation/reports` |
| Priorité | **P0** (stores + sécurité utilisateurs). L’API est prête. |

Ne pas ajouter de motifs. Ne pas promettre un délai de traitement (non documenté).

### UI-LEG-001 — Mentions légales / confidentialité / CGU

| | |
|---|---|
| Objectif | Exigence stores et lois CI/SN/CM/NG (ADR-0004) |
| État | **Absent.** Textes juridiques **À confirmer** (pas dans le dépôt). |
| Priorité | **P0** pour un lancement public ; contenu = décision juridique, pas invention UX |

### UI-NTF-003 — Permission notifications OS

Pas un écran métier : dialogue système `POST_NOTIFICATIONS` (API 33+) après enregistrement FCM. Dépend de Firebase (aujourd’hui absent). Priorité **P1** avec NOTIF-003.

### UI-SOC-004 — déjà listé (followers).

### UI-PAY-W1 — Paiement Wave

**Non créé dans cet audit.** Service payment absent. Une maquette Wave avant le contrat API produirait un écran orphelin.

---

## I. Composants d’écran récurrents (existants)

| ID | Composant | Fichier |
|---|---|---|
| CMP-001 | `Avatar` | `ui/common/Components.kt` |
| CMP-002 | `BackTopBar` | idem |
| CMP-003 | `LoadableContent` | `Loadable.kt` |
| CMP-004 | `ErrorMessageText` | `auth/ErrorMessageText.kt` |
| CMP-005 | `SubmitButton` / `AuthForm` | `auth/AuthForm.kt` |
| CMP-006 | `CodeField` | `auth/CodeField.kt` |
| CMP-007 | `QuantityStepper` | `ProductScreen.kt` |
| CMP-008 | `ProductCard` | `ShopScreen.kt` |
| CMP-009 | `NotificationBell` | `NotificationsScreen.kt` |
| CMP-010 | `VideoPlayer` / `VideoSurface` | `ui/player/VideoPlayer.kt` |

Manques design system : menu « … », dialogue générique de confirmation (aujourd’hui `AlertDialog` local), snackbar métier unique (`SnackbarHost` MainActivity), empty-state illustré (texte seul).

---

## J. Synthèse quantitative

| Catégorie | Nombre |
|---|---|
| Destinations Navigation Compose | 27 |
| Splash | 1 |
| Feuilles / sections embarquées | Commentaires, partage, mes vidéos, cloche, actions sociales, compteurs |
| Écrans à créer avec API déjà là | UI-MOD-001, UI-SOC-004 |
| Écrans à créer sans copie encore | UI-LEG-001 |
| Écrans volontairement non créés | Chat, Wave, Enterprise, admin modération, recherche produit, grille vidéos publiques |

Tout écran existant ci-dessus a un fichier Kotlin nommé. Un composable n’implique pas un parcours robuste : l’upload meurt avec le process ; le push est un switch inerte ; le partage peut n’avoir pas d’URL.
