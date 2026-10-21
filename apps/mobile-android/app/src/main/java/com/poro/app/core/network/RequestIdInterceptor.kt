package com.poro.app.core.network

import android.util.Log
import java.util.UUID
import okhttp3.Interceptor
import okhttp3.Response

/**
 * Ajoute un `X-Request-ID` (UUID) à chaque requête et le journalise : il relie un rapport de bug
 * aux logs du serveur (docs/runbooks/observability.md). Un identifiant déjà présent est conservé,
 * ce qui garde le même identifiant quand l'authenticator rejoue une requête.
 */
class RequestIdInterceptor : Interceptor {
    override fun intercept(chain: Interceptor.Chain): Response {
        val request = chain.request()
        val tagged = if (request.header(HEADER) != null) {
            request
        } else {
            request.newBuilder().header(HEADER, UUID.randomUUID().toString()).build()
        }
        val response = chain.proceed(tagged)
        if (!response.isSuccessful) {
            // Jamais d'URL complète ni de corps : ils peuvent contenir des données personnelles.
            Log.w(TAG, "${tagged.method} ${tagged.url.encodedPath} -> ${response.code} request_id=${tagged.header(HEADER)}")
        }
        return response
    }

    companion object {
        const val HEADER = "X-Request-ID"
        private const val TAG = "PoroApi"
    }
}
