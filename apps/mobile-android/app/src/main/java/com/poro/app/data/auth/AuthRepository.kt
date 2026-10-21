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

    /** Crée un compte email et ouvre sa session ; un code de vérification part en arrière-plan. */
    suspend fun register(email: String, password: String, fullName: String): ApiResult<SignedIn>

    suspend fun login(email: String, password: String): ApiResult<SignedIn>

    /** Renvoie un code de vérification à l'adresse du compte connecté. */
    suspend fun requestEmailVerification(): ApiResult<CodeSent>

    suspend fun confirmEmailVerification(code: String): ApiResult<UserDto>

    /** Toujours un succès si la requête aboutit : la réponse ne révèle pas si l'adresse a un compte. */
    suspend fun forgotPassword(email: String): ApiResult<CodeSent>

    /** Change le mot de passe ; tous les appareils du compte sont déconnectés. */
    suspend fun resetPassword(email: String, code: String, newPassword: String): ApiResult<Unit>

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

    override suspend fun verifyOtp(phone: String, code: String): ApiResult<SignedIn> =
        saveSession(apiCall { api.verifyOtp(OtpVerify(phone, code, deviceName())) })

    override suspend fun register(email: String, password: String, fullName: String): ApiResult<SignedIn> =
        saveSession(apiCall { api.emailRegister(EmailRegister(email, password, fullName, deviceName())) })

    override suspend fun login(email: String, password: String): ApiResult<SignedIn> =
        saveSession(apiCall { api.emailLogin(EmailLogin(email, password, deviceName())) })

    override suspend fun requestEmailVerification(): ApiResult<CodeSent> =
        apiCall { api.requestEmailVerification() }

    override suspend fun confirmEmailVerification(code: String): ApiResult<UserDto> =
        apiCall { api.confirmEmailVerification(CodeConfirm(code)) }

    override suspend fun forgotPassword(email: String): ApiResult<CodeSent> =
        apiCall { api.forgotPassword(PasswordForgot(email)) }

    override suspend fun resetPassword(email: String, code: String, newPassword: String): ApiResult<Unit> =
        apiCall(ifEmpty = { Unit }) { api.resetPassword(PasswordReset(email, code, newPassword)) }

    /** Enregistre la session d'une connexion réussie (OTP, inscription ou connexion email). */
    private suspend fun saveSession(result: ApiResult<SignedIn>): ApiResult<SignedIn> {
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
