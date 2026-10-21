package com.poro.app.testing

import com.poro.app.core.network.ApiResult
import com.poro.app.data.auth.AccountRepository
import com.poro.app.data.auth.DeletionScheduled
import com.poro.app.data.auth.RevokedCount
import com.poro.app.data.auth.SessionDto
import com.poro.app.data.auth.UserDto

class FakeAccountRepository : AccountRepository {
    var account = UserDto(id = "u1", email = "awa@poro.africa", emailVerified = true)
    var sessions = listOf(SessionDto("s1", "Tecno Spark 20", current = true), SessionDto("s2", "Pixel 8"))
    val deleteResults = ArrayDeque<ApiResult<DeletionScheduled>>()
    val deletions = mutableListOf<Pair<String?, String?>>()

    override suspend fun account(): ApiResult<UserDto> = ApiResult.Success(account)

    override suspend fun deleteAccount(password: String?, otpCode: String?): ApiResult<DeletionScheduled> {
        deletions += password to otpCode
        return deleteResults.removeFirst()
    }

    override suspend fun sessions(): ApiResult<List<SessionDto>> = ApiResult.Success(sessions)

    override suspend fun revokeSession(id: String): ApiResult<Unit> {
        sessions = sessions.filterNot { it.id == id }
        return ApiResult.Success(Unit)
    }

    override suspend fun revokeOtherSessions(): ApiResult<RevokedCount> {
        val revoked = sessions.count { !it.current }
        sessions = sessions.filter { it.current }
        return ApiResult.Success(RevokedCount(revoked))
    }
}
