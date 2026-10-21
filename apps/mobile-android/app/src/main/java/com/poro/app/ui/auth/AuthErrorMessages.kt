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
        else -> ErrorMessage(R.string.error_unexpected, requestId)
    }
}
