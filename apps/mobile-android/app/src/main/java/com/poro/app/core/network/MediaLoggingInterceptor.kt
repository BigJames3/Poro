package com.poro.app.core.network

import android.util.Log
import java.io.IOException
import okhttp3.Interceptor
import okhttp3.Response

/**
 * Debug uniquement : trace les échanges avec le stockage (envois, médias) sans la requête
 * (`?X-Amz-Signature=…` : une URL présignée vaut une autorisation) ni les en-têtes.
 */
class MediaLoggingInterceptor : Interceptor {
    override fun intercept(chain: Interceptor.Chain): Response {
        val request = chain.request()
        val target = "${request.method} ${request.url.scheme}://${request.url.host}:${request.url.port}${request.url.encodedPath}"
        return try {
            chain.proceed(request).also { Log.i(TAG, "$target -> ${it.code}") }
        } catch (e: IOException) {
            Log.w(TAG, "$target -> ${e.javaClass.simpleName}: ${e.message}")
            throw e
        }
    }

    private companion object {
        const val TAG = "PoroMedia"
    }
}
