package com.poro.app.testing

import com.poro.app.core.network.ApiResult
import com.poro.app.data.user.ProfileDto
import com.poro.app.data.user.ProfileRepository
import com.poro.app.data.user.UsernameAvailability

/** Réponses programmées dans l'ordre ; enregistre les appels reçus. */
class FakeProfileRepository : ProfileRepository {
    var profile = ProfileDto(userId = "u1", username = "awa", displayName = "Awa")
    val availability = mutableMapOf<String, ApiResult<UsernameAvailability>>()
    val updateResults = ArrayDeque<ApiResult<ProfileDto>>()
    val avatarResults = ArrayDeque<ApiResult<ProfileDto>>()
    val calls = mutableListOf<String>()

    override suspend fun me(): ApiResult<ProfileDto> = ApiResult.Success(profile)

    override suspend fun update(username: String?, displayName: String, bio: String): ApiResult<ProfileDto> {
        calls += "update:$username:$displayName:$bio"
        return updateResults.removeFirst()
    }

    override suspend fun usernameAvailability(username: String): ApiResult<UsernameAvailability> {
        calls += "availability:$username"
        return availability[username] ?: ApiResult.Success(UsernameAvailability(username, available = true))
    }

    override suspend fun byUsername(username: String): ApiResult<ProfileDto> = ApiResult.Success(profile)

    override suspend fun uploadAvatar(bytes: ByteArray, contentType: String): ApiResult<ProfileDto> {
        calls += "avatar:${bytes.size}:$contentType"
        return avatarResults.removeFirst()
    }

    override suspend fun removeAvatar(): ApiResult<ProfileDto> = ApiResult.Success(profile.copy(avatarUrl = null))

    override suspend fun becomeCreator(): ApiResult<ProfileDto> = ApiResult.Success(profile.copy(isCreator = true))
}
