# ADR-0002 — Sessions par appareil et rotation des refresh tokens

- Statut : accepté
- Date : 2026-09-29

## Contexte

La rotation créait le nouveau token puis révoquait l'ancien, sans transaction : deux requêtes
concurrentes obtenaient chacune une session valide, et un token volé restait exploitable sans
détection. Les réseaux mobiles africains perdent fréquemment des réponses : une détection de
réutilisation stricte déconnecterait des utilisateurs légitimes.

## Décision

1. **Famille de session** : chaque connexion crée une famille (`refresh_tokens.family_id`),
   identifiée dans l'access token par le claim `sid`. Toutes les rotations restent dans la famille.
2. **Rotation atomique** : dans une transaction, `SELECT … FOR UPDATE` sur le token présenté,
   création du successeur, puis `revoked_at` + `replaced_by` sur l'ancien.
3. **Détection de réutilisation** : un token déjà révoqué présenté alors que la famille contient
   encore un token actif révoque toute la famille et renvoie `401 session_revoked`.
4. **Fenêtre de grâce** (`JWT_REFRESH_REUSE_GRACE`, 30 s par défaut) : si le token a été
   remplacé il y a moins de 30 s et que son successeur n'a jamais servi, on considère que la
   réponse a été perdue ; le successeur est révoqué et un nouveau est émis. Si ce successeur
   « perdu » est utilisé plus tard, cela prouve deux détenteurs : la famille est révoquée.
5. **Logout** : blacklist du `jti` de l'access token dans Redis (TTL = durée restante) et
   révocation de la famille `sid` uniquement ; les autres appareils restent connectés.

## Conséquences pour les clients mobiles

- Le client doit **sérialiser** ses refresh (un seul en vol, via un mutex dans l'`Authenticator`
  OkHttp côté Android). Deux refresh parallèles avec le même token peuvent déclencher une
  révocation.
- Sur `session_revoked` ou `refresh_token_invalid`, le client efface ses tokens et renvoie vers
  la connexion.

## Alternatives écartées

- Rotation sans détection : ne protège pas contre le vol de token.
- Détection stricte sans grâce : déconnexions fréquentes sur réseau instable.
- Stocker le token brut chiffré pour le renvoyer à l'identique : augmente la surface d'attaque.
