# Poro Android

Application Android de Poro (Kotlin, Jetpack Compose), module unique `:app`.

## Développement

- Android Studio récent (AGP 9.4), SDK Platform 37 ; le JDK est téléchargé par Gradle.
- Backend local : `docker compose up` à la racine du monorepo, gateway sur `127.0.0.1:8000`.
- Émulateur : l'application de debug appelle `http://10.0.2.2:8000/`. Téléphone physique :
  `adb reverse tcp:8000 tcp:8000`.

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

Sans keystore, le build de release n'est pas signé. R8 est activé (règles propres à
l'application dans `app/src/main/keepRules`) ; conserver `app/build/outputs/mapping/release/mapping.txt`
pour lire les traces de plantage.

## Limites connues

- Pas encore de notifications push (aucun projet Firebase) ni de paiement en ligne (Wave, PORO-015).
- Un envoi de vidéo ne survit pas à l'arrêt du processus (pas de WorkManager).
- Le catalogue n'a pas de recherche (PORO-026) : une boutique s'ouvre par son identifiant.
