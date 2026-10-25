package com.poro.app.data.video

import com.poro.app.core.network.ApiEnvelope
import com.poro.app.core.network.PageDto
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.POST
import retrofit2.http.Path
import retrofit2.http.Query

/** Service vidéo : envoi multipart S3, métadonnées et URL HLS (docs/api/video.openapi.yaml). */
interface VideoApi {
    @GET("api/v1/videos/{id}")
    suspend fun video(@Path("id") id: String): Response<ApiEnvelope<VideoDto>>

    /** Publications visibles par tous d'un utilisateur (prêtes, non retirées), pour son profil. */
    @GET("api/v1/videos/users/{userId}")
    suspend fun byUser(
        @Path("userId") userId: String,
        @Query("cursor") cursor: String?,
        @Query("limit") limit: Int,
    ): Response<ApiEnvelope<PageDto<VideoDto>>>

    /** Ses propres vidéos, tous statuts confondus, plus récentes d'abord. */
    @GET("api/v1/videos")
    suspend fun mine(@Query("cursor") cursor: String?, @Query("limit") limit: Int): Response<ApiEnvelope<PageDto<VideoDto>>>

    /** 204 sans corps ; les médias quittent le bucket public peu après. */
    @DELETE("api/v1/videos/{id}")
    suspend fun delete(@Path("id") id: String): Response<ApiEnvelope<Unit>>

    @POST("api/v1/videos/uploads")
    suspend fun initUpload(@Body body: InitUploadRequest): Response<ApiEnvelope<InitUpload>>

    /** Reprise d'un envoi `uploading` : parties déjà reçues et nouvelles URL pour les autres. */
    @POST("api/v1/videos/uploads/{id}/resume")
    suspend fun resumeUpload(@Path("id") id: String): Response<ApiEnvelope<ResumeUpload>>

    @POST("api/v1/videos/uploads/{id}/complete")
    suspend fun completeUpload(@Path("id") id: String, @Body body: CompleteUpload): Response<ApiEnvelope<VideoDto>>

    /** 204 sans corps ; annule aussi une publication photo pas encore finalisée. */
    @DELETE("api/v1/videos/uploads/{id}")
    suspend fun abortUpload(@Path("id") id: String): Response<ApiEnvelope<Unit>>

    /** Publication photo (ADR-0020) : une URL PUT signée (taille et type) par image. */
    @POST("api/v1/videos/photo-uploads")
    suspend fun initPhotoUpload(@Body body: InitPhotoUploadRequest): Response<ApiEnvelope<InitPhotoUpload>>

    /** Vérifie les images envoyées et passe la publication en traitement ; rejoué : 409. */
    @POST("api/v1/videos/photo-uploads/{id}/complete")
    suspend fun completePhotoUpload(@Path("id") id: String): Response<ApiEnvelope<VideoDto>>
}

@Serializable
data class VideoDto(
    val id: String,
    @SerialName("user_id") val userId: String = "",
    /** `video` ou `photo` (publication photo, avec ses [images] et sans HLS). */
    @SerialName("media_type") val mediaType: String = MEDIA_VIDEO,
    val title: String = "",
    val description: String = "",
    /** `uploading`, `processing`, `ready` ou `failed`. */
    val status: String = "",
    /** `approved` ou `removed` (masquée par la modération ; seul l'auteur la voit, sans média). */
    @SerialName("moderation_status") val moderationStatus: String = "approved",
    @SerialName("duration_ms") val durationMs: Long = 0,
    val width: Int = 0,
    val height: Int = 0,
    @SerialName("hls_url") val hlsUrl: String? = null,
    @SerialName("thumbnail_url") val thumbnailUrl: String? = null,
    @SerialName("created_at") val createdAt: String? = null,
    /** Images d'une publication photo prête, dans l'ordre. */
    val images: List<ImageDto> = emptyList(),
) {
    val isReady: Boolean get() = status == STATUS_READY && moderationStatus != MODERATION_REMOVED

    /** Titre à afficher : le titre, facultatif, ou la première ligne de la description. */
    val headline: String get() = title.ifBlank { description.lineSequence().map { it.trim() }.firstOrNull { it.isNotEmpty() }.orEmpty() }
    val isPhoto: Boolean get() = mediaType == MEDIA_PHOTO

    companion object {
        const val STATUS_UPLOADING = "uploading"
        const val STATUS_PROCESSING = "processing"
        const val STATUS_READY = "ready"
        const val STATUS_FAILED = "failed"
        const val MODERATION_REMOVED = "removed"
        const val MEDIA_VIDEO = "video"
        const val MEDIA_PHOTO = "photo"
    }
}

/** Une image traitée : JPEG de 2048 px au plus et sa miniature de 540 px. */
@Serializable
data class ImageDto(
    val position: Int = 0,
    val url: String,
    @SerialName("thumbnail_url") val thumbnailUrl: String = url,
    val width: Int = 0,
    val height: Int = 0,
)

@Serializable
data class InitPhotoUploadRequest(
    val images: List<PhotoSpec>,
    /** Légende facultative (500 caractères, hashtags compris). */
    val description: String? = null,
)

@Serializable
data class PhotoSpec(
    @SerialName("content_type") val contentType: String,
    @SerialName("size_bytes") val sizeBytes: Long,
)

/** URL de chaque image, à envoyer avec exactement les [PhotoTarget.headers] signés. */
@Serializable
data class InitPhotoUpload(
    @SerialName("video_id") val videoId: String,
    @SerialName("expires_in") val expiresIn: Long = 0,
    val images: List<PhotoTarget>,
)

@Serializable
data class PhotoTarget(
    val position: Int,
    val url: String,
    val headers: Map<String, String> = emptyMap(),
)

@Serializable
data class InitUploadRequest(
    /** Facultatif (100 caractères) : vide, les lecteurs affichent la description. */
    val title: String = "",
    val description: String? = null,
    val filename: String? = null,
    @SerialName("content_type") val contentType: String,
    @SerialName("size_bytes") val sizeBytes: Long,
)

/** Une URL présignée par partie de [partSize] octets (8 Mio). */
@Serializable
data class InitUpload(
    @SerialName("video_id") val videoId: String,
    @SerialName("upload_id") val uploadId: String = "",
    val key: String = "",
    @SerialName("part_size") val partSize: Long,
    val parts: List<UploadPart>,
)

/** Ce qu'il reste à envoyer : le stockage fait foi pour les parties déjà reçues. */
@Serializable
data class ResumeUpload(
    @SerialName("video_id") val videoId: String,
    @SerialName("part_size") val partSize: Long,
    @SerialName("uploaded_parts") val uploadedParts: List<CompletedPart> = emptyList(),
    val parts: List<UploadPart> = emptyList(),
)

@Serializable
data class UploadPart(@SerialName("part_number") val partNumber: Int, val url: String)

@Serializable
data class CompleteUpload(val parts: List<CompletedPart>)

@Serializable
data class CompletedPart(@SerialName("part_number") val partNumber: Int, val etag: String)
