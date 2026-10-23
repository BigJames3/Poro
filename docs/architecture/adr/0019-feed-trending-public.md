# ADR-0019 — Trending public (B-FEED)

- Statut : **accepté** (décision produit du 21 octobre 2026) ; **implémenté**
  le 21 octobre 2026 dans `services/feed` et `docs/api/feed.openapi.yaml`
- Date : 2026-10-21
- Complète : [ADR-0009](0009-feed-v1.md)

## Contexte

Le mode visiteur validé (`docs/product/guest-mode/`) promet : ouvrir Poro =
fil public, compte seulement à l’action. Les trois `GET /api/v1/feed/*`
exigent aujourd’hui un JWT. Le handler `Trending` n’utilise déjà pas le
`sub` ; seul le middleware `jwtauth.Middleware` bloque l’anonyme (401
constaté sur la gateway locale le 21 octobre 2026).

For You mélange follows et session Redis ; Following lit les abonnements.
Les rendre publics serait incorrect. Search est déjà `OptionalMiddleware` +
60 requêtes / min / IP.

Sans cette décision, le lot Android G1 livrerait un Accueil en 401.
Un fil bricolé depuis `GET /videos/{id}` + search est interdit
(revue sécu guest-mode).

## Décision

1. **Accepter B-FEED.** `GET /api/v1/feed/trending` devient optionnellement
   authentifié : absence de header = 200 (page tendances, éventuellement
   vide) ; Bearer invalide ou mal formé = **401** (ne pas traiter un jeton
   pourri comme anonyme). Même `OptionalMiddleware` que search / vidéo.
2. **For You et Following restent Auth.** Middleware inchangé.
3. **Quota anonyme par IP : 60 / min** sur trending (aligné search), en plus
   du plafond membre existant (120 / min / `sub`) et du plafond gateway.
4. **Pas de JWT `GUEST`.** Visiteur = aucun header `Authorization`.
5. **Pas de PII ajoutée** dans les items tendances (contrat feed actuel).
6. **OpenAPI** : `security: [{}, {bearerAuth}]` sur trending seulement ;
   description « All feed routes need a token » à corriger.
7. **Tests exigés avant de déclarer B-FEED vert** : 200 sans header ;
   For You / Following 401 sans header ; Bearer pourri 401 ; 429 après
   quota IP.
8. **Android guest (G1) et posts image (B-IMG-POST) restent interdits**
   tant qu’une autorisation distincte n’est pas donnée **et**, pour G1,
   que les tests du point 7 sont verts. L’implémentation backend du 21
   octobre 2026 ne lève pas le mur Android.

La gateway ne change pas : elle ne vérifie pas les JWT.

## Conséquences

- Surface publique nouvelle : le catalogue des clips `ready`+`approved` des
  72 h devient lisible sans compte. Accepté : l’algo d’engagement 72 h
  était déjà visible à tout membre (revue sécu §9).
- Implémentation : `services/feed` (routes séparées ou Opt sur trending
  seul), limiteur IP, ADR déjà écrite ici, OpenAPI, tests service ± e2e.
- Si l’implémentation est refusée plus tard, l’app garde l’auth-wall ; le
  dossier guest-mode reste la spec, G1 annulé. Ce n’est plus le choix par
  défaut : le choix par défaut est **ouvrir trending**.
