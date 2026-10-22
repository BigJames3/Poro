package com.poro.app.testing

import com.poro.app.core.network.ApiResult
import com.poro.app.data.auth.AuthRepository
import com.poro.app.data.auth.CodeSent
import com.poro.app.data.auth.SignedIn
import com.poro.app.data.auth.UserDto

/** Réponses programmées dans l'ordre ; enregistre les appels reçus. */
class FakeAuthRepository : AuthRepository {
    val requestResults = ArrayDeque<ApiResult<CodeSent>>()
    val verifyResults = ArrayDeque<ApiResult<SignedIn>>()
    val signInResults = ArrayDeque<ApiResult<SignedIn>>()
    val emailCodeResults = ArrayDeque<ApiResult<CodeSent>>()
    val confirmResults = ArrayDeque<ApiResult<UserDto>>()
    val resetResults = ArrayDeque<ApiResult<Unit>>()
    val requested = mutableListOf<String>()
    val verified = mutableListOf<Pair<String, String>>()
    /** Appels email, dans l'ordre : "login:awa@…", "confirm:123456"… (jamais le mot de passe). */
    val emailCalls = mutableListOf<String>()
    var loggedOut = 0

    override suspend fun requestOtp(phone: String): ApiResult<CodeSent> {
        requested += phone
        return requestResults.removeFirst()
    }

    override suspend fun verifyOtp(phone: String, code: String): ApiResult<SignedIn> {
        verified += phone to code
        return verifyResults.removeFirst()
    }

    override suspend fun register(email: String, password: String, fullName: String): ApiResult<SignedIn> {
        emailCalls += "register:$email:$fullName"
        return signInResults.removeFirst()
    }

    override suspend fun login(email: String, password: String): ApiResult<SignedIn> {
        emailCalls += "login:$email"
        return signInResults.removeFirst()
    }

    override suspend fun requestEmailVerification(): ApiResult<CodeSent> {
        emailCalls += "verification"
        return emailCodeResults.removeFirst()
    }

    override suspend fun confirmEmailVerification(code: String): ApiResult<UserDto> {
        emailCalls += "confirm:$code"
        return confirmResults.removeFirst()
    }

    override suspend fun forgotPassword(email: String): ApiResult<CodeSent> {
        emailCalls += "forgot:$email"
        return emailCodeResults.removeFirst()
    }

    override suspend fun resetPassword(email: String, code: String, newPassword: String): ApiResult<Unit> {
        emailCalls += "reset:$email:$code"
        return resetResults.removeFirst()
    }

    override suspend fun logout() {
        loggedOut++
    }

    companion object {
        fun failure(status: Int, code: String, retryAfter: Long? = null) =
            ApiResult.Failure(status, code, "m", "req-1", retryAfter)

        fun signedIn(deletionCancelled: Boolean = false) =
            ApiResult.Success(SignedIn("access", "refresh", 900, null, deletionCancelled))
    }
}
