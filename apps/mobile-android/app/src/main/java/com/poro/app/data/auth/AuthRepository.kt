package com.poro.app.data.auth

import com.poro.app.core.device.DeviceName
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.apiCall
import com.poro.app.core.security.SessionTokens
import com.poro.app.core.security.TokenStorageException
import com.poro.app.core.security.TokenStore
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.withContext

/** Connexion par téléphone et déconnexion de l'appareil. */
interface AuthRepository {
    /** Envoie un code à 6 chiffres par SMS au numéro [phone] (E.164). */
    suspend fun requestOtp(phone: String): ApiResult<CodeSent>

    /** Vérifie le code et, en cas de succès, enregistre la session de l'appareil. */
    suspend fun verifyOtp(phone: String, code: String): ApiResult<SignedIn>

    /** Termine la session côté serveur si possible ; les jetons locaux sont effacés dans tous les cas. */
    suspend fun logout()
}

class DefaultAuthRepository(
    private val api: AuthApi,
    private val tokens: TokenStore,
    private val deviceName: () -> String = DeviceName::current,
    private val io: CoroutineDispatcher = Dispatchers.IO,
) : AuthRepository {

    override suspend fun requestOtp(phone: String): ApiResult<CodeSent> =
        apiCall { api.requestOtp(OtpRequest(phone)) }

    override suspend fun verifyOtp(phone: String, code: String): ApiResult<SignedIn> {
        val result = apiCall { api.verifyOtp(OtpVerify(phone, code, deviceName())) }
        if (result !is ApiResult.Success) return result
        return try {
            withContext(io) { tokens.save(SessionTokens(result.data.accessToken, result.data.refreshToken)) }
            result
        } catch (e: TokenStorageException) {
            ApiResult.Failure(
                httpStatus = 0,
                code = ApiErrorCodes.STORAGE_FAILED,
                message = e.cause?.javaClass?.simpleName.orEmpty(),
                requestId = null,
                retryAfterSeconds = null,
            )
        }
    }

    override suspend fun logout() {
        try {
            // 204 sans corps. Le résultat est ignoré : hors ligne ou session déjà révoquée,
            // l'utilisateur doit quand même être déconnecté de l'appareil.
            if (withContext(io) { tokens.current() } != null) {
                apiCall(ifEmpty = { Unit }) { api.logout() }
            }
        } finally {
            withContext(NonCancellable + io) { tokens.clear() }
        }
    }
}
