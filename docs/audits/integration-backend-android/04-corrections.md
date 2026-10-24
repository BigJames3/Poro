# E. Bilan des corrections (arbre de travail, non commité)

## Mission d'intégration (première passe)

| Fichier | Changement | Écart | Test |
|---|---|---|---|
| `app/build.gradle.kts` | Les tâches `assemble`, `bundle` et `package` Release échouent sans `PORO_API_BASE_URL` / `poro.apiBaseUrl` | E-06 | Sonde Gradle |
| `core/text/Runes.kt` (nouveau) | Comptage en points de code, troncature sans couper une paire de substitution, détection des caractères Cc et Cf | E-02, E-03 | `RunesTest` |
| `ui/upload/UploadViewModel.kt`, `UploadScreen.kt` | Titre 100 et description 500 points de code ; caractère invisible signalé sous le champ et publication bloquée | E-02, E-03 | `RunesTest` |
| `core/account/AccountRules.kt`, `ui/profile/EditProfileViewModel.kt`, `EditProfileScreen.kt` | Nom 50 et bio 160 points de code, bio sur 5 lignes au plus, compteurs et erreurs affichés | E-04 | `AccountRulesTest` |
| `data/video/VideoUploader.kt` | `UploadSessionStore` : envoi mémorisé. À la reprise, une vidéo déjà publiée est renvoyée telle quelle, et un envoi `uploading` laissé en plan est annulé avant de recommencer | E-05 | `VideoUploadSessionTest` |
| `data/video/WorkManagerUploads.kt`, `AppContainer.kt` | `PrefsUploadSessionStore` (préférences privées), branché sur le client d'envoi | E-05 | CI requise (Android) |
| `ui/notification/NotificationPreferencesScreen.kt` | Réglages « push » et « commandes » désactivés avec une explication, tant que FCM n'est pas intégré | E-07 | — |
| `ui/shop/ProductEditorScreen.kt` | 10 photos par produit (au lieu de 8) | E-08 | — |
| `res/values/strings.xml` | Textes des messages ci-dessus ; `upload_hashtags_hint` remplacé par `upload_description_count` | — | — |
| Tests : `core/text/RunesTest.kt`, `data/video/VideoUploadSessionTest.kt`, `core/account/AccountRulesTest.kt` | +9 tests | — | 191 tests, 0 échec |
| `docs/audits/integration-backend-android/*` (nouveau) | Livrables A à E | — | — |

## Mission de fiabilisation (seconde passe, 2026-10-22)

| Fichier | Changement | Écart | Test |
|---|---|---|---|
| `services/user/src/profiles/profile.dto.ts` | `username: null` refusé en 400 au lieu d'une erreur 500 | B-05 | `profile.dto.spec.ts` (4) |
| `docs/api/video.openapi.yaml`, `user.openapi.yaml`, `auth.openapi.yaml` | Limites et comportement alignés sur le code | B-03 | YAML validé |
| `packages/shared-go/ratelimit/` (nouveau) | Limiteur Redis à fenêtre fixe qui refuse en 503 si Redis est indisponible ; clé par compte ou par IP | B-06 | `ratelimit_test.go` (5) |
| `services/{social,search,feed,video}/internal/middleware/rate_limit.go`, `routes.go`, `cmd/server/main.go`, `go.mod`, `go.sum` | Passage au limiteur partagé (nouveaux préfixes `…:rl:v2:`) ; video compte après le JWT | B-06 | Tests existants adaptés ; video : panne Redis → 503 |
| `services/auth/internal/middleware/rate_limit.go`, `routes/routes.go` | Même limiteur ; routes connectées comptées par compte, `/refresh` séparé | B-06 (CGNAT) | `TestAccountLimitsCountPerAccount`, tableau étendu |
| `services/video/internal/{model,repository,service}`, `cmd/server/main.go` | Nettoyage horaire des envois `uploading` de plus de 24 h | B-01 | `TestSweepAbandonedUploads` (Postgres : exécuté par la CI seulement) |
| `apps/mobile-android/…/core/text/MultilineText.kt` (nouveau), `ui/common/TextLimitHint.kt` (nouveau), `ui/shop/{MyShop,ProductEditor,Checkout}*.kt`, `strings.xml` | Limites de lignes et caractères refusés des descriptions et adresses | E-11 | `MultilineTextTest` (3), `CheckoutViewModelTest` (+1) |

Aucun secret, aucune infrastructure ni donnée de production modifiés. Aucun commit ni push.

## Liste des tâches (état au 2026-10-22)

| Priorité | Tâche | Statut |
|---|---|---|
| P0 | E-01 Envoi d'images (avatar, logo, photo produit) | **Diagnostiqué côté app** (chantier 7) : envoi de l'app accepté par SeaweedFS 4.45, avec la réécriture de l'émulateur. Cause restante : port 9000 injoignable depuis l'appareil. À confirmer avec la ligne `PoroMedia` du Logcat |
| P0 | Configuration de production (URL d'API, keystore, Firebase) | **BLOQUÉ** : valeurs à fournir par le propriétaire. Garde-fou release en place (E-06) |
| P1 | Fiabilité de l'envoi vidéo (doublons, orphelins) | TERMINÉ (E-05, B-01 vidéo) ; CI Android et Postgres requises |
| P1 | Limites de validation (vidéo, profil, boutique, commande) | TERMINÉ (E-02 à E-04, E-08, E-11, B-03) |
| P1 | Réglages push et commandes honnêtes | TERMINÉ (E-07) |
| P1 | `username: null` → 500 | TERMINÉ (B-05) |
| P1 | Limiteurs ouverts si Redis tombe ; CGNAT | TERMINÉ (B-06) |
| P1 | Envois bruts d'avatar et de boutique orphelins | TERMINÉ : `UploadSweeper` (user, shop), tests `upload-sweeper.spec.ts` |
| P2 | Filet de sécurité côté stockage et plafond par partie (PORO-008) | TERMINÉ sur la branche du chantier 1, sous réserve de la CI : TTL du filer, `seaweedfs-janitor`, `Content-Length` signé |
| P1 | Vendeur suspendu toujours actif dans shop et order (B-07) | À FAIRE (PORO-021, contrôle inter-services) |
| — | Mode invité (B-02) | **Décision** du propriétaire : seul le fil public des tendances est ouvert (nouvelle route), aucune route privée rendue publique ; voir chantier 5 |
| — | Émojis composés dans les titres vidéo (B-04) | **Décision** : autoriser U+200D seul ouvre la porte à du texte invisible |
| — | Wave (B-09) | Laissé désactivé : seul `cash_on_delivery` est proposé ; aucune confirmation simulée |
| — | Fusion du travail Android local non poussé | **BLOQUÉ** : `wip/android-local` absente de GitHub |

## Risques résiduels

- **Le code Android modifié n'a pas été compilé par AGP.** Les parties Android pures (`PrefsUploadSessionStore`) passent seulement par la CI, après un commit.
- **Limiteurs** : au déploiement, les compteurs repartent de zéro (nouveaux préfixes). Une panne Redis rend désormais social, search, feed et video indisponibles (503) au lieu de les laisser sans limite : c'est voulu, mais Redis devient une dépendance critique à surveiller.
- **Nettoyage vidéo** : testé seulement sans Postgres ici ; le test d'intégration ne tourne qu'en CI.
- **Reprise d'envoi** : le serveur ne permet pas de reprendre un envoi entamé. Après un arrêt du processus, l'envoi recommence depuis le début : le volume de données consommé est doublé, mais il n'y a plus ni doublon ni orphelin.
- **Le travail Android signalé comme fait sur le poste du propriétaire** (édition de commentaire, carnet d'adresses, vidage du panier, retrait du logo, listes d'abonnés, copie locale du média) **n'est pas dans le dépôt**. Fusionner ce travail avec les modifications ci-dessus produira probablement des conflits dans `UploadViewModel.kt`, `VideoUploader.kt`, `WorkManagerUploads.kt` et `strings.xml`.

## Décisions métier ouvertes

1. Mode invité : tranché (fil public en lecture seule, chantier 5).
2. Émojis composés dans les titres et descriptions vidéo (B-04).
3. Écrans « vidéos aimées » et « qui a aimé » : utiles au lancement ?
4. Écran « annuler la suppression du compte » : la reconnexion suffit-elle ?
5. Projet Firebase (FCM) et URL de production de l'API.

## Chantiers de finalisation (prompts du 2026-10-22)

Décisions du propriétaire (2026-10-22) : suspension de compte par ADMIN et MODERATOR (un MODERATOR ne
vise ni un ADMIN ni un MODERATOR) ; commandes non expédiées d'une boutique suspendue annulées
automatiquement ; mode invité en lecture seule (fil « Pour toi » public) ; émojis composés autorisés
dans les titres vidéo (U+200D entre deux émojis seulement) ; Wave seul paiement en ligne au lancement.

| # | Chantier | Statut |
|---|---|---|
| 1 | PORO-008 (fin) : expiration côté stockage, plafond par partie | TERMINÉ (PR #61 fusionnée) |
| 2 | PORO-021 : suspension des comptes (ADMIN, MODERATOR) et des boutiques | TERMINÉ (PR #62 fusionnée) |
| 2b | Annulation automatique des commandes d'une boutique suspendue | TERMINÉ (PR #63 fusionnée) |
| 3 | PORO-026 : recherche dans le catalogue des boutiques | TERMINÉ (PR #64 fusionnée) |
| 4 | Émojis composés dans les titres et descriptions vidéo | TERMINÉ (PR #65 fusionnée) |
| 5 | Mode invité en lecture seule | TERMINÉ (PR #66 fusionnée) |
| 6 | Build de release signé dans la CI | TERMINÉ (PR #67 fusionnée, workflow `android-release.yml`) ; BLOQUÉ pour la signature réelle (keystore, URL de production) |
| 7 | Tests instrumentés Android et parcours de bout en bout | EN COURS : 7a diagnostic E-01 (PR #68 fusionnée) ; 7b tests instrumentés (PR #69 fusionnée) ; 7c parcours de bout en bout, 97/97 routes de l'app (PR ouverte, job `e2e`) ; correctifs E-01 et B-10 (PR #70) |
| 7d | Portage de la sauvegarde, lot A (marketplace) : carnet d'adresses, vider le panier, retirer le logo | EN COURS : branche locale `claude/lot-a-marketplace`, en attente d'autorisation de pousser |
| 8 | PORO-015 : service de paiement Wave | À FAIRE ; BLOQUÉ pour la mise en service (clés de bac à sable, secret de webhook) |
| 9 | Notifications push (FCM) | À FAIRE ; BLOQUÉ pour la mise en service (projet Firebase) |

## Module « Publier », phase 1 : défauts de l'envoi vidéo

Diagnostic du 2026-10-24 (parcours vidéo réel de bout en bout ; publication d'images absente du
backend, traitée dans les phases suivantes). Quatre défauts corrigés :

- **« Réessayer » ne réessayait pas** : le bouton fermait seulement l'erreur, et le formulaire était
  vidé dès la mise en file. Désormais la vidéo, le titre et la description restent jusqu'à la
  confirmation du serveur ; « Réessayer » relance le même envoi après une erreur réseau, un refus
  du stockage ou une erreur serveur, jamais après un refus de contenu (« Modifier la publication »).
- **Doublon possible à la reprise** : si la réponse à `complete` se perdait, l'app annulait puis
  publiait une seconde fois. Vérifié sur la pile réelle : un `complete` rejoué reçoit 409 et la
  vidéo est déjà `processing`. Sur réseau, 5xx ou 409, la session est gardée sans rien annuler ;
  l'essai suivant relit la vidéo au lieu de la republier.
- **Aucun aperçu** : la vidéo choisie est maintenant lisible dans l'écran (Media3, contrôles standard).
- **Accès au fichier perdu** : autorisation retirée ou fichier supprimé donnent un message
  explicite (« Choisissez-la à nouveau ») et le bouton « Choisir à nouveau la vidéo » ; le titre et
  la description sont conservés. Avant : plantage du worker et erreur générique.
