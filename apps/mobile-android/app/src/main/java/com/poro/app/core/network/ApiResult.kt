package com.poro.app.core.network

import java.io.IOException
import kotlin.coroutines.cancellation.CancellationException
import retrofit2.Response

/** Résultat d'un appel à l'API, sans exception à attraper côté écran. */
sealed interface ApiResult<out T> {
    data class Success<T>(val data: T) : ApiResult<T>

    /**
     * Le serveur a répondu avec une erreur de l'enveloppe.
     * [retryAfterSeconds] vient de l'en-tête `Retry-After` (429, connexion verrouillée) :
     * l'interface doit attendre ce délai, jamais réessayer en boucle.
     */
    data class Failure(
        val httpStatus: Int,
        val code: String,
        val message: String,
        val requestId: String?,
        val retryAfterSeconds: Long?,
    ) : ApiResult<Nothing>

    /** Pas de réponse : réseau coupé, délai dépassé, serveur injoignable. */
    data class NetworkError(val cause: IOException) : ApiResult<Nothing>
}

/** Codes d'erreur communs (fichiers docs/api/<domaine>.openapi.yaml). Chaque domaine en ajoute d'autres. */
object ApiErrorCodes {
    const val INVALID_REQUEST = "invalid_request"
    const val INVALID_JSON = "invalid_json"
    const val UNAUTHORIZED = "unauthorized"
    const val RATE_LIMITED = "rate_limited"
    const val UNAVAILABLE = "unavailable"
    const val ACCOUNT_BLOCKED = "account_blocked"
    const val REFRESH_TOKEN_INVALID = "refresh_token_invalid"
    const val SESSION_REVOKED = "session_revoked"
    const val INTERNAL_ERROR = "internal_error"

    // Connexion par téléphone (docs/api/auth.openapi.yaml).
    const val OTP_INVALID = "otp_invalid"
    const val OTP_EXPIRED = "otp_expired"
    const val OTP_TOO_MANY_ATTEMPTS = "otp_too_many_attempts"
    const val OTP_THROTTLED = "otp_throttled"
    const val PHONE_COUNTRY_NOT_SUPPORTED = "phone_country_not_supported"
    const val PHONE_UNREACHABLE = "phone_unreachable"
    const val SMS_UNAVAILABLE = "sms_unavailable"

    /** Réponse d'erreur illisible (pas d'enveloppe) : propre au client. */
    const val UNEXPECTED_RESPONSE = "unexpected_response"

    /** Session acceptée par le serveur mais impossible à enregistrer sur l'appareil : propre au client. */
    const val STORAGE_FAILED = "storage_failed"
}

/**
 * Délai à respecter avant de réessayer après un 429, ou null pour une autre erreur :
 * `Retry-After` quand le serveur l'envoie, sinon un recul exponentiel selon le nombre de 429
 * consécutifs [consecutive] (5 s, 10 s, 20 s… 60 s au plus). Jamais de réessai immédiat.
 */
fun ApiResult.Failure.retryDelaySeconds(consecutive: Int): Long? {
    if (httpStatus != 429) return null
    retryAfterSeconds?.takeIf { it > 0 }?.let { return it }
    val exponent = (consecutive - 1).coerceIn(0, 4)
    return (BACKOFF_BASE_SECONDS shl exponent).coerceAtMost(BACKOFF_MAX_SECONDS)
}

private const val BACKOFF_BASE_SECONDS = 5L
private const val BACKOFF_MAX_SECONDS = 60L

/**
 * Exécute un appel Retrofit et convertit sa réponse en [ApiResult].
 * Les réponses sans corps (204) produisent `Success(Unit)` via [ifEmpty].
 */
suspend fun <T> apiCall(
    ifEmpty: (() -> T)? = null,
    call: suspend () -> Response<ApiEnvelope<T>>,
): ApiResult<T> {
    val response = try {
        call()
    } catch (e: CancellationException) {
        throw e
    } catch (e: IOException) {
        return ApiResult.NetworkError(e)
    }
    return response.toApiResult(ifEmpty)
}

internal fun <T> Response<ApiEnvelope<T>>.toApiResult(ifEmpty: (() -> T)? = null): ApiResult<T> {
    val retryAfter = headers()["Retry-After"]?.trim()?.toLongOrNull()
    if (isSuccessful) {
        val data = body()?.data
        return when {
            data != null -> ApiResult.Success(data)
            ifEmpty != null -> ApiResult.Success(ifEmpty())
            else -> ApiResult.Failure(code(), ApiErrorCodes.UNEXPECTED_RESPONSE, "empty body", requestIdHeader(), retryAfter)
        }
    }
    val raw = errorBody()?.string().orEmpty()
    val envelope = runCatching { PoroJson.decodeFromString<ApiEnvelope<Unit>>(raw) }.getOrNull()
    val error = envelope?.error
    return ApiResult.Failure(
        httpStatus = code(),
        code = error?.code ?: ApiErrorCodes.UNEXPECTED_RESPONSE,
        message = error?.message ?: "HTTP ${code()}",
        requestId = envelope?.meta?.requestId ?: requestIdHeader(),
        retryAfterSeconds = retryAfter,
    )
}

private fun Response<*>.requestIdHeader(): String? =
    raw().request.header(RequestIdInterceptor.HEADER)
