# ADR-0014 — Gateway : Traefik comme point d'entrée unique de l'API

- Statut : accepté
- Date : 2026-10-13

## Contexte

Dix services exposent chacun leur port (8081 à 8091) : un client doit connaître la carte des
ports. L'architecture cible prévoit un ingress Traefik dans k3s (`/api/v1/{domaine}`), qui
n'existe pas encore (P0-12). Deux préfixes sont partagés : `/api/v1/videos` (video et social)
et `/api/v1/users` (user et social). Les services vérifient déjà eux-mêmes les JWT via le JWKS
et ont leurs propres limites de débit par route.

## Décisions

1. **Traefik v3 par fichiers** plutôt qu'une gateway maison : même outil que l'ingress cible,
   rien à maintenir en code. Service compose `gateway`, `127.0.0.1:8000`, image à version
   exacte, utilisateur non-root, système de fichiers en lecture seule.
2. **Routage par préfixe de segments entiers**, chemins non réécrits. Les ressources sociales
   d'une vidéo (`/videos/{id}/like…`) et d'un compte (`/users/{uuid}/follow…`) partent vers
   social par des règles plus précises et prioritaires.
3. **Seul `/api/v1/**` est public**, plus le `/health` de la gateway. `/metrics`, `/health/*`,
   `/api/v1/health` et le JWKS restent internes.
4. **Pas de vérification de JWT à la gateway** : une seule source de vérité (chaque service),
   et la gateway ne voit jamais un token dans ses logs.
5. **Protections globales** : 1 Mio par requête (les médias passent par S3 présigné), timeouts
   bornés, compression, en-têtes de sécurité, CORS centralisé (`GATEWAY_CORS_ORIGINS`).
6. **Limite de débit large** (100 req/s, rafales de 200 par IP) : derrière le CGNAT des
   opérateurs mobiles africains, des centaines d'utilisateurs partagent une IP. Les limites
   fines restent dans les services, la défense contre les abus sera Cloudflare.
7. **IP du client** : la gateway ignore le `X-Forwarded-For` reçu et pose le sien ; les
   services ne le croient que venant du réseau Docker privé (`GATEWAY_TRUSTED_PROXIES`).
8. **Le contrat de routage est testé contre les OpenAPI** : toute route documentée doit
   atteindre son service, sinon la CI échoue.

## Conséquences

- Ajouter un préfixe d'API demande une règle dans `infra/gateway/routes.yml`.
- P0-12 reprendra ces règles en `IngressRoute` k3s, avec TLS et HSTS.
- Les erreurs propres à Traefik (`404`, `413`, `429`, `503`) sont en texte brut.
- Les ports directs restent publiés en local pour le débogage.

## Alternatives écartées

- Gateway Go maison : contrôle total et enveloppe JSON partout, mais un service de plus et un
  doublon de l'ingress de production.
- nginx, Envoy, Kong : sans lien avec l'ingress k3s prévu, ou plus lourds à opérer.
