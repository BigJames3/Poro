# ADR-0006 — Service user (NestJS) : profils, username, avatars, créateurs

- Statut : accepté
- Date : 2026-09-30
- Décideurs : ingénierie

## Contexte

Auth possède l'identité (téléphone, email, mot de passe, rôles, sessions).
Le produit a besoin d'un profil public (username, bio, avatar) et d'un
parcours « devenir créateur » qui ajoute le rôle CREATOR **sans** qu'un
second service n'écrive dans la table `user_roles`.

## Décision

1. **Service NestJS dédié**, port 8082, base `poro_user`. `user_id` = JWT `sub`
   (id auth). Auth n'expose pas le profil ; user ne stocke ni téléphone ni email.

2. **Création du profil**
   - Consommateur `poro-user-profiles` de `poro.auth.user.created` (inbox).
   - Repli : `GET /api/v1/users/me` upsert le profil si l'événement est en retard.

3. **Username** unique (y compris après soft-delete : le handle n'est pas
   réattribué), minuscule, 3–30 `[a-z0-9._]`, pas seulement des chiffres,
   liste de réservés (poro, admin, support, …). Contrainte SQL en défense.

4. **Avatar** : POST présigné S3 (`content-length-range` 1–5 MiB, Content-Type
   jpeg/png/webp). À la confirmation le service relit l'objet, le ré-encode
   en WebP 512×512 (Sharp) — ça rejette le HTML/SVG et enlève l'EXIF (GPS).
   L'URL publique n'est jamais l'objet d'upload brut.

5. **Créateur** : `POST /me/creator` exige un username, pose `is_creator` et
   écrit `poro.user.creator.activated` dans l'outbox **de la même transaction**.
   Auth (groupe `poro-auth-creator-roles`) ajoute CREATOR par
   `INSERT … ON CONFLICT DO NOTHING`. Idempotent. Le rôle arrive au prochain
   refresh, pas sur l'access token déjà émis.

6. **Profil public diffusé** (ajout du 2026-10-07) : chaque changement d'un
   champ public (`username`, `display_name`, avatar, `is_creator`) écrit
   `poro.user.profile.updated` v1 dans l'outbox **de la même transaction**,
   sous verrou de ligne. L'événement est un instantané complet (`user_id`,
   `username`, `display_name`, `avatar_url`, `is_creator`, `updated_at`) :
   les consommateurs (notification) gardent celui au `updated_at` le plus
   récent. Une bio, ou une requête qui ne change rien, n'émet rien. Un profil
   créé par `poro.auth.user.created` n'a encore aucun champ public : pas
   d'événement avant le premier changement.

7. **Paiement** : hors de ce service. Rien ici n'est source de vérité financière.

## Conséquences

- Deux déploiables de plus (user + relais `poro_user`).
- Un compte peut être PERSONAL+CREATOR sans enum exclusif (ADR-0001).
- La suppression de compte (P1) devra soft-delete le profil et garder le
  username bloqué.
