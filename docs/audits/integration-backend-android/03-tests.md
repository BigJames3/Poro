# D. Rapport de tests (2026-10-22)

Environnement : conteneur Linux **sans démon Docker, sans SDK Android, sans émulateur**. Les tests qui dépendent de
ces outils ne sont **pas** comptés comme exécutés.

## Android

| Commande | Résultat | Remarque |
|---|---|---|
| Tests JVM (`app/src/test`), avant intervention | **43 classes, 182 tests, 0 échec** | Harnais local : Gradle 8.14, Compose Multiplatform desktop à la place des artefacts Android (dépôt Maven de Google inaccessible) ; exclus : Keystore, `ContentResolverMediaReader`, lecteur Media3, WorkManager, source des textes légaux |
| Mêmes tests, après corrections | **45 classes, 191 tests, 0 échec, 0 ignoré** | +9 : `RunesTest` (3), `VideoUploadSessionTest` (5), `AccountRulesTest` (1) |
| Mêmes tests, après la mission de fiabilisation | **46 classes, 195 tests, 0 échec, 0 ignoré** | +4 : `MultilineTextTest` (3), `CheckoutViewModelTest` (1) |
| `./gradlew testDebugUnitTest assembleDebug` (CI `mobile-build`) | **Vert** sur `884b652` (2026-10-22 08:33 UTC) | Dernier commit poussé. **Les corrections de ce rapport ne sont pas commitées : la CI ne les a pas compilées** |
| Garde-fou release (sonde Gradle, mêmes fonctions que `app/build.gradle.kts`) | `assembleDebug` et `testDebugUnitTest` passent ; `assembleRelease` **échoue** sans URL (« Release sans URL d'API… ») ; `bundleRelease` passe avec `PORO_API_BASE_URL=https://api.example.com/` | AGP non chargé : la logique Gradle est vérifiée, pas la release réelle |
| `assembleRelease` / `bundleRelease` réels | **Non exécuté** | Pas de SDK Android ni de keystore |
| Tests instrumentés (`androidTest`) | **Aucun dans le dépôt** | — |
| Parcours réels sur émulateur | **Partiels**, journaux du propriétaire du 2026-10-21 | 24 endpoints observés avec la réponse attendue (VERT dans la matrice) ; échecs : envoi d'images (E-01), sessions 404 et statistiques 503 (environnement) |

## Backend

Résultats ci-dessous : première passe. **Seconde passe (mission de fiabilisation, même environnement)** :
shared-go 38 passent (+5, `ratelimit`), auth 113 (+1), social 15, search 16, video 40 (+1 ignoré :
`TestSweepAbandonedUploads`, Postgres), feed 31, outbox-relay 3 ; user 118 (+4, `profile.dto.spec.ts`), shop 125,
order 100, notification 106 ; moderation 69. **Total : 774 tests passent** (Go 256, Jest 449, pytest 69), aucune
assertion en échec ; les mêmes 8 échecs et 38 erreurs, tous dus à l'absence de Docker. `golangci-lint` v2.14.0 (version
de la CI) : 0 problème sur shared-go, auth, social, search, feed et video ; `eslint` et `tsc` propres sur le service user.

Toolchain Go 1.26.10 téléchargée automatiquement (`GOTOOLCHAIN=auto`). **Aucune assertion en échec** : tous les échecs
et toutes les erreurs viennent de l'absence de Docker (testcontainers).

| Module | Commande | Résultat exact | Non exécuté (raison) |
|---|---|---|---|
| packages/shared-go | `go test -count=1 ./...` | FAIL : 33 tests passent, 3 échouent (`TestKafka`, `TestOutbox`, `TestInboxClaim`) | Les 3 échecs : « rootless Docker not found » |
| services/auth | idem | FAIL : 112 passent, 5 échouent (consumer ×2, e2e ×3) ; package `internal/repository` arrêté dans `TestMain` (30 tests jamais lancés) | Postgres (Docker) |
| services/social | idem | ok : 15 passent, 33 ignorés | Postgres (Docker) |
| services/search | idem | ok : 16 passent, 12 ignorés | idem |
| services/video | idem | ok : 40 passent, 20 ignorés | Postgres ; `TestS3AgainstSeaweedFS` (pas de `S3_INTEGRATION_ENDPOINT`) |
| services/feed | idem | ok : 31 passent, 20 ignorés | Postgres |
| services/outbox-relay | idem | ok : 3 passent | — |
| services/user | `npm ci && npm test -- --ci --testPathIgnorePatterns '\.e2e\.spec\.ts$'` | 11 suites, **114 tests passent** | 5 fichiers e2e (Postgres) ; `DATABASE_URL` factice pour `prisma generate`, sans connexion |
| services/shop | idem | 12 suites, **125 passent** | 6 fichiers e2e |
| services/order | idem | 10 suites, **100 passent** | 5 fichiers e2e |
| services/notification | idem | 13 suites, **106 passent** | 6 fichiers e2e |
| services/moderation | `uv sync && uv run pytest -q` | **69 passent, 38 erreurs** | Les 38 erreurs : fixture Postgres (Docker) |

Constat : les suites Jest incluent les fichiers e2e par défaut (`testRegex` `.*\.spec\.ts$`), et les tests Go de
shared-go et d'auth **échouent** au lieu d'être ignorés sans Docker. Une CI sans Docker les verrait rouges.

## Tests de bout en bout

**Non exécutés dans cet environnement** (pas de pile Docker, pas d'appareil). À rejouer sur le poste du propriétaire :
scénarios NR-01 à NR-10 et MK-01 à MK-16 de `docs/product/mobile-ux-phase-2/08-acceptance-tests.md`, plus :

- E2E-01 Envoi vidéo puis arrêt forcé de l'app (Paramètres Android → Forcer l'arrêt). Résultat attendu : à la reprise, une seule vidéo publiée, et aucune ligne `uploading` orpheline dans « Mes vidéos ».
- E2E-02 Session expirée pendant la navigation, vérifiée dans les journaux : `401` suivi de `POST /auth/refresh`, puis la requête rejouée.
- E2E-03 Mode avion pendant le fil, puis retour du réseau.
- E2E-04 Rôles : un compte non modérateur reçoit 403 sur `/moderation/cases`. Vérifié par les tests du service, pas depuis l'app.
