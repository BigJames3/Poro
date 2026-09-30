# ADR-0001 — Rôles multiples par compte

- Statut : accepté
- Date : 2026-09-29

## Contexte

Le service auth stockait un rôle unique (`users.role`). Le produit exige qu'un même compte
puisse être à la fois PERSONAL, CREATOR, BUSINESS et ENTERPRISE, et que le personnel
(ADMIN, MODERATOR, SUPPORT) soit distinct.

## Décision

- Table `user_roles (user_id, role, granted_at)`, clé primaire `(user_id, role)`, contrainte
  `CHECK` sur la liste des rôles, FK `ON DELETE CASCADE`.
- Tout compte reçoit `PERSONAL` à la création. Les autres rôles seront attribués par les
  parcours métier (onboarding créateur, ouverture de boutique) via une API interne d'auth.
- Le JWT porte `roles: ["PERSONAL", "CREATOR", …]`. Les rôles sont relus à chaque refresh :
  une attribution est visible en 15 minutes maximum (durée de l'access token).
- Les **permissions** fines (ex. `shop:write`, `order:refund`) sont dérivées des rôles dans le
  code de chaque service, pas stockées dans le token. Une table de permissions ne sera
  introduite que si des rôles personnalisés deviennent nécessaires (ex. employés d'une
  entreprise), afin d'éviter une abstraction sans besoin réel.

## Migration

`000004_create_user_roles_table` recopie le rôle existant de chaque utilisateur, ajoute
`PERSONAL`, puis supprime la colonne. La migration descendante recrée la colonne avec le rôle le
plus privilégié : elle est **avec perte** si un compte avait plusieurs rôles. Testée par
`TestMigrationsUpgradeLegacyDataAndRoundTrip`.

## Conséquences

- Contrat JWT : `role` (chaîne) devient `roles` (tableau). Aucun client ne consommait encore
  l'API.
- Les requêtes utilisateur agrègent les rôles via une sous-requête indexée par la clé primaire.
