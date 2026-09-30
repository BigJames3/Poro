# ADR-0003 — Frontières des services déployables

- Statut : accepté (révisable quand la charge ou l'équipe l'exigeront)
- Date : 2026-09-29

## Contexte

Les règles initiales prévoient 13 services en Go, NestJS et Python. Pour une petite équipe, chaque
service ajoute un pipeline CI, un déploiement, des alertes, une base et des mises à jour de
dépendances. Les domaines métier restent pourtant nécessaires pour garder un couplage faible.

## Décision

Les **domaines** restent séparés (package, schéma de données, événements, tests). Les
**déployables** sont regroupés tant que l'isolation, la sécurité et la charge le permettent :

| Déployable | Langage | Domaines | Justification |
|---|---|---|---|
| `auth` | Go | AUTH | Sécurité : surface minimale, secrets isolés |
| `user` | NestJS | USER | Logique métier de profil et d'onboarding |
| `content` | Go | VIDEO (API), SOCIAL, FEED | Chemins de lecture chauds et partagés, latence critique |
| `media-worker` | Go + FFmpeg | VIDEO (traitement) | Profil CPU très différent : doit scaler seul |
| `commerce` | NestJS | SHOP, PRODUCT, ORDER, PAYMENT, DELIVERY | Monolithe modulaire transactionnel ; PAYMENT extractible si la conformité l'exige |
| `notification` | Go | NOTIFICATION | Consommateur Kafka, envoi FCM/APNs |
| `moderation`, `reco`, `analytics` | Python | idem | Plus tard (P2), écosystème ML |

Règles :
- une base PostgreSQL **par déployable** sur un même cluster au départ ; aucune jointure entre
  bases ; communication par API ou événements ;
- dans un déployable, chaque domaine a son propre schéma SQL et n'accède pas aux tables d'un autre ;
- extraction d'un domaine en service dédié quand sa charge, son équipe ou ses exigences de
  sécurité divergent.

## Conséquences

- 6 déployables au MVP au lieu de 13.
- La structure `services/` suivra ces noms à la création de chaque service.
- Les règles Cursor (`.cursor/rules/project.md`) doivent être alignées sur cette décision.
