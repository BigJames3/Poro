# ADR-0013 — Cycle de vie du compte : sessions par appareil et suppression

- Statut : accepté
- Date : 2026-10-12

## Contexte

Les sessions par appareil existent depuis l'ADR-0002 (une famille de refresh tokens par
connexion, claim `sid`), sans API pour les voir ni les couper. Aucun utilisateur ne peut
supprimer son compte, alors que les stores l'exigent et que l'effacement des données
personnelles est attendu. Les comptes s'ouvrent par téléphone (OTP) ou par email et mot de
passe ; auth n'envoie pas encore d'email. Les autres services vérifient les access tokens
hors ligne avec le JWKS.

## Décisions

1. **Sessions** : `GET /sessions` liste les familles qui ont encore un refresh token valide ;
   `DELETE /sessions/:id` et `POST /sessions/revoke-others` révoquent des familles. L'app peut
   nommer l'appareil à la connexion (`device_name`, 60 caractères, nettoyé) ; le nom suit les
   rotations.
2. **Pas de révocation des access tokens entre services** : un token déjà émis reste valide
   jusqu'à son expiration (15 min au plus), comme pour le logout de l'ADR-0002. Auth seul
   blackliste le `jti` courant.
3. **Suppression en deux temps** : `POST /me/delete` programme l'effacement à J+30
   (`ACCOUNT_DELETION_GRACE`) et déconnecte tous les appareils. Se reconnecter avant
   l'échéance l'annule ; `POST /me/delete/cancel` aussi. Le délai protège des suppressions
   accidentelles et d'un vol de session.
4. **Nouvelle preuve obligatoire** : mot de passe pour un compte email, code OTP frais pour un
   compte téléphone. La route est limitée comme une connexion (10/min par IP).
5. **Effacement** : une tâche planifiée (`FOR UPDATE SKIP LOCKED`, sûre sur plusieurs
   réplicas) anonymise le compte dans une transaction : téléphone, email, mot de passe et pays
   effacés, rôles, sessions et codes OTP supprimés, statut `DELETED`. La ligne et l'id restent
   (références dans les autres services) ; la contrainte d'identifiant accepte désormais un
   compte `DELETED` sans téléphone ni email. Le numéro et l'email redeviennent libres.
6. **Propagation** : `poro.auth.user.deleted` v1 (`user_id`, `deleted_at`), sans donnée de
   contact, part par l'outbox dans la même transaction. Chaque service l'appliquera dans sa
   propre PR (profil, vidéos, social, notifications, boutique et commandes).

## Conséquences

- Pendant les 30 jours, le compte reste visible dans les autres services : seul auth sait
  qu'il va disparaître.
- Un vendeur qui supprime son compte garde sa boutique tant que shop ne consomme pas
  `poro.auth.user.deleted` ; les commandes en cours seront traitées dans cette PR de shop.
- Après l'effacement, rien n'est récupérable ni restaurable par le support.

## Alternatives écartées

- Suppression immédiate : irréversible au moindre doigt qui glisse, ou pour un voleur de
  session.
- Liste noire des `sid` partagée entre services : un appel réseau ou un cache Redis par
  requête dans chaque service, pour gagner 15 minutes au plus.
- Supprimer la ligne `users` : casserait les références et l'historique côté inbox.
