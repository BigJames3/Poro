# Runbook — gateway (Traefik)

Port public 8000 (`127.0.0.1` en local), port interne 8082 (`/ping`, `/metrics`, non publié).
Configuration : [`infra/gateway`](../../infra/gateway/README.md).

## Sondes

- `GET /health` sur 8000 : la gateway répond. Le `HEALTHCHECK` Docker interroge `:8082/ping`.
- La santé des services n'en fait pas partie : chacun est vérifié sur `/health/ready` toutes
  les 10 s ; un service sans instance prête reçoit `503` au lieu d'un timeout.

## Diagnostiquer une réponse

| Réponse | Corps | Cause probable | Action |
|---|---|---|---|
| `404` | `404 page not found` (texte) | Aucune règle : chemin interne, préfixe inconnu, ou route ajoutée sans règle | Vérifier `routes.yml` ; lancer `infra/gateway/tests/run.sh` |
| `404` | enveloppe JSON | Le service a répondu 404 lui-même | Voir le service |
| `413` | texte | Corps > 1 Mio | Normal pour l'API ; les médias passent par S3 |
| `429` | texte | Limite de la gateway (100 req/s par IP) | Si légitime (NAT d'entreprise, test de charge), relever `rate-limit` |
| `429` | enveloppe JSON | Limite du service | Voir le runbook du service |
| `502` | `Bad Gateway` | Service joignable mais connexion coupée | Logs du service, redémarrages |
| `503` | `Service Unavailable` | Aucune instance prête (health check en échec) | `docker compose ps`, `/health/ready` du service |
| `504` | `Gateway Timeout` | Le service n'a pas répondu en 30 s | Requêtes lentes côté service |

Logs : `docker compose logs gateway`. Chaque ligne JSON d'accès donne `RouterName`,
`ServiceName`, `DownstreamStatus`, `Duration` (ns) et `ClientHost`. Les logs `Health check
failed` nomment le service et l'erreur.

## Service qui voit l'IP de la gateway

Les limites par IP d'un service se déclenchent pour tout le monde à la fois : il ne croit pas
la gateway. Vérifier `TRUSTED_PROXIES` (Go) ou `TRUST_PROXY` (Node) dans `docker-compose.yml`
et que le réseau Docker est dans `GATEWAY_TRUSTED_PROXIES`.

## CORS refusé dans un navigateur

Ajouter l'origine exacte (schéma, hôte, port) à `GATEWAY_CORS_ORIGINS` dans `.env`, puis
`docker compose up -d gateway`. L'app mobile n'utilise pas CORS.

## Mise à jour de Traefik

Changer le tag dans `docker-compose.yml`, `infra/gateway/tests/run.sh`,
`infra/gateway/tests/e2e_auth.sh` et le job CI `gateway`, puis lancer les deux scripts de
test. La CI liste les vulnérabilités de l'image sans bloquer.
