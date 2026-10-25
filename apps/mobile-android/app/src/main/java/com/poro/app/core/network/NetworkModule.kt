package com.poro.app.core.network

import com.poro.app.core.security.TokenStore
import com.poro.app.data.auth.AuthApi
import com.poro.app.data.auth.RefreshApi
import java.util.concurrent.TimeUnit
import okhttp3.HttpUrl
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.logging.HttpLoggingInterceptor
import retrofit2.Retrofit
import retrofit2.converter.kotlinx.serialization.asConverterFactory

/**
 * Clients HTTP de l'application.
 *
 * - [baseClient] : X-Request-ID et délais, sans jeton ; sert au rafraîchissement. Il n'a pas
 *   d'authenticator, donc pas de récursion pendant un rafraîchissement.
 * - [apiClient] : ajoute le Bearer et rafraîchit la session sur un 401.
 * - [mediaClient] : stockage objet (lecture HLS, images, envoi sur URL présignée). Jamais de jeton :
 *   ces URL sont publiques ou signées, et un Bearer y fuiterait vers un tiers (CDN, S3).
 */
class NetworkModule(
    private val baseUrl: HttpUrl,
    tokens: TokenStore,
    debug: Boolean,
    onSessionExpired: () -> Unit,
) {
    val baseClient: OkHttpClient = OkHttpClient.Builder()
        .connectTimeout(15, TimeUnit.SECONDS)
        .readTimeout(30, TimeUnit.SECONDS)
        .writeTimeout(30, TimeUnit.SECONDS)
        .addInterceptor(RequestIdInterceptor())
        .apply {
            if (debug) {
                // BASIC : méthode, chemin, statut, durée. Jamais les en-têtes ni les corps (jetons, codes OTP).
                addInterceptor(HttpLoggingInterceptor().setLevel(HttpLoggingInterceptor.Level.BASIC))
            }
        }
        .build()

    private val converter = PoroJson.asConverterFactory("application/json".toMediaType())

    private fun buildRetrofit(baseUrl: HttpUrl, client: OkHttpClient): Retrofit =
        Retrofit.Builder().baseUrl(baseUrl).client(client).addConverterFactory(converter).build()

    private val refreshApi: RefreshApi by lazy { buildRetrofit(baseUrl, baseClient).create(RefreshApi::class.java) }

    val apiClient: OkHttpClient = baseClient.newBuilder()
        .addInterceptor(CapabilitiesInterceptor())
        .addInterceptor(AuthInterceptor(tokens))
        .authenticator(TokenAuthenticator(tokens, { refreshApi }, onSessionExpired))
        .build()

    /** Délais longs : une partie de vidéo fait 8 Mio, sur des réseaux mobiles lents. */
    val mediaClient: OkHttpClient = OkHttpClient.Builder()
        .connectTimeout(15, TimeUnit.SECONDS)
        .readTimeout(60, TimeUnit.SECONDS)
        .writeTimeout(120, TimeUnit.SECONDS)
        .apply {
            if (debug) {
                addInterceptor(LoopbackRewriteInterceptor(baseUrl.host))
                addInterceptor(MediaLoggingInterceptor())
            }
        }
        .build()

    val retrofit: Retrofit = buildRetrofit(baseUrl, apiClient)

    /** Interface Retrofit d'un service, servie par la gateway sous la même URL de base. */
    inline fun <reified T : Any> api(): T = retrofit.create(T::class.java)

    val authApi: AuthApi = api()
}
