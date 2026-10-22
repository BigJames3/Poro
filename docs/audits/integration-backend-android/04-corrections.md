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
| P0 | E-01 Envoi d'images (avatar, logo, photo produit) | **BLOQUÉ** : il faut la ligne `PoroMedia` du Logcat. Code et serveur vérifiés ; hypothèse : `adb reverse tcp:9000 tcp:9000` absent après redémarrage de l'émulateur |
| P0 | Configuration de production (URL d'API, keystore, Firebase) | **BLOQUÉ** : valeurs à fournir par le propriétaire. Garde-fou release en place (E-06) |
| P1 | Fiabilité de l'envoi vidéo (doublons, orphelins) | TERMINÉ (E-05, B-01 vidéo) ; CI Android et Postgres requises |
| P1 | Limites de validation (vidéo, profil, boutique, commande) | TERMINÉ (E-02 à E-04, E-08, E-11, B-03) |
| P1 | Réglages push et commandes honnêtes | TERMINÉ (E-07) |
| P1 | `username: null` → 500 | TERMINÉ (B-05) |
| P1 | Limiteurs ouverts si Redis tombe ; CGNAT | TERMINÉ (B-06) |
| P1 | Envois bruts d'avatar et de boutique orphelins | TERMINÉ : `UploadSweeper` (user, shop), tests `upload-sweeper.spec.ts` |
| P2 | Règle de cycle de vie S3 (filet de sécurité côté stockage) | À FAIRE (PORO-008, infrastructure) |
| P1 | Vendeur suspendu toujours actif dans shop et order (B-07) | À FAIRE (PORO-021, contrôle inter-services) |
| — | Mode invité (B-02) | **Décision** du propriétaire (B1) : aucune route privée rendue publique |
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

1. Mode invité : rendre `trending` public (B1) ?
2. Émojis composés dans les titres et descriptions vidéo (B-04).
3. Écrans « vidéos aimées » et « qui a aimé » : utiles au lancement ?
4. Écran « annuler la suppression du compte » : la reconnexion suffit-elle ?
5. Projet Firebase (FCM) et URL de production de l'API.
