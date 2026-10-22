# A. Rapport exécutif : intégration Backend ↔ Android (2026-10-22)

Périmètre : 10 services backend (auth, user, video, feed, social, search, notification, shop, order, moderation),
la gateway Traefik et l'application Android. Branche `claude/gifted-ritchie-t5ej2z` (`884b652`, plus `main`
@ `f1ea1ec`), corrections dans l'arbre de travail, **non commitées**. Mis à jour le 2026-10-22 après la mission de
fiabilisation : liste des tâches et statuts dans `04-corrections.md`.

## Réserve préalable

Le propriétaire signale du code Android fait sur son poste et **non poussé** : édition de commentaire, carnet
d'adresses, vidage du panier, retrait du logo, listes d'abonnés, copie locale du média, réglage push. **Ce code n'a
pas pu être audité.** Les fonctionnalités concernées apparaissent en BLEU dans la matrice, avec cette mention.

## Architecture vérifiée

- Les clients passent par une **gateway Traefik unique** (`127.0.0.1:8000`). Seul `/api/v1/**` est exposé.
- **Auth** émet des JWT RS256 (15 minutes) et des refresh tokens rotatifs (30 jours, révocables). Les autres services vérifient les jetons hors ligne (JWKS) et la liste de révocation.
- Les services communiquent par Kafka/Redpanda, via des outbox SQL.
- **Android** :
  - lit son URL de base dans `BuildConfig.API_BASE_URL` : `10.0.2.2` en debug, ou `poro.debugApiBaseUrl` ; `PORO_API_BASE_URL` en release, HTTPS obligatoire ;
  - renouvelle les jetons sur un 401 avec un `Authenticator` OkHttp ;
  - chiffre les jetons avec le Keystore.

## Conclusion

| Question | Réponse |
|---|---|
| **Backend ↔ Android** | **Partiellement conforme.** Les 92 appels Android correspondent tous à des routes des contrats, et toutes ces routes existent dans le code. Mais des limites de validation divergeaient (E-02 à E-04, corrigées), et l'envoi d'images échoue en conditions réelles (E-01, non diagnostiqué) |
| **Couverture fonctionnelle mobile** | **86,5 %** (90 / 104) des endpoints destinés aux utilisateurs ont un parcours dans l'app. Dénominateur : 109 endpoints métier des contrats, moins 5 internes (modération, suspension de boutique) |
| **Intégration vérifiée en réel** | **23 %** (24 / 104) observés avec la réponse attendue contre un vrai backend ; **58,7 %** (61 / 104) seulement couverts par des tests JVM |
| **Bugs bloquants** | **2** : E-01 (envoi d'images : ni avatar ni produit publiable ; diagnostic bloqué sans journal `PoroMedia`) et l'absence d'URL de production (release impossible, désormais bloquée volontairement : E-06) |
| **Backend sans interface mobile** | 14 endpoints (BLEU), détaillés dans la matrice : annulation de suppression de compte, enregistrement FCM (2), vider le panier, carnet d'adresses (4), retrait du logo, listes d'abonnés et d'abonnements, vidéos aimées, comptes ayant aimé, édition de commentaire |
| **Tests** | Android : 195 tests JVM, 0 échec. Backend : 774 tests passent (Go 256, Jest 449, pytest 69), 0 assertion en échec ; 8 échecs et 38 erreurs, tous dus à l'absence de Docker (`03-tests.md`) |
| **Prêt pour publication** | **Non** |

## Pourquoi pas prêt à publier

1. Envoi d'images défaillant en réel (E-01).
2. Pas d'URL de production, de keystore ni de build release réellement produit.
3. Textes légaux non validés : tous en brouillon « À valider ».
4. Pas de push : aucun projet Firebase.
5. Moins d'un quart de l'API est vérifié de bout en bout ; aucun test instrumenté.
6. Le code Android du poste du propriétaire n'est ni poussé ni audité.
7. Côté backend : corrections B-01 (vidéo), B-03, B-05 et B-06 faites mais **non commitées**, donc ni compilées ni testées par la CI (Postgres, AGP). Restent PORO-008 pour les images et B-07 (vendeur suspendu).

## Décisions nécessaires avant publication

- URL de production de l'API, keystore de publication et variables `PORO_*` (voir `apps/mobile-android/README.md`).
- Projet Firebase, ou lancement sans push assumé.
- Validation juridique des 4 documents légaux.
- Mode invité (B1) : oui ou non au lancement.
- Pousser et fusionner le travail Android local, puis rejouer cet audit sur le code réuni.

Livrables : `01-matrice-couverture.md` (B), `02-ecarts.md` (C), `03-tests.md` (D), `04-corrections.md` (E).
