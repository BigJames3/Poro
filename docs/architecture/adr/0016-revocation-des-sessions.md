# ADR-0016 — Révocation des sessions dans tous les services

- Statut : accepté
- Date : 2026-10-15
- Révise : ADR-0002 (logout par `jti`) et ADR-0013 (décision 2 et alternative « liste noire des `sid` »)

## Contexte

Un access token est un JWT RS256 de 15 minutes portant un identifiant de session (`sid`, la
famille de refresh tokens d'un appareil). L'audit du 14/10/2026 (PORO-003) a constaté que :

- auth ne refusait que le jeton exact mis en liste noire (`jti`), et seulement à la déconnexion
  ou à la révocation de sa propre session ;
- « déconnecter les autres appareils », une session révoquée depuis un autre appareil, la
  réinitialisation du mot de passe, la demande de suppression du compte et la réutilisation d'un
  refresh token (signe de vol) ne révoquaient que les refresh tokens : l'access token restait
  valide jusqu'à 15 minutes ;
- les 10 autres services ne vérifiaient que la signature et l'expiration : aucune révocation ne
  les atteignait, pas même la déconnexion.

Pour Poro, ces 15 minutes sont exactement la fenêtre utile à un voleur de session après qu'une
victime a réinitialisé son mot de passe. Chaque service dispose déjà d'un Redis (limites de
débit).

## Décisions

1. **Un registre des sessions révoquées, partagé** : une clé Redis par session,
   `poro:revoked:sid:<sid>` (UUID en minuscules), dans une base dédiée (`/15` en local). Auth
   l'écrit, tous les services la lisent. Une entrée vit autant qu'un access token, plus une
   minute de marge pour les écarts d'horloge : le registre ne contient jamais plus que les
   sessions révoquées pendant les 16 dernières minutes.

2. **Révoquer une session plutôt qu'un jeton.** La liste noire par `jti` est supprimée. Entrent
   dans le registre :

   | Événement | Sessions révoquées |
   |---|---|
   | Déconnexion | la session courante |
   | Révocation d'une session (`DELETE /sessions/:id`) | la session visée, si elle appartient au compte |
   | « Déconnecter les autres appareils » | chaque autre session active |
   | Réinitialisation du mot de passe | toutes les sessions actives, avant le changement de mot de passe |
   | Demande de suppression du compte | toutes les sessions actives et la session courante |
   | Réutilisation d'un refresh token | la session volée |

   « Révoquer tout le compte » revient à révoquer toutes ses sessions actives : il n'y a ni
   horodatage par compte ni dépendance à l'horloge ou à la date d'émission des jetons.

3. **Registre d'abord, base ensuite.** Auth écrit le registre avant de révoquer les refresh
   tokens. Si le registre échoue, rien n'a changé et l'appel peut être rejoué. Si la base échoue
   ensuite, la session est déjà refusée partout ; un refresh produirait un jeton portant le même
   `sid`, lui aussi refusé. Pour la réutilisation d'un refresh token, l'écriture a lieu dans la
   transaction, qui est annulée si le registre échoue.

4. **Refuser plutôt qu'accepter sans vérifier.** Si le registre ne répond pas, la requête reçoit
   `503 unavailable`, comme quand les clés JWKS sont indisponibles. Les clients Redis du registre
   ont des délais courts (connexion 1 s, commande 0,5 s, une seule nouvelle tentative) pour
   qu'une panne échoue vite.

5. **Obligatoire hors dev.** `REVOCATION_REDIS_URL` est exigée par auth dans tous les
   environnements (défaut dev : `redis://localhost:6380/15`) et par les autres services hors dev.
   Un service de dev lancé seul sur le poste peut s'en passer : il démarre avec un avertissement
   et ne vérifie pas la révocation.

6. **Une implémentation par langage, un seul contrat.**
   - Go : `packages/shared-go/revocation` (écriture et lecture), branché dans `jwtauth` ;
   - Node : `src/auth/revocations.ts` dans user, shop, order et notification ;
   - Python : `moderation/http/revocations.py`.

   Chacune est testée contre la clé littérale, que l'auth écrit avec le paquet Go.

## Conséquences

- Une lecture Redis (`EXISTS`) par requête authentifiée, dans chaque service.
- Une panne du Redis du registre rend toute l'API authentifiée indisponible (503). La
  supervision (PORO-009) devra alerter sur la santé du registre ; les healthchecks de
  disponibilité des services l'incluent (`revocation`).
- En production, le registre devra être restreint par ACL Redis : auth en écriture, les autres
  services en lecture seule sur `poro:revoked:*` (P0-12).
- La durée de vie des access tokens reste de 15 minutes : la révocation ne dépend plus d'elle.
- Hors périmètre : la révocation d'un rôle (prise en compte au refresh suivant, PORO-024) et la
  suspension d'un compte par un administrateur, qui n'a pas encore d'endpoint.

## Alternatives écartées

- **Révocation dans auth seulement, avec des jetons de 5 minutes** : trois fois plus de
  rafraîchissements sur des réseaux mobiles lents et facturés à la donnée, et une session volée
  reste utilisable jusqu'à 5 minutes dans tous les autres services.
- **Événements Kafka et liste noire en mémoire dans chaque service** : pas de Redis sur le
  chemin des requêtes, mais trois implémentations complexes, une liste perdue à chaque
  redémarrage et un délai de propagation.
- **Horodatage « jetons émis avant » par compte** : dépend des horloges et de la seconde
  d'émission ; la révocation des sessions actives couvre les mêmes cas.
