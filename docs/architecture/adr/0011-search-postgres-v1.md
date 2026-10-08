# ADR-0011 — Search v1 : recherche plein texte Postgres derrière une interface

- Statut : accepté
- Date : 2026-10-08

## Contexte

La mission prévoyait OpenSearch pour le service `search`. OpenSearch ajoute au
moins 1 Go de RAM sur les postes de développement, un conteneur et une
sécurité à configurer en production. Au lancement, les volumes (vidéos,
comptes, hashtags) restent faibles.

## Décision

1. **Postgres 16 en v1**, base `poro_search` :
   - recherche plein texte avec une configuration `search_fr` (racines
     françaises, accents retirés par `unaccent`) ;
   - `pg_trgm` pour les fautes de frappe (`word_similarity`) et la similarité
     des hashtags ; préfixes indexés pour l'autocomplétion ;
   - classement : pertinence × engagement (`ln(1 + likes + 2·commentaires +
     3·partages)`) ÷ ancienneté en mois ; pour les comptes, username exact en
     tête, puis pertinence × abonnés, bonus créateur.
2. **Interface `index.Index`** : l'API et le service ne connaissent que cette
   interface ; la projection (consommateur `poro-search-indexer`) est propre à
   Postgres.
3. **Seuil de bascule vers OpenSearch** : p95 de `GET /api/v1/search` au-delà de
   **200 ms**, ou plus de **5 millions** de vidéos indexées, ou besoin de
   fonctions absentes de Postgres (synonymes par langue, recherche multilingue
   pondérée). La bascule ajoute un indexeur OpenSearch alimenté par les mêmes
   événements et une implémentation de `index.Index`.
4. **Recherche ouverte sans compte** : le jeton est facultatif et ne sert qu'à
   la limite de débit (60 requêtes par minute par compte, sinon par IP).

## Conséquences

- Aucune nouvelle infrastructure ; tout se teste de bout en bout avec
  testcontainers.
- La pagination se fait par décalage (500 résultats au plus) : un classement
  qui change entre deux pages peut répéter ou sauter un résultat.
- Les commentaires ne sont pas cherchables en v1.
