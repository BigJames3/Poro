package com.poro.app.data.user

import com.poro.app.core.network.ApiEnvelope
import com.poro.app.core.network.PageDto
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonObject
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.PATCH
import retrofit2.http.POST
import retrofit2.http.PUT
import retrofit2.http.Path
import retrofit2.http.Query

/** Service user : profils publics, pseudos, avatars (docs/api/user.openapi.yaml). */
interface UserApi {
    @GET("api/v1/users/me")
    suspend fun me(): Response<ApiEnvelope<ProfileDto>>

    /** Champ omis : inchangé ; `null` : effacé. D'où un JsonObject, PoroJson omettant les null. */
    @PATCH("api/v1/users/me")
    suspend fun updateMe(@Body body: JsonObject): Response<ApiEnvelope<ProfileDto>>

    @GET("api/v1/users/username-availability")
    suspend fun usernameAvailability(@Query("username") username: String): Response<ApiEnvelope<UsernameAvailability>>

    /** Profils publics de 1 à 100 comptes ; comptes inconnus ou supprimés omis. */
    @GET("api/v1/users")
    suspend fun profiles(@Query("ids") ids: String): Response<ApiEnvelope<PageDto<ProfileDto>>>

    @GET("api/v1/users/by-username/{username}")
    suspend fun byUsername(@Path("username") username: String): Response<ApiEnvelope<ProfileDto>>

    @POST("api/v1/users/me/avatar/upload-url")
    suspend fun avatarUploadUrl(@Body body: ImageUploadRequest): Response<ApiEnvelope<PresignedForm>>

    @PUT("api/v1/users/me/avatar")
    suspend fun confirmAvatar(@Body body: UploadKey): Response<ApiEnvelope<ProfileDto>>

    @DELETE("api/v1/users/me/avatar")
    suspend fun deleteAvatar(): Response<ApiEnvelope<ProfileDto>>

    @POST("api/v1/users/me/creator")
    suspend fun becomeCreator(): Response<ApiEnvelope<ProfileDto>>
}

/** Profil public ; [countryCode] n'est présent que sur son propre profil. */
@Serializable
data class ProfileDto(
    @SerialName("user_id") val userId: String,
    val username: String? = null,
    @SerialName("display_name") val displayName: String? = null,
    val bio: String? = null,
    @SerialName("avatar_url") val avatarUrl: String? = null,
    @SerialName("is_creator") val isCreator: Boolean = false,
    @SerialName("creator_since") val creatorSince: String? = null,
    @SerialName("country_code") val countryCode: String? = null,
)

@Serializable
data class UsernameAvailability(
    val username: String,
    val available: Boolean,
    /** `invalid`, `reserved`, `taken`, ou null si disponible. */
    val reason: String? = null,
)

@Serializable
data class ImageUploadRequest(@SerialName("content_type") val contentType: String)

/** Formulaire POST présigné : envoyer [fields] puis le fichier à [uploadUrl], puis confirmer [uploadKey]. */
@Serializable
data class PresignedForm(
    @SerialName("upload_url") val uploadUrl: String,
    val fields: Map<String, String> = emptyMap(),
    @SerialName("upload_key") val uploadKey: String,
    @SerialName("max_bytes") val maxBytes: Long = 0,
    @SerialName("expires_in") val expiresIn: Int = 0,
)

@Serializable
data class UploadKey(@SerialName("upload_key") val uploadKey: String)
