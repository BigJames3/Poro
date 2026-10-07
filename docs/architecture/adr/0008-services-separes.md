# ADR-0008 — Un déployable par domaine pour social, feed et notification

- Statut : accepté
- Date : 2026-10-07
- Remplace : [ADR-0003](0003-frontieres-services.md)

## Contexte

ADR-0003 regroupait VIDEO, SOCIAL et FEED dans un déployable Go `content`, et
prévoyait NOTIFICATION en Go. Le plan produit (`.cursor/rules/project.md`) et la
feuille de route des vagues 1 à 4 prévoient au contraire un service par domaine :
`social` et `feed` en Go/Fiber, `notification` en NestJS. `video` est déjà livré
seul (ADR-0007).

## Décision

| Service | Langage | Port | Base |
|---|---|---|---|
| `social` | Go + Fiber | 8085 | `poro_social` |
| `feed` | Go + Fiber | 8084 | `poro_feed` |
| `notification` | NestJS | 8086 | `poro_notification` |

Règles inchangées depuis ADR-0003 et ADR-0005 :
- une base PostgreSQL par service, aucune jointure entre bases ;
- communication par événements Kafka (JSON Schema, outbox, inbox, DLQ) ;
  chaque service maintient ses propres projections des données des autres.

## Conséquences

- Plus de pipelines CI, d'images et de bases que prévu par ADR-0003 : chaque
  service a son job CI filtré par chemin pour limiter le coût.
- `feed` lit les vidéos par projection de `poro.video.ready` / `poro.video.deleted`,
  d'où l'ajout rétro-compatible de `title`, `description`, `hashtags`,
  `published_at` à `poro.video.ready` et la création de `poro.video.deleted`.
- `notification` construit ses noms d'acteurs à partir de
  `poro.user.profile.updated`, publié par `user`.
