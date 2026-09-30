# ADR-0005 — Événements JSON, outbox transactionnel, relais et DLQ

- Statut : accepté
- Date : 2026-09-30
- Décideurs : ingénierie

## Contexte

Auth crée des comptes. User crée des profils et active les créateurs. Les deux
doivent se parler sans appel HTTP synchrone (couplage, indisponibilité, timeouts).
Les règles du projet citent Avro ; aucun registre de schémas n'est en place.

## Décision

1. **JSON versionné**, pas Avro, pour le MVP. Enveloppe commune :

   `{id, type, version, source, subject, occurred_at, data}`

   `id` est un UUIDv7. `type` = nom du topic Kafka = `poro.{domaine}.{entité}.{action}`.
   Clé Kafka = `subject` (id d'agrégat) pour garder l'ordre par compte.
   Un changement additif de `data` conserve `version` ; un changement cassant
   l'incrémente. Les schémas vivent dans `packages/contracts/events`.

2. **Outbox transactionnel** dans chaque base de service (`outbox_events`).
   L'effet métier et l'événement committent ensemble. Un binaire
   `services/outbox-relay` (un processus par base) publie vers Redpanda
   (`acks=all`, idempotence producteur). Livraison **at-least-once**.

3. **Inbox** `processed_events (consumer, event_id)` : le consommateur
   enregistre l'id dans la même transaction que l'effet. Un doublon est un no-op.

4. **Pas d'auto-création de topics.** Un job compose `rpk topic create` les
   déclare, y compris `<topic>.dlq`. Après 3 tentatives (ou une erreur
   permanente : JSON illisible, schéma inconnu) le record va sur la DLQ avec
   les en-têtes `x-poro-error`, `x-poro-original-topic`, partition, offset,
   consumer-group. Le commit n'a lieu qu'après traitement ou park.

5. **Vérification JWT hors-ligne via JWKS.** Un logout (blacklist Redis côté
   auth) n'est pas vu des autres services. Un access token révoqué reste
   accepté jusqu'à 15 minutes. Accepté : logout n'est pas un kill-switch
   immédiat ; le refresh token, lui, est révoqué tout de suite.

## Conséquences

- Avro + Schema Registry restent possibles plus tard sans changer les topics
  (nouveau `version` ou suffixe `.v2`).
- Redis n'est jamais source de vérité des événements.
- Un courtier down n'empêche pas le login : Kafka n'est pas dans `/health/ready`
  d'auth ni de user. L'outbox accumule ; le relais rattrape.
- Rejouer une DLQ est une opération manuelle (voir le runbook relais).
