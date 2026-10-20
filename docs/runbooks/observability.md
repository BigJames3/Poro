# Runbook — observabilité (PORO-009)

## Démarrer

```bash
docker compose --profile observability up -d prometheus alertmanager grafana
```

| Outil | Adresse (poste local) | Usage |
|---|---|---|
| Prometheus | http://127.0.0.1:9090 | métriques, état des cibles (`Status → Targets`), alertes |
| Alertmanager | http://127.0.0.1:9093 | alertes en cours, silences |
| Grafana | http://127.0.0.1:3300 | tableau « PORO — vue d'ensemble » (lecture seule, sans compte) |

Ces ports n'écoutent que sur `127.0.0.1`. Grafana sans compte est réservé au dev : en staging et
en production, activer l'authentification (SSO de l'hébergeur ou comptes nominatifs) et ne jamais
exposer Prometheus ni Alertmanager publiquement.

## Ce qui est mesuré

| Signal | Métrique | Source |
|---|---|---|
| Trafic, erreurs, latence (RED) | `http_requests_total{service,method,route,status}`, `http_request_duration_seconds` | 10 API (Go, Node, Python), gateway Traefik |
| Disponibilité | `up` | Prometheus (19 cibles) |
| Outbox | `outbox_pending`, `outbox_published_total`, `outbox_publish_failures_total` | 7 relais |
| Consommateurs Kafka | `kafka_consumer_records_total{group,topic,outcome}` : `processed`, `retried`, `dead_lettered` | tous les consommateurs des API Go, Node et Python (le media-worker n'expose pas de métriques : ses échecs se lisent dans `poro.video.failed` et ses logs) |
| Kafka | métriques publiques Redpanda | broker |
| SMS OTP | `auth_sms_sent_total{country}`, `auth_sms_blocked_total{reason}` | auth |
| Processus | mémoire, CPU, GC | tous les services |

Les `/metrics` des services ne sont jamais publiés par la gateway (route interne, testée par
`infra/gateway/tests/run.sh`).

## Logs et corrélation

- Logs JSON structurés hors dev, avec le champ `service`.
- **Requête HTTP** : chaque réponse porte `X-Request-ID` (fourni par le client ou généré), repris
  dans le champ `request_id` de l'access log et des erreurs.
- **Traitement asynchrone** : chaque ligne d'un consommateur porte `consumer_group`, `topic`,
  `partition`, `offset`, `event_id`, `event_type` et `event_subject`. Le sujet est l'entité
  concernée (vidéo, commande, utilisateur…) : chercher sa valeur dans les logs de tous les
  services suit l'entité d'une requête à la fin du flux. `event_id` suit un événement précis ; en
  DLQ, l'en-tête `x-poro-error` donne la cause.
- **Traces** : OpenTelemetry est câblé (`OTEL_EXPORTER_OTLP_ENDPOINT`), sans collecteur en dev.
  La propagation du contexte de trace à travers Kafka (colonne de contexte dans les outbox) reste
  à faire ; d'ici là, la corrélation asynchrone passe par `event_subject` et `event_id`.

### Rédaction des secrets et données personnelles

- Services Go : tout logger passe par `logging.Redact`, qui remplace :
  - la valeur des champs `password`, `token`, `*_token`, `secret`, `authorization`, `cookie`,
    `api_key`, `otp`, `private_key`… ;
  - dans les messages, les erreurs et les chaînes : les JWT, les en-têtes `Bearer`/`Basic`, les
    emails et les numéros au format international.

  Les services gardent leur masquage volontaire (par exemple les derniers chiffres d'un
  téléphone) : la rédaction est le filet de sécurité.
- Services Node : `pino` masque `authorization`, `cookie`, `set-cookie` et, dans tout objet
  journalisé, `password`, `token`, `access_token`, `refresh_token`, `fcm_token`, `otp`,
  `secret`, `phone` et `email`.
- Ne jamais logger un corps de requête complet.

## Alertes (`infra/observability/rules/poro.rules.yml`)

Testées par `promtool test rules` (job CI `observability`). Les seuils sont des points de départ
pour le volume du lancement.

### ServiceDown
Une cible ne répond plus depuis 2 min. Vérifier le conteneur (`docker compose ps`, logs), puis
`/health/ready` du service (dépendance en panne : Postgres, Redis, registre de révocation).

### HighErrorRate
Plus de 5 % de 5xx pendant 10 min, avec un trafic minimal. Chercher les `error` du service et
leur `request_id`. Une dépendance commune (Postgres, Redis) fait monter plusieurs services
ensemble.

### HighLatency
p95 au-dessus de 1 s pendant 10 min. Regarder la route (`route` dans les métriques), la charge
Postgres (requêtes lentes) et les appels externes (SMS, SMTP, S3).

### OutboxBacklog
Plus de 100 événements non publiés pendant 10 min : voir le runbook outbox-relay (broker
injoignable, relais arrêté).

### OutboxPublishFailing
Le relais échoue et ne publie plus rien : broker en panne ou topic absent. Les événements restent
dans l'outbox et partiront au rétablissement ; aucune perte.

### OutboxParked
Kafka a refusé définitivement un événement (trop gros, invalide) : le relais l'a mis à l'écart
(`last_error` commence par `rejected: `), ainsi que les événements suivants de la même entité
(même topic et même clé), pour garder leur ordre. Les autres entités continuent d'être publiées.
Voir le runbook outbox-relay, section « Événements mis à l'écart ».

### EventsDeadLettered
Des événements sont partis en DLQ (`<topic>.dlq`). Lire l'en-tête `x-poro-error` :

```bash
rpk topic consume poro.video.ready.dlq --brokers localhost:9092 -n 5 -f '%h{x-poro-error}\n%v\n'
```

Corriger la cause (donnée invalide : correctif côté producteur ; bug du consommateur : correctif,
puis rejouer en republiant le message sur le topic d'origine). Les inbox rendent le rejeu sûr.

### ConsumerRetrying
Un groupe réessaie en boucle : dépendance indisponible (base, S3). Les messages partiront en DLQ
après 3 essais.

### SMSBudgetBlocking
Plus de 50 SMS OTP bloqués en 15 min par un plafond : attaque de type SMS pumping probable (voir
le runbook auth et l'ADR-0017), ou plafond trop bas pour le trafic réel.

## Recevoir les alertes

En dev, les alertes restent dans Alertmanager. Pour staging et production, remplacer le
récepteur `dev` par le canal d'astreinte (webhook Slack ou Teams, PagerDuty, email). L'URL est un
secret monté en fichier (`url_file`), jamais commité. C'est une **action humaine** : choix du
canal et création du webhook.

## Tests

- `infra/observability/tests/run.sh` (job CI `observability`) :
  - tests unitaires des 9 alertes ;
  - validation des configurations ;
  - démarrage réel de Prometheus, Alertmanager et Grafana, avec 9 règles chargées, Alertmanager
    connecté, 19 cibles, tableau de bord provisionné et Grafana anonyme en lecture seule.
- `packages/shared-go/kafka` : compteurs `processed`, `retried` et `dead_lettered` contre un vrai
  Redpanda.
- `packages/shared-go/logging` : rédaction des champs, messages, erreurs et du logger de chaque
  service.
