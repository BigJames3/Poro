package com.poro.app.core.network

import com.poro.app.core.security.TokenStore
import okhttp3.Interceptor
import okhttp3.Response

/** Ajoute `Authorization: Bearer <access token>` quand une session existe. */
class AuthInterceptor(private val tokens: TokenStore) : Interceptor {
    override fun intercept(chain: Interceptor.Chain): Response {
        val request = chain.request()
        val access = tokens.current()?.accessToken
        if (access == null || request.header(AUTHORIZATION) != null) {
            return chain.proceed(request)
        }
        return chain.proceed(request.newBuilder().header(AUTHORIZATION, "Bearer $access").build())
    }

    companion object {
        const val AUTHORIZATION = "Authorization"
    }
}
