# Sécurité

## Signaler une faille

Ne pas ouvrir d'issue publique. Écrire au propriétaire du dépôt (BigJames3)
par message privé GitHub, avec les étapes pour reproduire. Réponse sous 72 h.

## Contrôles en CI

| Contrôle | Où | Bloque si |
|---|---|---|
| golangci-lint (dont gosec, errorlint, bodyclose) | Tous les modules Go | Une alerte |
| govulncheck v1.8.0 | Tous les modules Go | Une faille atteignable depuis le code |
| npm audit `--omit=dev --audit-level=high` | user, notification, shop, order | Une faille HIGH ou CRITICAL en production |
| pip-audit | moderation | Une faille connue |
| Trivy (images) | Chaque image construite | Une faille HIGH ou CRITICAL **avec correctif disponible** ; les autres sont listées |
| Trivy (dépôt) | Job `security`, à chaque PR | Un secret ou une erreur de configuration HIGH ou CRITICAL |

Les actions GitHub sont épinglées par SHA de commit, la version en
commentaire. Dependabot propose chaque semaine les mises à jour des actions,
des modules Go, des paquets npm, de moderation (uv) et des images de base des
Dockerfiles. Les images de `docker-compose.yml` restent figées par digest et
se changent à la main ; `apps/mobile-android` est hors périmètre.

## Exceptions

Une exception doit être ciblée et justifiée sur la ligne même :

```go
cmd := exec.CommandContext(ctx, bin, args...) //nolint:gosec // G204: configured binary, arguments built here
```

Jamais de désactivation globale d'un linter ni de `--exit-code 0` pour
« faire passer » la CI. Une faille sans correctif qui bloquerait s'ignore dans
un `.trivyignore` avec l'identifiant, la raison et une date de revue.

Les `overrides` npm de `mysql2` et `deepmerge-ts` corrigent des failles du CLI
Prisma 7.10 (outil de migration). À retirer quand Prisma livre des versions
corrigées.

## Réglages GitHub à activer (hors code)

Dans **Settings → Branches → Branch protection rules** pour `main` :
- *Require a pull request before merging* ;
- *Require status checks to pass*, avec **`ci-ok`** et `security`. `ci-ok` agrège tous les jobs
  de la CI (tests, lint, format, typecheck, build et scan des images, migrations appliquées par les
  tests, gateway, stockage, workflows, compose) : il échoue si l'un d'eux a échoué ou a été annulé ;
  un job non concerné par le changement (filtre de chemins) compte comme réussi. Exiger seulement
  `detect-changes` laissait passer une PR dont les tests échouent ;
- *Require branches to be up to date before merging* ;
- *Do not allow bypassing the above settings* ;
- interdire les *force pushes* et la suppression de `main`.

Ces réglages demandent un administrateur du dépôt : ils ne peuvent pas être posés depuis le code.

## Publication des images

`.github/workflows/images.yml` publie les images sur GHCR (`ghcr.io/<propriétaire>/poro-<service>`)
seulement après une CI verte sur un push de `main`, jamais depuis une PR. Étiquette unique
`sha-<commit complet>` : jamais `latest`, une étiquette existante n'est jamais réécrite. Chaque
image passe le scan Trivy avant publication et porte une attestation de provenance signée
(`gh attestation verify oci://<image> --owner <propriétaire>`). Le job a seulement
`packages: write`, `id-token: write` et `attestations: write`, et n'utilise que le `GITHUB_TOKEN`.

Déployer par **digest** (`@sha256:…`) ou par `sha-<commit>`, jamais par une étiquette mobile.

Dans **Settings → Code security** : *Dependabot alerts*, *Dependabot security
updates* et *Secret scanning* (avec *Push protection*).
