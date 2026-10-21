package com.poro.app.ui.auth

import androidx.annotation.StringRes
import com.poro.app.R
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult

/**
 * Message d'erreur à afficher. [requestId] n'est renseigné que pour les erreurs inattendues :
 * l'utilisateur peut le transmettre au support, qui retrouve la requête dans les logs.
 */
data class ErrorMessage(@StringRes val text: Int, val requestId: String? = null)

/** Traduit un échec de l'API Auth en message français, à partir du code stable du contrat. */
fun ApiResult<*>.toErrorMessage(): ErrorMessage? = when (this) {
    is ApiResult.Success -> null
    is ApiResult.NetworkError -> ErrorMessage(R.string.error_network)
    is ApiResult.Failure -> when (code) {
        ApiErrorCodes.INVALID_REQUEST, ApiErrorCodes.INVALID_JSON -> ErrorMessage(R.string.error_invalid_request)
        ApiErrorCodes.PHONE_COUNTRY_NOT_SUPPORTED -> ErrorMessage(R.string.error_phone_country_not_supported)
        ApiErrorCodes.PHONE_UNREACHABLE -> ErrorMessage(R.string.error_phone_unreachable)
        ApiErrorCodes.OTP_THROTTLED -> ErrorMessage(R.string.error_otp_throttled)
        ApiErrorCodes.RATE_LIMITED -> ErrorMessage(R.string.error_rate_limited)
        ApiErrorCodes.SMS_UNAVAILABLE -> ErrorMessage(R.string.error_sms_unavailable)
        ApiErrorCodes.OTP_INVALID -> ErrorMessage(R.string.error_otp_invalid)
        ApiErrorCodes.OTP_EXPIRED -> ErrorMessage(R.string.error_otp_expired)
        ApiErrorCodes.OTP_TOO_MANY_ATTEMPTS -> ErrorMessage(R.string.error_otp_too_many_attempts)
        ApiErrorCodes.ACCOUNT_BLOCKED -> ErrorMessage(R.string.error_account_blocked)
        ApiErrorCodes.STORAGE_FAILED -> ErrorMessage(R.string.error_storage_failed)
        ApiErrorCodes.EMAIL_TAKEN -> ErrorMessage(R.string.error_email_taken)
        ApiErrorCodes.INVALID_CREDENTIALS -> ErrorMessage(R.string.error_invalid_credentials)
        ApiErrorCodes.LOGIN_THROTTLED -> ErrorMessage(R.string.error_login_throttled)
        ApiErrorCodes.EMAIL_THROTTLED -> ErrorMessage(R.string.error_email_throttled)
        ApiErrorCodes.EMAIL_UNAVAILABLE -> ErrorMessage(R.string.error_email_unavailable)
        ApiErrorCodes.EMAIL_MISSING -> ErrorMessage(R.string.error_email_missing)
        ApiErrorCodes.CODE_INVALID -> ErrorMessage(R.string.error_code_invalid)
        ApiErrorCodes.CODE_EXPIRED -> ErrorMessage(R.string.error_code_expired)
        ApiErrorCodes.CODE_TOO_MANY_ATTEMPTS -> ErrorMessage(R.string.error_code_too_many_attempts)
        ApiErrorCodes.REAUTH_REQUIRED -> ErrorMessage(R.string.error_reauth_required)
        ApiErrorCodes.NOT_FOUND, ApiErrorCodes.PROFILE_NOT_FOUND -> ErrorMessage(R.string.error_not_found)
        ApiErrorCodes.FORBIDDEN -> ErrorMessage(R.string.error_forbidden)
        ApiErrorCodes.USERNAME_TAKEN -> ErrorMessage(R.string.username_taken)
        ApiErrorCodes.USERNAME_INVALID -> ErrorMessage(R.string.username_invalid)
        ApiErrorCodes.USERNAME_RESERVED -> ErrorMessage(R.string.username_reserved)
        ApiErrorCodes.USERNAME_REQUIRED -> ErrorMessage(R.string.error_username_required)
        ApiErrorCodes.DISPLAY_NAME_INVALID -> ErrorMessage(R.string.error_display_name_invalid)
        ApiErrorCodes.BIO_INVALID -> ErrorMessage(R.string.error_bio_invalid)
        ApiErrorCodes.AVATAR_TOO_LARGE -> ErrorMessage(R.string.error_avatar_too_large)
        ApiErrorCodes.AVATAR_INVALID -> ErrorMessage(R.string.error_avatar_invalid)
        ApiErrorCodes.UPLOAD_REJECTED -> ErrorMessage(R.string.error_upload_rejected)
        ApiErrorCodes.VIDEO_NOT_FOUND, ApiErrorCodes.COMMENT_NOT_FOUND, ApiErrorCodes.USER_NOT_FOUND -> ErrorMessage(R.string.error_not_found)
        ApiErrorCodes.COMMENT_INVALID -> ErrorMessage(R.string.error_comment_invalid)
        else -> ErrorMessage(R.string.error_unexpected, requestId)
    }
}
