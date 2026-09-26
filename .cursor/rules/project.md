# Règles du projet

## Architecture
- Monorepo Turborepo
- Backend : Go (critique) + NestJS (métier)
- Mobile : Kotlin + Compose (Android), Swift (iOS)
- Infra : Terraform + k3s

## Conventions
- Commits : Conventional Commits
- Go : gofmt + golangci-lint
- TS : ESLint + Prettier strict
- Kotlin : ktlint + detekt
- Jamais de secrets en dur
- Toujours valider les inputs
