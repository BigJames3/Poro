# Plan d'implémentation

Aucun de ces lots n'est commencé. Toute modification de `apps/mobile-android/` attend
l'autorisation explicite du propriétaire. Chemins relatifs à `app/src/main/java/com/poro/app/`.

Ordre proposé : **L0 → L1 → L2 → L3**, en parallèle des décisions backend ; **L4** seulement après B1 ;
**L5** et **L6** ensuite.

## Lot 0 — Écarts documentaires et contrats

| Rubrique | Détail |
|---|---|
| Tâches | L0-1 récupérer et pousser `docs/product/mobile-ux-audit/` puis le comparer à ce dossier ; L0-2 fusionner ou rebaser la PR #58 sur `main` (6 commits d'écart, contrats `users?ids=` et `videos/stats?ids=`) ; L0-3 confirmer le comportement public d'une boutique `closed`/`suspended` ; L0-4 obtenir le journal `PoroMedia` de l'envoi de photo ; L0-5 reconstruire les images Docker (`auth`, `social`) et rejouer NR-01 à NR-09 |
| Prérequis | Accès au poste du propriétaire |
| Fichiers | Aucun fichier Android |
| Tests | NR-01 à NR-09 en l'état (référence avant changements) |
| Risques | L'audit de phase 1 contredit ce dossier → arbitrage avant L1 |
| Terminé quand | Écarts listés dans `10` tranchés ; journal de référence NR enregistré |

## Lot 1 — Stabilisation de la marketplace et messages trompeurs

| Rubrique | Détail |
|---|---|
| Tâches | M1 à M11 (`02` § 4), dans l'ordre : M4 (photos, après diagnostic), M1, M2, M3, M5, M8, M9, M6, M7, M10, M11 |
| Prérequis | L0-4, L0-5 |
| Fichiers | `ui/shop/ProductViewModel.kt`, `ProductScreen.kt`, `CheckoutViewModel.kt`, `CheckoutScreen.kt`, `OrderScreens.kt`, `OrderViewModels.kt`, `ShopGraph.kt`, `core/network/ApiResult.kt`, `ui/auth/AuthErrorMessages.kt`, `res/values/strings.xml`, éventuellement `core/network/PresignedUpload.kt` ; tests `ui/shop/*Test.kt`, `testing/FakeOrderRepository.kt` |
| Tests | MK-01 à MK-16, NR-09, NR-10 |
| Risques | M1 : course entre deux ajouts rapides (lecture puis écriture) → sérialiser les ajouts dans le ViewModel ; M4 : cause environnementale possible (aucun code à changer) |
| Terminé quand | Tests U verts, MK exécutés, aucun message ne contredit le statut serveur |

## Lot 2 — Signalement

| Rubrique | Détail |
|---|---|
| Tâches | Couche `data/moderation`, `ReportSheet`, `ReportViewModel`, entrées vidéo / commentaire / profil |
| Prérequis | Libellés des motifs validés (décision R1) |
| Fichiers | nouveaux : `data/moderation/ModerationApi.kt`, `ModerationRepository.kt`, `ui/moderation/ReportSheet.kt`, `ReportViewModel.kt` ; modifiés : `AppContainer.kt`, `ui/social/VideoActions.kt`, `CommentsSheet.kt`, `PublicProfileScreen.kt`, `SocialGraph.kt`, `strings.xml`, `ApiResult.kt` |
| Tests | RP-01 à RP-08, NR-03, NR-04 |
| Risques | Encombrement de `VideoActions` → menu « ⋯ » plutôt qu'un bouton de plus |
| Terminé quand | Signalement possible sur les 3 cibles, idempotence et limites gérées |

## Lot 3 — Écrans légaux

| Rubrique | Détail |
|---|---|
| Tâches | `LegalIndexRoute`, `LegalDocumentRoute`, documents locaux « À valider », entrée Paramètres, mention sur les écrans de connexion |
| Prérequis | Liste des documents (L2) ; décision sur la mention (L1) |
| Fichiers | nouveaux : `ui/legal/*`, `res/raw/legal_*.md` (ou `strings`) ; modifiés : `Routes.kt`, `PoroNavHost.kt` ou `ProfileGraph.kt`, `SettingsScreen.kt`, `PhoneEntryScreen.kt`, `EmailRegisterScreen.kt` |
| Tests | LG-01 à LG-04, NR-01, NR-02, NR-08 |
| Risques | Texte provisoire pris pour définitif → bandeau non masquable, revue juridique obligatoire avant toute publication |
| Terminé quand | Écrans accessibles, textes marqués, aucune case d'acceptation |

## Lot 4 — Mode invité (bloqué)

| Rubrique | Détail |
|---|---|
| Prérequis | **B1** livré (feed public documenté), B2, B3 (`03` § 5) ; autorisation explicite |
| Tâches | État de session `Guest`, démarrage sur le fil, `SignInPromptSheet`, reprise après connexion, onglets invités |
| Fichiers | `core/security/TokenStore.kt` (état de session), `ui/PoroNavHost.kt`, `ui/AppNav.kt` (`isSignInScreen`, `goToSignIn`), `ui/feed/*`, `ui/social/*`, `ui/shop/ProductScreen.kt`, nouveau `ui/common/SignInPromptSheet.kt` |
| Tests | GU-01 à GU-04, NR-01 à NR-09 |
| Risques | Régression de la gestion de session expirée (critique) ; fuite de données d'un compte précédent |
| Terminé quand | Aucune requête protégée sans jeton ; NR complets |

## Lot 5 — Améliorations P1

État au 2026-10-22 (thème exclu à la demande du propriétaire, en attente de DS1) : noms d'auteur
des commentaires, compteurs du fil par lot et envoi vidéo par WorkManager **réalisés** ;
préchargement de la vidéo suivante **réalisé** (second lecteur, tampon plafonné à 15 s) ; push FCM bloqué (aucun projet Firebase).
Limite connue de l'envoi : sur Android 13+, la notification de progression n'est visible que si
l'utilisateur a accordé la permission de notifications, qui n'est pas encore demandée.

| Tâche | Prérequis | Fichiers probables | Risque |
|---|---|---|---|
| Envoi de vidéo survivant à l'arrêt du processus (WorkManager) | Accord sur l'ajout de `androidx.work` | `data/video/UploadManager.kt`, nouveau worker | Reprise d'un envoi multipart (URL présignées expirées) |
| Noms d'auteur des commentaires via `GET /api/v1/users?ids=` | L0-2 | `ui/social/CommentsViewModel.kt`, `data/user/*` | Faible |
| Compteurs du fil via `GET /api/v1/videos/stats?ids=` (une requête par page) | L0-2 | `ui/social/VideoInteractionsViewModel.kt` | Faible |
| Lecture vidéo : préchargement de la vidéo suivante, reprise après coupure | — | `ui/player/VideoPlayer.kt`, `ui/feed/FeedScreen.kt` | Mémoire sur petits appareils |
| Push FCM | Projet Firebase, `FCM_CREDENTIALS_B64` | nouveau service de messagerie, `PUT /notifications/devices` | Nouvelle dépendance Firebase, consentement aux notifications (Android 13+) |
| Extension `PoroTheme` (`06`) | DS1 | `ui/theme/Theme.kt`, `res/values/themes.xml` | Changement visuel global → captures avant/après |

## Lot 6 — Fonctionnalités conditionnées au backend

| Fonctionnalité | Service requis |
|---|---|
| Découverte et recherche catalogue | PORO-026 |
| Paiement Wave / GeniusPay | `services/payment`, PORO-015 |
| Signalement de produit ou de boutique | Extension de `target_type` (modération) |
| Consentement enregistré | Endpoint légal (option B de `05`) |
| Liens de partage et liens d'application | Domaine web, `assetlinks.json`, `SHARE_BASE_URL` |
