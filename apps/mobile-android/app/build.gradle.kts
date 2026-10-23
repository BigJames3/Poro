import java.util.Properties

plugins {
    // AGP 9 compile Kotlin lui-même (« built-in Kotlin ») : pas de plugin kotlin-android.
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.compose)
    alias(libs.plugins.kotlin.serialization)
}

/** local.properties (ignoré par Git) : réglages propres au poste, comme l'URL de debug. */
val localProperties = Properties().apply {
    rootProject.file("local.properties").takeIf { it.isFile }?.reader()?.use { load(it) }
}

/**
 * Valeur de build fournie par une propriété Gradle (`-Pporo.x=…`), local.properties ou une
 * variable d'environnement (CI). Jamais de secret dans le dépôt : sans valeur, le build de
 * release n'est pas signé.
 */
fun buildValue(property: String, env: String): String? =
    (findProperty(property) as String?)?.takeIf { it.isNotBlank() }
        ?: localProperties.getProperty(property)?.takeIf { it.isNotBlank() }
        ?: System.getenv(env)?.takeIf { it.isNotBlank() }

// Émulateur : 10.0.2.2 est le poste de développement, où la gateway écoute sur 127.0.0.1:8000.
// Téléphone physique, ou émulateur dont le réseau ne joint pas l'hôte : `adb reverse tcp:8000 tcp:8000`
// puis `poro.debugApiBaseUrl=http://127.0.0.1:8000/` dans local.properties.
val debugApiUrl = (buildValue("poro.debugApiBaseUrl", "PORO_DEBUG_API_BASE_URL") ?: "http://10.0.2.2:8000/").also {
    require(it.endsWith("/")) { "poro.debugApiBaseUrl doit se terminer par /" }
}

val releaseApiUrl = (buildValue("poro.apiBaseUrl", "PORO_API_BASE_URL") ?: "https://api.poro.invalid/").also {
    // Le build de release refuse le trafic en clair (network_security_config) : HTTPS obligatoire.
    require(it.startsWith("https://") && it.endsWith("/")) { "poro.apiBaseUrl doit être une URL https:// terminée par /" }
}
val shareBaseUrl = buildValue("poro.shareBaseUrl", "PORO_SHARE_BASE_URL") ?: ""
val releaseKeystore = buildValue("poro.keystore", "PORO_KEYSTORE_FILE")

android {
    // Identifiant définitif sur le Play Store : ne plus le changer après la première publication.
    namespace = "com.poro.app"
    compileSdk {
        version = release(37)
    }

    defaultConfig {
        applicationId = "com.poro.app"
        minSdk = 24
        targetSdk = 37
        // La CI fournit un numéro croissant (numéro de run) ; 1 en local.
        versionCode = buildValue("poro.versionCode", "PORO_VERSION_CODE")?.toInt() ?: 1
        versionName = buildValue("poro.versionName", "PORO_VERSION_NAME") ?: "1.0"

        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"

        // Base des liens de partage (ex. https://poro.africa) : vide tant que le site et les liens
        // d'application n'existent pas ; le partage n'envoie alors que le titre.
        buildConfigField("String", "SHARE_BASE_URL", "\"$shareBaseUrl\"")
    }

    signingConfigs {
        // Keystore de publication hors du dépôt : chemin et mots de passe viennent des secrets CI.
        if (releaseKeystore != null) {
            create("release") {
                storeFile = file(releaseKeystore)
                storePassword = buildValue("poro.keystorePassword", "PORO_KEYSTORE_PASSWORD")
                keyAlias = buildValue("poro.keyAlias", "PORO_KEY_ALIAS")
                keyPassword = buildValue("poro.keyPassword", "PORO_KEY_PASSWORD")
            }
        }
    }

    buildTypes {
        debug {
            // Voir debugApiUrl en tête de fichier ; le HTTP n'est autorisé que vers 10.0.2.2 et 127.0.0.1.
            buildConfigField("String", "API_BASE_URL", "\"$debugApiUrl\"")
            applicationIdSuffix = ".debug"
        }
        release {
            // URL HTTPS de staging/production (docs/runbooks/deploy.md), fournie au build de publication.
            buildConfigField("String", "API_BASE_URL", "\"$releaseApiUrl\"")
            // R8 : réduction, obscurcissement et réduction des ressources. Règles propres à l'app
            // dans src/main/keepRules (Retrofit et kotlinx.serialization fournissent les leurs).
            optimization {
                enable = true
            }
            signingConfigs.findByName("release")?.let { signingConfig = it }
        }
    }

    buildFeatures {
        compose = true
        buildConfig = true
    }

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_17
        targetCompatibility = JavaVersion.VERSION_17
    }

    testOptions {
        // Les tests JVM ne touchent pas au framework Android ; android.util.Log renvoie des valeurs par défaut.
        unitTests.isReturnDefaultValues = true
    }
}

dependencies {
    implementation(libs.androidx.core.ktx)
    implementation(libs.androidx.activity.compose)
    implementation(libs.androidx.lifecycle.runtime.compose)
    implementation(libs.androidx.lifecycle.viewmodel.compose)
    implementation(libs.androidx.lifecycle.viewmodel.savedstate)
    implementation(libs.androidx.navigation.compose)
    implementation(libs.androidx.datastore.preferences)
    implementation(libs.androidx.work.runtime)

    implementation(platform(libs.androidx.compose.bom))
    implementation(libs.androidx.compose.ui)
    implementation(libs.androidx.compose.ui.tooling.preview)
    implementation(libs.androidx.compose.material3)
    implementation(libs.androidx.compose.material.icons.core)
    debugImplementation(libs.androidx.compose.ui.tooling)

    implementation(libs.kotlinx.coroutines.android)
    implementation(libs.kotlinx.serialization.json)
    implementation(libs.retrofit)
    implementation(libs.retrofit.kotlinx.serialization)
    implementation(libs.okhttp)
    implementation(libs.okhttp.logging)
    implementation(libs.media3.exoplayer)
    implementation(libs.media3.exoplayer.hls)
    implementation(libs.media3.ui)
    implementation(libs.media3.datasource.okhttp)
    implementation(libs.coil.compose)
    implementation(libs.coil.network.okhttp)

    testImplementation(libs.junit)
    testImplementation(libs.kotlinx.coroutines.test)
    testImplementation(libs.okhttp.mockwebserver)
}
