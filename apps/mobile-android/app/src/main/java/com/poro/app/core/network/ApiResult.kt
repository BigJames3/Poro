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

    // Connexion par email (docs/api/auth.openapi.yaml).
    const val EMAIL_TAKEN = "email_taken"
    const val INVALID_CREDENTIALS = "invalid_credentials"
    const val LOGIN_THROTTLED = "login_throttled"
    const val EMAIL_THROTTLED = "email_throttled"
    const val EMAIL_UNAVAILABLE = "email_unavailable"
    const val EMAIL_ALREADY_VERIFIED = "email_already_verified"
    const val EMAIL_MISSING = "email_missing"
    const val CODE_INVALID = "code_invalid"
    const val CODE_EXPIRED = "code_expired"
    const val CODE_TOO_MANY_ATTEMPTS = "code_too_many_attempts"

    // Compte et profil.
    const val REAUTH_REQUIRED = "reauth_required"
    const val NOT_FOUND = "not_found"
    const val FORBIDDEN = "forbidden"
    const val CONFLICT = "conflict"
    const val INVALID_CURSOR = "invalid_cursor"

    // Interactions (docs/api/social.openapi.yaml).
    const val VIDEO_NOT_FOUND = "video_not_found"
    const val COMMENT_NOT_FOUND = "comment_not_found"
    const val USER_NOT_FOUND = "user_not_found"
    const val CANNOT_FOLLOW_SELF = "cannot_follow_self"
    const val COMMENT_INVALID = "comment_invalid"
    const val COMMENT_EDIT_WINDOW_CLOSED = "comment_edit_window_closed"
    const val USERNAME_TAKEN = "username_taken"
    const val USERNAME_INVALID = "username_invalid"
    const val USERNAME_RESERVED = "username_reserved"
    const val USERNAME_REQUIRED = "username_required"
    const val DISPLAY_NAME_INVALID = "display_name_invalid"
    const val BIO_INVALID = "bio_invalid"
    const val PROFILE_NOT_FOUND = "profile_not_found"
    const val AVATAR_TOO_LARGE = "avatar_too_large"
    const val AVATAR_INVALID = "avatar_invalid"

    /** Le stockage a refusé un envoi direct (fichier trop gros, URL expirée) : propre au client. */
    const val UPLOAD_REJECTED = "upload_rejected"

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
