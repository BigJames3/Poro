# Moderation Service

Modération PORO v1 ([ADR-0010](../../docs/architecture/adr/0010-moderation-v1-vague-2.md)) :
signalements des utilisateurs, filtre automatique par règles et file des
modérateurs. Premier service Python du dépôt (FastAPI, SQLAlchemy async,
Alembic, aiokafka).

- Contrat HTTP : [`docs/api/moderation.openapi.yaml`](../../docs/api/moderation.openapi.yaml)
- Exploitation : [`docs/runbooks/moderation.md`](../../docs/runbooks/moderation.md)
- Contrats d'événements : [`packages/contracts/events`](../../packages/contracts/events)

## Événements

| Sens | Topic | Effet |
|---|---|---|
| Consommé | `poro.video.ready` | Titre, description et hashtags filtrés |
| Consommé | `poro.social.comment.created`, `poro.social.comment.updated` | Texte complet filtré (à défaut, l'extrait des anciens événements) |
| Consommé | `poro.user.profile.updated` | Nom affiché et username filtrés ; jamais de retrait automatique d'un compte |
| Produit | `poro.moderation.content.removed` | Vidéo ou commentaire retiré (`decided_by` : `auto` ou `moderator`) |
| Produit | `poro.moderation.content.restored` | Vidéo restaurée par un modérateur |

Groupe `poro-moderation-scanner` : inbox `processed_events` dans la même
transaction que l'effet, 3 essais puis `<topic>.dlq` (en-têtes `x-poro-*`
identiques à shared-go). Les événements produits passent par l'outbox et
`outbox-relay-moderation`. Feed, social et notification appliquent les retraits.

## Filtre automatique

Règles déterministes, listes dans [`src/moderation/rules/fr.yml`](src/moderation/rules/fr.yml)
(redéploiement pour les modifier) :

- texte normalisé (minuscules, sans accents, leetspeak `0→o 1→i 3→e 4→a 5→s 7→t @→a $→s`,
  lettres répétées réduites, mots épelés « c o n n a r d » recollés) ;
- termes `block` (haine, menaces, sollicitation sexuelle, arnaques) : **retrait immédiat** ;
- numéro de téléphone (CI, SN, CM) avec un mot de paiement mobile : retrait (`fraud`) ;
- termes `review` (insultes…), plus de 2 liens, numéro seul, plus de 70 % de
  majuscules, répétitions : dossier en file, priorité haute ;
- un compte n'est jamais retiré : un verdict `block` sur un profil devient `review`.

Un dossier restauré par un humain n'est plus retiré par le filtre (il repasse
seulement en priorité haute). Un dossier classé repasse en file si un nouveau
texte de la même cible est signalé par le filtre.

## API

| Méthode | Chemin | Accès | Limite |
|---|---|---|---|
| `POST` | `/api/v1/moderation/reports` | tout compte | 10 / heure, 120 / min |
| `GET` | `/api/v1/moderation/cases` (`status`, `target_type`, `priority`, `cursor`, `limit` 1–50) | MODERATOR, ADMIN | 120 / min |
| `GET` | `/api/v1/moderation/cases/{id}` | MODERATOR, ADMIN | 120 / min |
| `POST` | `/api/v1/moderation/cases/{id}/decision` (`remove`, `dismiss`, `restore`) | MODERATOR, ADMIN | 120 / min |
| `GET` | `/health`, `/health/live`, `/health/ready`, `/api/v1/health`, `/metrics` | — | — |

- Un second signalement de la même cible par la même personne renvoie le
  premier (`200`, `created: false`) ; le premier répond `201`.
- `REPORT_THRESHOLD` signalements distincts (3) passent le dossier en priorité haute.
- On ne peut pas se signaler soi-même (`422 cannot_report_self`).
- `remove` exige un motif ; un compte ne peut être que classé
  (`422 action_not_allowed`) ; un commentaire retiré ne peut pas être restauré
  (`422 restore_not_supported`) ; une transition impossible répond `409 invalid_transition` ;
  un contenu pas encore reçu par la modération ne peut pas être retiré (`409 target_unknown`).

## Variables d'environnement

| Variable | Défaut | Rôle |
|---|---|---|
| `APP_ENV` | `dev` | `dev`, `staging`, `prod` (hors dev : TLS Postgres et Redis obligatoires) |
| `PORT` | `8087` | Port HTTP |
| `LOG_LEVEL` | `DEBUG` en dev, `INFO` sinon | Logs JSON sur la sortie standard |
| `DATABASE_URL` | — | `postgresql://…/poro_moderation?sslmode=…` |
| `REDIS_URL` | vide | Limites de débit partagées (DB 6) ; en mémoire si vide (dev) |
| `JWKS_URL` | `http://localhost:8081/.well-known/jwks.json` | Clés publiques d'auth |
| `KAFKA_BROKERS` | `localhost:9092` | Brokers |
| `KAFKA_CONSUMER_ENABLED` | `true` | `false` : HTTP seul |
| `REPORT_THRESHOLD` | `3` | Signalements distincts pour la priorité haute |
| `RULES_PATH` | `rules/fr.yml` du paquet | Listes de termes |

## Démarrage local

```powershell
docker compose up -d moderation-migrate moderation outbox-relay-moderation
```

Depuis `services/moderation` (Postgres et Redpanda déjà démarrés) :

```powershell
Copy-Item .env.example .env
uv sync
uv run alembic upgrade head
uv run python -m moderation
```

## Tests

```powershell
uv run ruff format --check src tests migrations
uv run ruff check src tests migrations
uv run mypy
uv run pytest --cov
uv run pip-audit --skip-editable
```

Les tests démarrent Postgres et Redpanda via testcontainers. Couverture
minimale 70 %.

## Limites

- Pas d'analyse d'image ni de vidéo (pas de ML en v1).
- Les médias d'une vidéo retirée restent servis par l'API video et le stockage
  jusqu'à la PR video de quarantaine (prochaine étape).
- Aucune sanction de compte (bannissement : auth), pas d'appel utilisateur, pas
  de notification « contenu retiré » à l'auteur.
- Listes en français uniquement ; langues locales (wolof, nouchi…) à compléter.
- Pas de traces OpenTelemetry (logs JSON et métriques Prometheus seulement).
