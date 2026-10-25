package com.poro.app.core.network

import okhttp3.Interceptor
import okhttp3.Response

/**
 * Déclare ce que l'application sait afficher (ADR-0020) sur chaque appel à l'API, depuis un seul
 * endroit : aucun écran ne peut l'oublier. Sans cet en-tête, le serveur ne renvoie jamais de
 * publication photo (versions publiées avant les photos).
 */
class CapabilitiesInterceptor : Interceptor {
    override fun intercept(chain: Interceptor.Chain): Response =
        chain.proceed(chain.request().newBuilder().header(HEADER, VALUE).build())

    companion object {
        const val HEADER = "Poro-Capabilities"

        /** Publications photo et leur carrousel. */
        const val VALUE = "photo-posts"
    }
}
