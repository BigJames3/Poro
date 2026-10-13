# Gateway

Point d'entrée unique de l'API PORO : Traefik v3, configuré par fichiers
([ADR-0014](../../docs/architecture/adr/0014-gateway-traefik.md)).

- `traefik.yml` : configuration statique (entrées, timeouts, logs, métriques).
- `routes.yml` : routes, services et middlewares (provider fichier).
- `tests/` : test de contrat du routage et test de bout en bout.

## Démarrage

```powershell
docker compose up -d gateway
curl http://127.0.0.1:8000/health
```

Les clients appellent `http://127.0.0.1:8000/api/v1/...`. Les chemins ne sont pas réécrits :
`/api/v1/orders` arrive tel quel au service order.

## Routage

| Préfixe | Service |
|---|---|
| `/api/v1/auth` | auth |
| `/api/v1/users` | user, sauf `/users/{uuid}/(likes, follow, followers, following, stats)` → social |
| `/api/v1/videos` | video, sauf `/videos/{id}/(like, likes, stats, comments, shares)` → social |
| `/api/v1/comments` | social |
| `/api/v1/feed` | feed |
| `/api/v1/search` | search |
| `/api/v1/notifications` | notification |
| `/api/v1/moderation` | moderation |
| `/api/v1/shops`, `/api/v1/products`, `/api/v1/admin/shops` | shop |
| `/api/v1/cart`, `/api/v1/checkout`, `/api/v1/orders`, `/api/v1/addresses`, `/api/v1/seller` | order |
| `/health` | la gateway elle-même |

Tout le reste répond 404 : `/metrics`, `/health/*` et `/api/v1/health` des services, le JWKS.
Un préfixe couvre des segments entiers (`/api/v1/authx` n'est pas auth).

**Ajouter une route** : documenter le chemin dans `docs/api/<service>.openapi.yaml`, puis, si
son préfixe est nouveau ou partagé, ajouter la règle dans `routes.yml`. Le test de contrat lit
les OpenAPI : une route documentée qui n'atteint pas son service fait échouer la CI.

## Ce que fait la gateway

| Middleware | Réglage |
|---|---|
| Limite de débit par IP | 100 req/s en moyenne, rafales de 200 ; large à cause du CGNAT mobile, les services gardent leurs limites fines |
| Taille des requêtes | 1 Mio (les médias passent par des URL S3 présignées) → `413` au-delà |
| Timeouts | lecture 30 s, écriture 60 s, attente de la réponse du service 30 s, connexion 5 s |
| Compression | gzip, brotli ou zstd selon le client, au-delà de 1 Kio |
| En-têtes de sécurité | `nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: no-referrer`, pas de `Server`/`X-Powered-By` |
| CORS | origines exactes de `GATEWAY_CORS_ORIGINS` (vide = aucune) |
| `X-Forwarded-For` | celui du client est ignoré ; la gateway pose le sien |

Les services vérifient eux-mêmes les JWT (JWKS) : la gateway ne lit pas les tokens. Elle vérifie
`/health/ready` de chaque service toutes les 10 s et répond `503` quand aucun n'est prêt.

Les services font confiance à la gateway pour l'IP du client : `TRUSTED_PROXIES` (Go) et
`TRUST_PROXY` (Node) valent `GATEWAY_TRUSTED_PROXIES` (réseau Docker privé). moderation ignore
volontairement `X-Forwarded-For` (`proxy_headers=False`).

Logs d'accès JSON sur la sortie standard, sans `Authorization` ni `Cookie`. Métriques
Prometheus sur le port interne 8082 (`/metrics`), non publié.

## Variables

| Variable | Défaut | Rôle |
|---|---|---|
| `GATEWAY_CORS_ORIGINS` | *(vide)* | Origines web autorisées, séparées par des virgules |
| `GATEWAY_TRUSTED_PROXIES` | `172.16.0.0/12,192.168.0.0/16` | D'où les services acceptent `X-Forwarded-For` |

## Tests

```bash
infra/gateway/tests/run.sh        # contrat : toutes les routes des OpenAPI, 404, 413, CORS, gzip, 429
infra/gateway/tests/e2e_auth.sh   # vrai service auth derrière la gateway (Go, Docker, openssl)
```

`run.sh` démarre Traefik avec la vraie configuration et un `traefik/whoami` par service : la
réponse nomme le service qui a répondu.

## Limites

- Pas de TLS en local ; TLS, HSTS et ingress de staging viendront avec P0-12.
- Limite de débit en mémoire, par instance ; la vraie défense contre les abus sera Cloudflare.
- Les `404`, `413`, `429` et `503` de Traefik sont en texte brut, pas dans l'enveloppe JSON.
- `traefik:v3.7.14` embarque des vulnérabilités HIGH corrigées en amont (Go 1.26.9,
  `x/net` 0.60.0) sans release Traefik corrigée à ce jour : la CI les liste sans bloquer. Mettre
  à jour le tag dès qu'une version corrigée sort.
