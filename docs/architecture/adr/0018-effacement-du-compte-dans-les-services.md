# ADR-0018 — Effacement d'un compte dans tous les services (PORO-002)

- Statut : accepté (règles des commandes et conservation légale : **à valider par le juridique**)
- Date : 2026-10-15
- Complète : [ADR-0013](0013-cycle-de-vie-du-compte.md)

## Contexte

Auth efface un compte 30 jours après la demande et publie `poro.auth.user.deleted` v1
(`user_id`, `deleted_at`) par l'outbox. Aucun service ne le consommait : le profil, les vidéos,
les commentaires, les adresses et les jetons push restaient servis. L'objectif est qu'un compte
effacé ne laisse plus aucune donnée personnelle servie par l'API, et qu'on ne garde que ce
qu'une obligation légale ou technique impose.

## Décisions

### Principes communs

1. **Chaque service consomme l'événement lui-même**, dans un groupe dédié
   `poro-<service>-account-erasure`, avec les mécanismes existants :
   - inbox `processed_events` (ou `inbox_events` pour notification) réclamée dans la même
     transaction que l'effacement ;
   - 3 essais avec attente exponentielle, puis DLQ `poro.auth.user.deleted.dlq` ;
   - un événement mal formé part directement en DLQ.

   Aucun service ne dépend de l'ordre d'arrivée chez un autre.
2. **Effets idempotents** : rejouer l'événement, ou le recevoir après un effacement partiel, ne
   change rien de plus. Les suppressions d'objets S3 ont lieu **après** la validation de la
   transaction et ignorent les objets déjà absents. Une erreur S3 fait échouer le message, qui
   est réessayé puis part en DLQ. Comme pour la modération vidéo, l'inbox est alors réclamée
   seulement une fois le stockage traité.
3. **Réutiliser les événements de domaine existants** pour la cohérence en aval : chaque vidéo
   effacée publie `poro.video.deleted`, chaque like, commentaire ou abonnement retiré publie son
   `*.deleted`, chaque commande annulée publie `poro.order.order.cancelled`. Feed, search et
   notification mettent ainsi à jour compteurs et index avec le code déjà testé.
4. **L'identifiant seul n'est pas une donnée de contact.** Auth garde la ligne du compte avec
   son id (ADR-0013). Les autres services peuvent garder l'id opaque là où il est nécessaire
   (audit, comptabilité, intégrité) ; ces tables sont listées ci-dessous. Tout le reste (nom,
   pseudo, photo, texte, adresse, téléphone, jeton d'appareil) est effacé.
5. **Données libérées** : comme le téléphone et l'email dans auth, le pseudo redevient
   disponible.

### Par service

| Service | Effacé | Gardé (justification) |
|---|---|---|
| user | profil anonymisé : `username`, `display_name`, `bio`, `avatar_key`, `country_code` à `NULL`, `is_creator` faux, `deleted_at` posé ; avatar supprimé du stockage. `GET /users/by-username/<ancien>` et `GET /users/<id>` → 404 | ligne `profiles` avec l'id seul et `deleted_at` (empêche une recréation par un `user.created` rejoué) |
| video | toutes les vidéos du compte, quel que soit leur statut : supprimées logiquement (titre et description vidés), `poro.video.deleted` publié pour chacune, uploads multipart abandonnés, médias mis en quarantaine par le consommateur existant, sources supprimées | lignes `videos` sans texte (id et clés, nécessaires à la quarantaine et aux tombstones) |
| social | likes, likes de commentaires, partages et abonnements dans les deux sens supprimés, avec compteurs et événements `*.deleted` ; commentaires supprimés logiquement avec texte vidé (`comment.deleted`) ; projection utilisateur et compteurs supprimés | lignes `comments` sans texte (les réponses des autres gardent leur fil) |
| feed | profil d'auteur, abonnements dans les deux sens (`RemoveFollow`, idempotent), compteur d'abonnés. Vidéos : via `video.deleted` | rien |
| search | utilisateur désindexé (ligne supprimée et jamais recréée). Vidéos : via `video.deleted` | rien |
| shop | boutique fermée **quel que soit son statut** (y compris suspendue), description et logo effacés, produits archivés (`product.updated` publié), images supprimées du stockage, uploads en attente supprimés | nom et `handle` de la boutique (identité commerciale des commandes passées, obligation comptable) |
| order | carnet d'adresses, panier, aperçus de paiement supprimés. Commandes **non expédiées** (`pending`, `awaiting_payment`, `confirmed`, `paid`) où le compte est acheteur ou vendeur : annulées avec la raison `account_deleted` (le stock est libéré par shop ; `paid: true` déclenche le remboursement). Contacts de livraison effacés sur toutes ses commandes terminées ou annulées | commandes avec ids, montants, lignes et statuts (obligation comptable, durée **à valider**) ; commandes `shipped` qui se terminent normalement, contact effacé à leur clôture par la tâche existante |
| notification | appareils (jetons FCM), préférences, notifications reçues, notifications dont le compte est l'acteur, projection utilisateur | rien |
| moderation | signalements faits par le compte : `reporter_id` remplacé par un UUID aléatoire (pseudonymisation non réversible), commentaire vidé ; contenus du compte : texte des instantanés vidé, `owner_id` à `NULL` | dossiers et décisions (journal de modération, obligation de traçabilité), avec ids opaques |

### Commandes en cours (règle retenue, conservatrice)

- **Non expédiée → annulée.** Un acheteur effacé ne peut plus être livré ni contacté ; un
  vendeur effacé ne peut plus expédier. L'annulation libère le stock et déclenche le
  remboursement des commandes payées, par le chemin existant.
- **Expédiée → terminée normalement.** Le colis est parti : l'annuler priverait le vendeur d'un
  paiement dû. Le contact est effacé à la clôture, comme pour toute commande.
- La durée de conservation des commandes (obligation comptable, 10 ans dans l'espace OHADA
  selon l'acte uniforme comptable) et la base légale du maintien des ids sont **à confirmer par
  le juridique**. Le code ne garde que l'id, les montants et les lignes.

### Précisions d'implémentation

- **Tombstones `erased_users`** (id et date seulement) dans social, feed, search, notification et
  moderation. Sans elles, un événement tardif ou rejoué (`user.created`, `profile.updated`,
  `follow.created`, un like) recréerait la projection ou une notification. Tout chemin
  d'écriture de ces services ignore un id effacé. Dans user, la ligne `profiles` avec
  `deleted_at` joue ce rôle.
- **Ordre base / stockage** : user, shop et video valident la base, suppriment les objets, puis
  réclament l'inbox. Une panne S3 laisse l'événement non réclamé, et la reprise retrouve les
  objets par préfixe (user, shop) ou à partir des lignes (video).
- **Course scan / effacement** dans moderation : verrou consultatif par compte, partagé par le
  scanner et exclusif pour l'effacement.
- **Commandes expédiées** : le contact est effacé par la tâche de rétention existante,
  `CONTACT_RETENTION_DAYS` après la clôture, et non à la clôture même.
- **Boutique** : les produits archivés gardent titre et variantes (catalogue commercial, référencé
  par les lignes de commande).
- **Notification** : une notification groupée d'un autre destinataire qui cite le compte effacé
  est supprimée entière, plutôt que réécrite. Une annulation `account_deleted` prévient la partie
  restante sans nommer personne.

## Conséquences

- Nouveau motif d'annulation `account_deleted` dans `poro.order.order.cancelled` (ajout
  rétro-compatible au contrat).
- Un compte effacé avec beaucoup de contenu est traité dans **une** transaction par service.
  Volumes du MVP : quelques centaines de lignes au plus. Au-delà de 10 000 éléments, passer à un
  traitement par lots (suivi au backlog).
- Les compteurs de partages dans feed et search ne baissent pas : social ne publie pas
  d'événement de suppression de partage. L'écart est faible et sans donnée personnelle.
- Le registre de révocation (ADR-0016) coupe déjà les sessions à la demande de suppression ;
  l'effacement ne change rien aux sessions.

## Alternatives écartées

- **Supprimer les lignes en cascade partout** : casserait les fils de commentaires des autres,
  l'historique comptable et les tombstones qui empêchent une vidéo de réapparaître.
- **Un orchestrateur central qui appelle chaque service** : couplage synchrone, et l'outbox
  avec les inbox donne déjà la garantie « au moins une fois, effet unique ».
- **Publier un profil vide (`profile.updated`) au lieu de consommer l'événement** : ne couvre
  que les projections de profil, pas les vidéos, commandes, jetons ou signalements.
