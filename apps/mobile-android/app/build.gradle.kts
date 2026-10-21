plugins {
    // AGP 9 compile Kotlin lui-même (« built-in Kotlin ») : pas de plugin kotlin-android.
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.compose)
    alias(libs.plugins.kotlin.serialization)
}

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
        versionCode = 1
        versionName = "1.0"

        testInstrumentationRunner = "androidx.test.runner.AndroidJUnitRunner"
    }

    buildTypes {
        debug {
            // Émulateur : 10.0.2.2 est le poste de développement, où la gateway écoute sur 127.0.0.1:8000.
            // Téléphone physique : `adb reverse tcp:8000 tcp:8000` puis http://127.0.0.1:8000.
            buildConfigField("String", "API_BASE_URL", "\"http://10.0.2.2:8000/\"")
            applicationIdSuffix = ".debug"
        }
        release {
            // URL HTTPS de staging/production à renseigner quand l'hébergement existera (docs/runbooks/deploy.md).
            buildConfigField("String", "API_BASE_URL", "\"https://api.poro.invalid/\"")
            optimization {
                enable = false
            }
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
    implementation(libs.coil.compose)
    implementation(libs.coil.network.okhttp)

    testImplementation(libs.junit)
    testImplementation(libs.kotlinx.coroutines.test)
    testImplementation(libs.okhttp.mockwebserver)
}
