# Intégrer l'application Android à l'API PORO (PORO-013, côté backend)

Guide pour l'équipe mobile, propriétaire de `apps/mobile-android`. Ce document décrit ce que le
backend attend et garantit. Il ne modifie rien dans le projet Android : ce dossier est hors du
périmètre de cette livraison.

Les contrats complets sont dans `docs/api/*.openapi.yaml` (un fichier par domaine). Ils sont la
référence : chaque route documentée est testée contre la gateway (`infra/gateway/tests/run.sh`).

## État du projet Android constaté

- Gabarit Android Studio vide : `com.example.myporo`, aucun code Kotlin applicatif, pas de
  permission `INTERNET` dans le manifeste.
- La CI assemble maintenant réellement l'APK debug (`mobile-build`, `gradlew assembleDebug`) au
  lieu d'un `echo`.

## Adresse de l'API

| Environnement | URL de base | Remarque |
|---|---|---|
| Émulateur, pile locale | `http://10.0.2.2:8000` | la gateway écoute sur `127.0.0.1:8000` du poste ; `10.0.2.2` est l'hôte vu de l'émulateur |
| Téléphone physique en dev | non pris en charge directement | la gateway n'écoute que sur `127.0.0.1`. Utiliser `adb reverse tcp:8000 tcp:8000`, puis `http://127.0.0.1:8000` |
| Staging / production | `https://…` (à venir, `docs/runbooks/deploy.md`) | HTTPS uniquement |

Le HTTP en clair n'est nécessaire qu'en dev. Le limiter aux hôtes `10.0.2.2` et `127.0.0.1` par
une *network security config* du build debug, côté Android : le backend n'impose rien là-dessus.

Toutes les routes publiques commencent par `/api/v1/<domaine>`. Rien d'autre n'est exposé par la
gateway (`/health` mis à part).

## Format des réponses

```json
{ "data": { … }, "error": null, "meta": { "request_id": "…" } }
{ "data": null, "error": { "code": "otp_invalid", "message": "…" }, "meta": { "request_id": "…" } }
```

- Brancher l'interface sur `error.code` (énumération stable du contrat), jamais sur `message`
  (texte libre, en anglais).
- Envoyer un en-tête `X-Request-ID` (UUID) par requête et le journaliser côté app : il relie un
  rapport de bug aux logs serveur (`docs/runbooks/observability.md`).
- `429 rate_limited` : attendre la durée de `Retry-After` (en secondes) quand l'en-tête est
  présent (gateway, auth, social, modération), sinon reculer de façon exponentielle. Jamais de
  boucle de réessai immédiate. Les connexions verrouillées après trop d'échecs répondent aussi
  avec `Retry-After` (ADR-0017).

## Authentification

1. **Connexion** : par téléphone (`POST /api/v1/auth/otp/request`, puis `/otp/verify`) ou par
   email (`/email/register`, `/email/login`). Envoyer `device_name` (60 caractères au plus,
   ex. « Pixel 8 de Awa ») : il apparaît dans la liste des sessions.
2. **Jetons** : l'access token (JWT, 15 min, `expires_in`) va dans `Authorization: Bearer`. Le
   refresh token est opaque et **change à chaque rafraîchissement** : le stocker chiffré
   (Android Keystore / EncryptedSharedPreferences), jamais en clair ni dans les logs.
3. **Rafraîchir** (`POST /api/v1/auth/refresh`) : **une seule requête de rafraîchissement à la
   fois** (mutex dans l'intercepteur OkHttp). Rejouer un refresh token déjà utilisé révoque toute
   la session de l'appareil. Seule exception : un réessai dans les 30 s dont la réponse a été
   perdue (ADR-0002).
4. **401** :
   - sur une route métier : rafraîchir une fois, puis rejouer la requête ;
   - `refresh_token_invalid` ou `session_revoked` au rafraîchissement : effacer les jetons et
     revenir à l'écran de connexion.

   Une session révoquée depuis un autre appareil, une réinitialisation de mot de passe ou une
   demande de suppression de compte est refusée **immédiatement par tous les services**
   (ADR-0016), pas seulement à l'expiration du jeton.
5. **Sessions** : `GET /api/v1/auth/sessions`, `DELETE /sessions/{id}`,
   `POST /sessions/revoke-others`.

## Suppression du compte (exigence des stores)

- `POST /api/v1/auth/me/delete` avec une preuve récente : mot de passe pour un compte email, code
  OTP frais pour un compte téléphone. La suppression est programmée à J+30 et tous les appareils
  sont déconnectés.
- Se reconnecter avant l'échéance l'annule : la réponse de connexion contient
  `deletion_cancelled: true`, à afficher. `POST /me/delete/cancel` l'annule aussi.
- À l'échéance, toutes les données personnelles sont effacées dans tous les services
  (ADR-0018) ; le pseudo redevient libre.

## Envoi de vidéos

Upload direct vers le stockage, sans passer par l'API :

1. `POST /api/v1/videos/uploads` (taille, type ; titre facultatif) → identifiant et URL
   présignées, une par partie de 8 Mio. Sans titre, le fil, la recherche et les notifications
   affichent la première ligne de la description.
2. `PUT` de chaque partie sur son URL. Garder l'`ETag` de chaque réponse. Réessayer une partie
   qui échoue, pas tout le fichier.
3. `POST /api/v1/videos/uploads/{id}/complete` avec la liste des parties.

   Envoi interrompu (coupure, processus arrêté, URL expirée au bout d'une heure) :
   `POST /api/v1/videos/uploads/{id}/resume` liste les parties que le stockage a déjà reçues,
   avec leur `ETag`, et signe de nouvelles URL pour les autres. On n'envoie que celles-là, puis
   `complete` avec toutes les parties. Un 409 veut dire que l'envoi n'est plus ouvert : recommencer.
4. La vidéo passe `processing`, puis `ready` ou `failed` (`too_long` au-delà de 3 min,
   `invalid_media`, `transcode_failed`). Interroger `GET /api/v1/videos/{id}` avec un intervalle
   croissant, ou attendre la notification.

Formats acceptés : MP4/MOV (H.264, HEVC, MPEG-4, AV1) et WebM (VP8, VP9, AV1) ; 4096 px au plus
par côté ; 256 Mio et 3 minutes au plus. Les autres fichiers sont refusés (ADR-0007). Lecture :
HLS (`master.m3u8`), à lire avec ExoPlayer/Media3.

Le profil public d'un compte liste ses publications prêtes avec
`GET /api/v1/videos/users/{userId}` (sans connexion ; les publications photo seulement avec
`Poro-Capabilities: photo-posts`).

Avatars et images de boutique suivent le même principe : URL présignée, upload, puis
confirmation (`docs/api/user.openapi.yaml`, `shop.openapi.yaml`).

## Notifications push

- Enregistrer le jeton FCM avec `PUT /api/v1/notifications/devices` après chaque connexion et à
  chaque `onNewToken` (10 requêtes par minute au plus).
- Le supprimer à la déconnexion. Le backend supprime de lui-même les jetons que FCM déclare
  invalides.
- Le projet Firebase et le fichier `google-services.json` sont à créer par l'équipe mobile. Le
  backend a besoin du compte de service correspondant, un secret à fournir par environnement.

## Ce qui bloque côté backend : rien

Toutes les routes nécessaires au MVP sont disponibles et testées. Les dépendances bloquantes sont
côté Android ou côté exploitation :

- code applicatif Android, permission `INTERNET`, network security config de dev : équipe mobile ;
- projet Firebase (FCM) : équipe mobile, puis secret côté backend ;
- URL HTTPS de staging : hébergement (`docs/runbooks/deploy.md`).
