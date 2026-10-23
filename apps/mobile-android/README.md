# Poro Android

Application Android de Poro (Kotlin, Jetpack Compose), module unique `:app`.

## Développement

- Android Studio récent (AGP 9.4), SDK Platform 37 ; le JDK est téléchargé par Gradle.
- Backend local : `docker compose up` à la racine du monorepo, gateway sur `127.0.0.1:8000`.
- Émulateur : l'application de debug appelle `http://10.0.2.2:8000/`.
- Téléphone physique, ou émulateur qui ne joint pas l'hôte (`SocketTimeoutException` vers
  `10.0.2.2` dans Logcat) : passer par le tunnel adb, à relancer après chaque redémarrage de
  l'appareil, puis rebuild.

  ```bash
  adb reverse tcp:8000 tcp:8000   # API (gateway)
  adb reverse tcp:9000 tcp:9000   # médias (SeaweedFS : vidéos, images, envois)
  ```

  et dans `local.properties` (non versionné) : `poro.debugApiBaseUrl=http://127.0.0.1:8000/`.

```bash
./gradlew testDebugUnitTest assembleDebug
```

## Build de publication

Aucun secret n'est versionné. Le build de release lit ses valeurs dans des propriétés Gradle
(`-Pporo.…`) ou des variables d'environnement :

| Propriété Gradle | Variable d'environnement | Rôle |
| --- | --- | --- |
| `poro.apiBaseUrl` | `PORO_API_BASE_URL` | URL HTTPS de l'API, terminée par `/` |
| `poro.shareBaseUrl` | `PORO_SHARE_BASE_URL` | Base des liens de partage (vide : titre seul) |
| `poro.versionCode` | `PORO_VERSION_CODE` | Numéro de version croissant (1 par défaut) |
| `poro.versionName` | `PORO_VERSION_NAME` | Version affichée (`1.0` par défaut) |
| `poro.keystore` | `PORO_KEYSTORE_FILE` | Chemin du keystore de publication |
| `poro.keystorePassword` | `PORO_KEYSTORE_PASSWORD` | Mot de passe du keystore |
| `poro.keyAlias` | `PORO_KEY_ALIAS` | Alias de la clé |
| `poro.keyPassword` | `PORO_KEY_PASSWORD` | Mot de passe de la clé |

```bash
PORO_API_BASE_URL=https://api.example.com/ \
PORO_KEYSTORE_FILE=/chemin/poro-release.jks PORO_KEYSTORE_PASSWORD=… \
PORO_KEY_ALIAS=poro PORO_KEY_PASSWORD=… \
./gradlew bundleRelease
```

Sans `PORO_API_BASE_URL`, les tâches `assembleRelease`, `bundleRelease` et `packageRelease` échouent volontairement : aucune release ne peut partir vers `api.poro.invalid`. Sans keystore, le build de release n'est pas signé ; avec un keystore, il manque un mot de passe ou l'alias, ou le fichier est introuvable, ces tâches échouent en nommant la valeur manquante. R8 est activé (règles propres à
l'application dans `app/src/main/keepRules`) ; conserver `app/build/outputs/mapping/release/mapping.txt`
pour lire les traces de plantage.

### Release signée par la CI

Le workflow `.github/workflows/android-release.yml` construit l'AAB (Play Store) et l'APK signés,
vérifie leur signature (`apksigner`, `jarsigner`) et les garde 90 jours comme artefacts du run, avec
`mapping.txt`. Il part d'un tag `android-v<version>` (ex. `android-v1.0.0`, qui devient le
`versionName`) ou d'un lancement manuel ; le `versionCode` est le numéro du run.

Valeurs à créer dans les réglages du dépôt (Settings → Secrets and variables → Actions) :

| Type | Nom | Contenu |
| --- | --- | --- |
| Variable | `PORO_API_BASE_URL` | URL HTTPS de l'API, terminée par `/` |
| Variable | `PORO_SHARE_BASE_URL` | Facultative : base des liens de partage |
| Secret | `PORO_KEYSTORE_BASE64` | Keystore encodé : `base64 -w0 poro-release.jks` |
| Secret | `PORO_KEYSTORE_PASSWORD` | Mot de passe du keystore |
| Secret | `PORO_KEY_ALIAS` | Alias de la clé |
| Secret | `PORO_KEY_PASSWORD` | Mot de passe de la clé |

S'il en manque une, le job ne construit rien, réussit, et liste les valeurs manquantes dans son
résumé (« Release Android : BLOQUÉ »). Une pull request qui modifie le workflow ne lance que cette
vérification : le keystore n'est utilisé que sur un tag ou un lancement manuel. Le keystore et ses
mots de passe ne doivent jamais être perdus : sans eux, plus aucune mise à jour sur le Play Store
(sauf avec la signature d'application Play activée).

## Limites connues

- Pas encore de notifications push (aucun projet Firebase) ni de paiement en ligne (Wave, PORO-015).
