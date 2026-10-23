# ADR-0019 — Masquer un compte dès la demande de suppression

- Statut : accepté
- Date : 2026-10-21
- Complète : [ADR-0013](0013-cycle-de-vie-du-compte.md), [ADR-0018](0018-effacement-du-compte-dans-les-services.md)

## Contexte

Une demande de suppression efface le compte 30 jours plus tard (ADR-0013). Pendant ces 30 jours,
seul auth savait que la suppression était demandée : le profil, les vidéos, les commentaires et
les abonnements restaient visibles de tous. Une personne qui supprime son compte s'attend à
disparaître tout de suite.

## Décision

1. **Deux événements auth**, écrits dans l'outbox dans la transaction qui change
   `deletion_scheduled_at` :
   - `poro.auth.user.deletion_scheduled` v1 (`user_id`, `erase_at`, `changed_at`) à une nouvelle
     demande ; une demande répétée ne publie rien ;
   - `poro.auth.user.deletion_cancelled` v1 (`user_id`, `changed_at`) quand la suppression est
     annulée, par `POST /me/delete/cancel` ou par une nouvelle connexion.
2. **Masquer, ne rien effacer.** Les données restent intactes : une annulation remet tout en
   ligne. L'effacement reste celui d'ADR-0018, à l'échéance.
3. **Une table `hidden_accounts`** (`user_id`, `hidden`, `changed_at`) par service concerné, tenue
   par un groupe `poro-<service>-account-hiding`. Les deux événements passent par des topics
   différents, donc sans ordre garanti : le `changed_at` le plus récent gagne. Un rejeu ne
   change rien, sans inbox. Code commun : `packages/shared-go/hiding`.
4. **Ce qui disparaît** des lectures publiques et des nouvelles actions :

| Service | Masqué |
|---|---|
| user | profil : `GET /users/by-username/…` → 404, absent de `GET /users?ids=…` |
| video | `GET /videos/{id}` → 404 pour les autres comptes |
| feed | vidéos du compte dans tous les flux (le cache de première page vit 60 s) |
| search | compte et vidéos dans les résultats et les suggestions |
| social | vidéos (likes, commentaires, partages, stats → 404), compte (follow, listes, stats → 404) ; ses commentaires, likes et abonnements sortent des listes des autres |

   Les retraits restent possibles (retirer un like, se désabonner, supprimer son commentaire),
   pour que personne ne reste bloqué avec un compte masqué.

## Conséquences

- Les compteurs (likes, abonnés, commentaires) ne baissent pas pendant le masquage : ils
  reviendraient à l'annulation, et l'écart disparaît à l'effacement.
- Les médias déjà connus (HLS, miniatures) restent servis par le stockage et le CDN jusqu'à
  l'effacement, qui les met en quarantaine (ADR-0018). Un lien déjà partagé continue donc de
  lire la vidéo pendant 30 jours.
- Notification, shop, order et moderation ne masquent rien : leurs données ne sont pas publiques,
  ou relèvent d'une obligation (commandes en cours, dossiers de modération).
- Une boutique reste ouverte pendant les 30 jours ; elle est fermée à l'effacement (ADR-0018).
