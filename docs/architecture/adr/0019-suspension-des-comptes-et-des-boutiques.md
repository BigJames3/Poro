# ADR-0019 — Suspension d'un compte et d'une boutique dans tous les services (PORO-021)

- Statut : accepté (décisions du propriétaire du 2026-10-22)
- Date : 2026-10-22
- Complète : [ADR-0016](0016-revocation-des-sessions.md)

## Contexte

- Auth connaît le statut `SUSPENDED` : il refuse la connexion et le renouvellement d'un compte
  suspendu. Mais aucune route ne permettait de suspendre un compte, et les autres services vérifient
  les jetons d'accès hors ligne : un jeton émis avant une suspension restait accepté jusqu'à 15 minutes.
- Une boutique suspendue par la modération (`POST /api/v1/admin/shops/{id}/suspend`) disparaît du
  catalogue, mais son vendeur pouvait encore expédier et annuler ses commandes dans le service order.

## Décisions

### Compte suspendu : révocation des sessions, pas de statut copié dans chaque service

1. `POST /api/v1/auth/admin/users/{id}/suspend` met le compte en `SUSPENDED`, puis fait entrer
   **toutes ses sessions** dans le registre de révocation partagé (ADR-0016) et révoque ses refresh
   tokens.
   - Chaque service lit déjà ce registre à chaque requête : le compte est refusé partout
     immédiatement (401), y compris en lecture.
   - Auth refuse ensuite toute nouvelle connexion et tout renouvellement (403 `account_blocked`).
   - Les sessions sont révoquées même si le compte était déjà suspendu : un appel interrompu se
     termine en le répétant.
2. `POST /api/v1/auth/admin/users/{id}/reinstate` remet le compte en `PENDING` : la connexion
   suivante le rend `ACTIVE`, comme pour tout compte pas encore connecté.
3. **Rôles ADMIN et MODERATOR**, vérifiés par auth.
   - Un MODERATOR ne vise jamais un ADMIN ni un autre MODERATOR (403 `forbidden`).
   - Personne ne se suspend soi-même (422 `self_suspension`).
   - Chaque suspension et chaque rétablissement sont journalisés : acteur, rôles de l'acteur, cible,
     horodatage. La piste d'audit complète et la gestion des rôles relèvent de PORO-024.

Alternatives écartées :

- **Copier le statut dans chaque service** (événement `poro.auth.user.suspended` consommé partout) :
  dix consommateurs et projections à maintenir, avec une fenêtre d'incohérence. La révocation donne
  le même résultat avec un mécanisme déjà testé.
- **Contrôle synchrone auprès d'auth à chaque requête** : un appel réseau par requête, et auth
  devient un point de panne de toute l'API.

### Boutique suspendue : contrôle dans order

4. Order refuse l'expédition et l'annulation côté vendeur d'une boutique suspendue
   (403 `shop_suspended`). Le statut vient de sa copie locale du catalogue
   (`catalog_shops`, alimentée par `poro.shop.shop.updated`).
5. L'acheteur garde toutes ses actions : annuler une commande pas encore expédiée, confirmer
   la réception.

## Commandes en cours d'une boutique suspendue

Décision du propriétaire : les commandes pas encore expédiées sont annulées automatiquement
(motif `shop_suspended`), avec remboursement une fois Wave en service ; les commandes expédiées
restent à l'acheteur. Livré dans une PR séparée.

## Conséquences

- Un compte suspendu est bloqué dans tous les services en moins d'une seconde, sans code nouveau
  dans les services métier.
- Si le registre de révocation est indisponible, l'appel échoue (500). Le compte est déjà
  `SUSPENDED` (connexion et renouvellement refusés), mais ses jetons d'accès en cours restent
  acceptés au plus 15 minutes : relancer l'appel termine la suspension.
