package com.poro.app.data.user

import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PresignedUploader
import com.poro.app.core.network.UploadResult
import com.poro.app.core.network.apiCall
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject

/** Profil de l'utilisateur connecté et profils publics. */
interface ProfileRepository {
    suspend fun me(): ApiResult<ProfileDto>

    /** Remplace pseudo, nom affiché et bio ; un nom ou une bio vide est effacé (null côté serveur). */
    suspend fun update(username: String?, displayName: String, bio: String): ApiResult<ProfileDto>

    suspend fun usernameAvailability(username: String): ApiResult<UsernameAvailability>

    suspend fun byUsername(username: String): ApiResult<ProfileDto>

    /** Envoie l'image au stockage puis la fait valider (recadrée en 512×512, sans métadonnées). */
    suspend fun uploadAvatar(bytes: ByteArray, contentType: String): ApiResult<ProfileDto>

    suspend fun removeAvatar(): ApiResult<ProfileDto>

    suspend fun becomeCreator(): ApiResult<ProfileDto>
}

class DefaultProfileRepository(
    private val api: UserApi,
    private val uploader: PresignedUploader,
) : ProfileRepository {

    override suspend fun me(): ApiResult<ProfileDto> = apiCall { api.me() }

    override suspend fun update(username: String?, displayName: String, bio: String): ApiResult<ProfileDto> {
        val body = buildJsonObject {
            username?.let { put("username", JsonPrimitive(it)) }
            put("display_name", displayName.trim().ifEmpty { null }?.let(::JsonPrimitive) ?: JsonNull)
            put("bio", bio.trim().ifEmpty { null }?.let(::JsonPrimitive) ?: JsonNull)
        }
        return apiCall { api.updateMe(body) }
    }

    override suspend fun usernameAvailability(username: String): ApiResult<UsernameAvailability> =
        apiCall { api.usernameAvailability(username) }

    override suspend fun byUsername(username: String): ApiResult<ProfileDto> = apiCall { api.byUsername(username) }

    override suspend fun uploadAvatar(bytes: ByteArray, contentType: String): ApiResult<ProfileDto> {
        val form = when (val result = apiCall { api.avatarUploadUrl(ImageUploadRequest(contentType)) }) {
            is ApiResult.Success -> result.data
            is ApiResult.Failure -> return result
            is ApiResult.NetworkError -> return result
        }
        return when (val upload = uploader.postForm(form.uploadUrl, form.fields, bytes, contentType)) {
            is UploadResult.Success -> apiCall { api.confirmAvatar(UploadKey(form.uploadKey)) }
            is UploadResult.Rejected -> uploadRejected(upload.httpStatus)
            is UploadResult.NetworkError -> ApiResult.NetworkError(upload.cause)
        }
    }

    override suspend fun removeAvatar(): ApiResult<ProfileDto> = apiCall { api.deleteAvatar() }

    override suspend fun becomeCreator(): ApiResult<ProfileDto> = apiCall { api.becomeCreator() }
}

/** Le stockage a refusé le fichier (trop gros, formulaire expiré) : pas d'enveloppe, code client. */
internal fun uploadRejected(httpStatus: Int): ApiResult.Failure =
    ApiResult.Failure(httpStatus, ApiErrorCodes.UPLOAD_REJECTED, "storage refused the upload", null, null)
