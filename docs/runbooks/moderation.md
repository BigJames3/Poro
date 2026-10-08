# Runbook — service moderation

Port 8087, base `poro_moderation`, Redis DB 6, groupe Kafka
`poro-moderation-scanner`, relais `outbox-relay-moderation`.

## Sondes

`/health/ready` (aussi `/health` et `/api/v1/health`) vérifie Postgres et
Redis. Kafka n'en fait pas partie : signalements et file restent servis
pendant une panne du broker.

## Un contenu n'a pas été filtré

1. Texte reçu ? `SELECT body, source_at FROM content_snapshots WHERE target_id = '<id>';`
2. Absent : lag `rpk group describe poro-moderation-scanner --brokers localhost:9092`,
   DLQ `rpk topic consume poro.social.comment.created.dlq` (en-tête `x-poro-error`).
3. Présent mais autorisé : aucun terme ne correspond. Ajouter le terme dans
   `src/moderation/rules/fr.yml` et redéployer (les contenus déjà vus ne sont
   pas rescannés).

## Un contenu a été retiré à tort

`SELECT id, status, auto_matches FROM cases WHERE target_id = '<id>';` montre
la règle qui a déclenché. Une vidéo se restaure par
`POST /api/v1/moderation/cases/{id}/decision` `{"action": "restore"}`
(rôle MODERATOR ou ADMIN). Un commentaire retiré ne peut pas être restauré en
v1. Corriger ensuite la liste de termes.

## Le retrait n'est pas appliqué

1. Événement en attente ? `SELECT published_at, attempts, last_error FROM outbox_events WHERE event_key = '<id>';`
2. `published_at` nul : `outbox-relay-moderation` est-il démarré ?
3. Publié : vérifier feed (`moderation_status`), social (`status = 'removed'`)
   et notification (voir leurs runbooks).

## Volume

Les tables croissent avec les signalements et les textes vus ; aucune purge
n'existe encore en v1. Le relais purge l'outbox publiée.
