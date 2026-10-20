package com.poro.app.ui.auth

import com.poro.app.R
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import java.io.IOException
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class AuthErrorMessagesTest {

    private fun failure(code: String) = ApiResult.Failure(400, code, "english text", "req-9", null)

    @Test
    fun `every documented auth code has its own French message without reference`() {
        val expected = mapOf(
            ApiErrorCodes.INVALID_REQUEST to R.string.error_invalid_request,
            ApiErrorCodes.INVALID_JSON to R.string.error_invalid_request,
            ApiErrorCodes.PHONE_COUNTRY_NOT_SUPPORTED to R.string.error_phone_country_not_supported,
            ApiErrorCodes.PHONE_UNREACHABLE to R.string.error_phone_unreachable,
            ApiErrorCodes.OTP_THROTTLED to R.string.error_otp_throttled,
            ApiErrorCodes.RATE_LIMITED to R.string.error_rate_limited,
            ApiErrorCodes.SMS_UNAVAILABLE to R.string.error_sms_unavailable,
            ApiErrorCodes.OTP_INVALID to R.string.error_otp_invalid,
            ApiErrorCodes.OTP_EXPIRED to R.string.error_otp_expired,
            ApiErrorCodes.OTP_TOO_MANY_ATTEMPTS to R.string.error_otp_too_many_attempts,
            ApiErrorCodes.ACCOUNT_BLOCKED to R.string.error_account_blocked,
            ApiErrorCodes.STORAGE_FAILED to R.string.error_storage_failed,
        )
        for ((code, text) in expected) {
            assertEquals(code, ErrorMessage(text), failure(code).toErrorMessage())
        }
    }

    @Test
    fun `unknown codes fall back to a generic message carrying the request id`() {
        assertEquals(ErrorMessage(R.string.error_unexpected, "req-9"), failure(ApiErrorCodes.INTERNAL_ERROR).toErrorMessage())
        assertEquals(ErrorMessage(R.string.error_unexpected, "req-9"), failure("added_later_by_backend").toErrorMessage())
    }

    @Test
    fun `network errors and successes`() {
        assertEquals(ErrorMessage(R.string.error_network), ApiResult.NetworkError(IOException()).toErrorMessage())
        assertNull(ApiResult.Success(Unit).toErrorMessage())
    }
}
