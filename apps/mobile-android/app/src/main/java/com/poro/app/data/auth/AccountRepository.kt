package com.poro.app.data.auth

import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.apiCall
import com.poro.app.core.security.TokenStore
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.withContext

/** Compte d'authentification : identité, appareils connectés, suppression. */
interface AccountRepository {
    /** Compte courant : email ou téléphone, adresse vérifiée, suppression programmée. */
    suspend fun account(): ApiResult<UserDto>

    /**
     * Demande la suppression du compte. En cas de succès, tous les appareils sont déconnectés
     * côté serveur : la session locale est effacée aussi.
     */
    suspend fun deleteAccount(password: String?, otpCode: String?): ApiResult<DeletionScheduled>

    suspend fun sessions(): ApiResult<List<SessionDto>>

    suspend fun revokeSession(id: String): ApiResult<Unit>

    suspend fun revokeOtherSessions(): ApiResult<RevokedCount>
}

class DefaultAccountRepository(
    private val api: AuthApi,
    private val tokens: TokenStore,
    private val io: CoroutineDispatcher = Dispatchers.IO,
) : AccountRepository {

    override suspend fun account(): ApiResult<UserDto> = apiCall { api.me() }

    override suspend fun deleteAccount(password: String?, otpCode: String?): ApiResult<DeletionScheduled> {
        val result = apiCall { api.deleteAccount(DeleteAccount(password, otpCode)) }
        if (result is ApiResult.Success) withContext(NonCancellable + io) { tokens.clear() }
        return result
    }

    override suspend fun sessions(): ApiResult<List<SessionDto>> = apiCall { api.sessions() }

    override suspend fun revokeSession(id: String): ApiResult<Unit> = apiCall(ifEmpty = {}) { api.revokeSession(id) }

    override suspend fun revokeOtherSessions(): ApiResult<RevokedCount> = apiCall { api.revokeOtherSessions() }
}
