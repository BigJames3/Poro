# ADR-0010 — Vague 2 : modération v1 par règles, chat reporté

- Statut : accepté
- Date : 2026-10-08

## Contexte

La Vague 2 prévoyait moderation (Python), search (Go, OpenSearch) et chat
(Go, WebSocket). Les règles du projet (`.cursor/rules/project.md`) excluent le
chat du MVP et limitent la modération P0 à « signalement + filtre auto basique ».
La modération ML (PyTorch, Hugging Face) donnerait une image de plus de 2 Go et
une inférence CPU lente sur les postes de développement, sans GPU.

## Décision

1. **Ordre** : moderation, puis search (OpenSearch, confirmé au moment de son
   plan). **Chat est reporté** après le MVP.
2. **Modération v1 par règles**, déterministe et explicable : listes de mots
   configurables (normalisation des accents et du leetspeak), liens et numéros
   répétés, majuscules et répétitions excessives ; signalements des
   utilisateurs ; file humaine pour les rôles MODERATOR et ADMIN. Pas de modèle
   ML en v1.
3. **Contrats** posés avant le service :
   - `poro.moderation.content.removed` (vidéo ou commentaire, motif, auteur de
     la décision `auto` ou `moderator`) ;
   - `poro.moderation.content.restored` (vidéo seulement) ;
   - `poro.social.comment.created` porte le texte complet (`text`, additif) et
     `poro.social.comment.updated` publie le texte après une édition, pour que
     la modération ne lise pas qu'un extrait de 140 caractères ni ne manque une
     édition.
4. **Application par les consommateurs**, sans appel synchrone :
   - feed : `moderation_status = 'rejected'` ou `'approved'` ;
   - social : vidéo `removed` (plus aucune action), commentaire supprimé par
     le même chemin que son auteur, qui publie `poro.social.comment.deleted` ;
   - notification : suppression des notifications du contenu retiré.
   Un retrait arrivé avant `poro.video.ready` laisse une trace qui garde la
   vidéo masquée.

## Conséquences

- Un commentaire retiré ne peut pas être restauré en v1 (il faudrait le
  republier sans renotifier).
- Video consomme `content.removed` et `content.restored` : une vidéo retirée
  répond 404 sauf à son propriétaire, et ses médias passent dans le bucket
  privé `poro-quarantine` (de même pour une vidéo supprimée). Le cache CDN
  n'est pas purgé automatiquement.
- Un modèle ML pourra s'ajouter derrière le même contrat, sans changer les
  consommateurs.
