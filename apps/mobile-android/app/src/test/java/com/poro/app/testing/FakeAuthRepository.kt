package com.poro.app.testing

import com.poro.app.core.network.ApiResult
import com.poro.app.data.auth.AuthRepository
import com.poro.app.data.auth.CodeSent
import com.poro.app.data.auth.SignedIn

/** Réponses programmées dans l'ordre ; enregistre les appels reçus. */
class FakeAuthRepository : AuthRepository {
    val requestResults = ArrayDeque<ApiResult<CodeSent>>()
    val verifyResults = ArrayDeque<ApiResult<SignedIn>>()
    val requested = mutableListOf<String>()
    val verified = mutableListOf<Pair<String, String>>()
    var loggedOut = 0

    override suspend fun requestOtp(phone: String): ApiResult<CodeSent> {
        requested += phone
        return requestResults.removeFirst()
    }

    override suspend fun verifyOtp(phone: String, code: String): ApiResult<SignedIn> {
        verified += phone to code
        return verifyResults.removeFirst()
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
