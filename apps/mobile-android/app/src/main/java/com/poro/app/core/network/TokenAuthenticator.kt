package com.poro.app.core.network

import com.poro.app.core.security.SessionTokens
import com.poro.app.core.security.TokenStore
import com.poro.app.data.auth.RefreshApi
import com.poro.app.data.auth.RefreshRequest
import java.io.IOException
import okhttp3.Authenticator
import okhttp3.Request
import okhttp3.Response
import okhttp3.Route

/**
 * Sur un 401, rafraîchit la session une seule fois puis rejoue la requête (ADR-0002).
 *
 * - Un seul rafraîchissement à la fois : le refresh token change à chaque rotation, et renvoyer
 *   un jeton déjà utilisé révoque toute la session de l'appareil. Les requêtes qui reçoivent un
 *   401 pendant ce temps attendent, puis repartent avec le nouveau jeton.
 * - `refresh_token_invalid` ou `session_revoked` : la session est terminée ; les jetons sont
 *   effacés et [onSessionExpired] ramène l'utilisateur à l'écran de connexion.
 * - Erreur réseau pendant le rafraîchissement : les jetons sont conservés (la session est
 *   peut-être encore valide) et la requête échoue avec son 401.
 */
class TokenAuthenticator(
    private val tokens: TokenStore,
    private val refreshApi: () -> RefreshApi,
    private val onSessionExpired: () -> Unit = {},
) : Authenticator {

    private val lock = Any()

    override fun authenticate(route: Route?, response: Response): Request? {
        // Une requête déjà rejouée qui reçoit encore 401 : on n'insiste pas.
        if (response.priorResponse != null) return null

        val failedToken = response.request.header(AuthInterceptor.AUTHORIZATION)
            ?.removePrefix("Bearer ")
            ?: return null // requête anonyme : rien à rafraîchir

        synchronized(lock) {
            val current = tokens.current() ?: return null

            // Un autre appel a déjà rafraîchi pendant que celui-ci attendait le verrou.
            if (current.accessToken != failedToken) return response.request.withToken(current.accessToken)

            val refreshed = refresh(current) ?: return null
            return response.request.withToken(refreshed.accessToken)
        }
    }

    private fun refresh(current: SessionTokens): SessionTokens? {
        val result = try {
            refreshApi().refresh(RefreshRequest(current.refreshToken)).execute()
        } catch (e: IOException) {
            return null
        } catch (e: RuntimeException) {
            // Réponse illisible (sérialisation) : on garde la session, l'appel échoue proprement en 401.
            return null
        }
        if (result.isSuccessful) {
            val data = result.body()?.data ?: return null
            return SessionTokens(data.accessToken, data.refreshToken).also(tokens::save)
        }
        val code = (result.toApiResult() as? ApiResult.Failure)?.code
        if ((code != null && code in SESSION_ENDED) || result.code() == 401) {
            tokens.clear()
            onSessionExpired()
        }
        return null
    }

    private fun Request.withToken(access: String): Request =
        newBuilder().header(AuthInterceptor.AUTHORIZATION, "Bearer $access").build()

    private companion object {
        val SESSION_ENDED = setOf(ApiErrorCodes.REFRESH_TOKEN_INVALID, ApiErrorCodes.SESSION_REVOKED, ApiErrorCodes.ACCOUNT_BLOCKED)
    }
}
