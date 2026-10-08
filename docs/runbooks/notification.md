# Runbook — service notification

Port 8086, base `poro_notification`, Redis DB 5, groupe Kafka
`poro-notification-dispatcher`.

## Sondes

`/health/ready` (aussi `/api/v1/health`) vérifie Postgres et Redis. Kafka et FCM
n'en font pas partie : la boîte de notifications reste servie pendant une panne
du broker ou de Firebase.

## Une notification n'arrive pas

1. Le destinataire est-il l'acteur ? Aucune notification dans ce cas.
2. Préférences : `SELECT * FROM preferences WHERE user_id = '<id>';`
   (pas de ligne = tout activé).
3. L'événement a-t-il été traité ?
   `SELECT * FROM inbox_events WHERE event_id = '<id>';`
4. Sinon : lag `rpk group describe poro-notification-dispatcher --brokers localhost:9092`,
   puis la DLQ, par exemple `rpk topic consume poro.social.like.created.dlq`
   (l'en-tête `x-poro-error` donne la cause).
5. Like : il est peut-être regroupé. Chercher
   `group_key LIKE 'like:<video_id>:%'` pour le propriétaire.

## Le push n'arrive pas

1. `FCM_CREDENTIALS_B64` est-il défini ? Sinon le journal de démarrage contient
   `push disabled, in-app only`.
2. Appareils : `SELECT fcm_token, platform, updated_at FROM devices WHERE user_id = '<id>';`
   Aucun appareil = l'application n'a pas appelé `PUT /devices` (ou FCM a
   invalidé le token, journal `invalid fcm tokens removed`).
3. Métrique `notification_pushes_total` par `outcome` :
   `error` = appel FCM en échec (réseau, identifiants) ; `failed` = refus par
   token sans invalidation (quota, payload) ; `invalid_token` = tokens supprimés.
4. Les groupes de likes ne poussent qu'au premier like : c'est voulu.

## Un nom affiché « Quelqu'un »

La projection n'a pas reçu le profil de l'acteur :
`SELECT * FROM user_projections WHERE user_id = '<acteur>';`
Le prochain `poro.user.profile.updated` la complète ; les notifications déjà
écrites gardent leur texte.

## Volume de la base

La purge quotidienne supprime les notifications sans activité depuis
`NOTIFICATION_RETENTION_DAYS` (90 j) et les lignes d'inbox de plus de 30 jours,
par lots de 5 000. Elle journalise `purge done` ou `purge failed`. Exécution
manuelle équivalente :

```sql
DELETE FROM notifications WHERE id IN (
  SELECT id FROM notifications WHERE last_activity_at < now() - interval '90 days' LIMIT 5000);
```

## Rotation des identifiants Firebase

Générer une nouvelle clé du compte de service, mettre à jour
`FCM_CREDENTIALS_B64`, redémarrer le service, puis révoquer l'ancienne clé dans
la console Google Cloud.
