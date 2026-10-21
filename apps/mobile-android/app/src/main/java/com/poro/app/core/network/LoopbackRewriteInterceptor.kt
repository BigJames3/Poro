package com.poro.app.core.network

import okhttp3.HttpUrl
import okhttp3.Interceptor
import okhttp3.Response

/**
 * Debug uniquement. En local, le backend signe des URL de médias sur `localhost:9000`
 * (`S3_PUBLIC_ENDPOINT`), injoignable depuis l'émulateur. On envoie la requête à [targetHost]
 * (l'hôte de l'API, `10.0.2.2` sur émulateur) en gardant l'en-tête `Host` d'origine : les
 * signatures S3 des URL présignées, qui couvrent cet en-tête, restent valides.
 */
class LoopbackRewriteInterceptor(private val targetHost: String) : Interceptor {
    override fun intercept(chain: Interceptor.Chain): Response {
        val request = chain.request()
        val url = request.url
        if (url.host !in LOOPBACK || url.host == targetHost) return chain.proceed(request)
        val originalHost = if (url.port == HttpUrl.defaultPort(url.scheme)) url.host else "${url.host}:${url.port}"
        val rewritten = request.newBuilder()
            .url(url.newBuilder().host(targetHost).build())
            .header("Host", originalHost)
            .build()
        return chain.proceed(rewritten)
    }

    private companion object {
        val LOOPBACK = setOf("localhost", "127.0.0.1")
    }
}
