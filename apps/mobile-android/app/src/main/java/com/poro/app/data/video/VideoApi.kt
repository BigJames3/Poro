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

    /** Ses propres vidéos, tous statuts confondus, plus récentes d'abord. */
    @GET("api/v1/videos")
    suspend fun mine(@Query("cursor") cursor: String?, @Query("limit") limit: Int): Response<ApiEnvelope<PageDto<VideoDto>>>

    /** 204 sans corps ; les médias quittent le bucket public peu après. */
    @DELETE("api/v1/videos/{id}")
    suspend fun delete(@Path("id") id: String): Response<ApiEnvelope<Unit>>

    @POST("api/v1/videos/uploads")
    suspend fun initUpload(@Body body: InitUploadRequest): Response<ApiEnvelope<InitUpload>>

    @POST("api/v1/videos/uploads/{id}/complete")
    suspend fun completeUpload(@Path("id") id: String, @Body body: CompleteUpload): Response<ApiEnvelope<VideoDto>>

    /** 204 sans corps. */
    @DELETE("api/v1/videos/uploads/{id}")
    suspend fun abortUpload(@Path("id") id: String): Response<ApiEnvelope<Unit>>
}

@Serializable
data class VideoDto(
    val id: String,
    @SerialName("user_id") val userId: String = "",
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
) {
    val isReady: Boolean get() = status == STATUS_READY && moderationStatus != MODERATION_REMOVED

    companion object {
        const val STATUS_UPLOADING = "uploading"
        const val STATUS_PROCESSING = "processing"
        const val STATUS_READY = "ready"
        const val STATUS_FAILED = "failed"
        const val MODERATION_REMOVED = "removed"
    }
}

@Serializable
data class InitUploadRequest(
    val title: String,
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

@Serializable
data class UploadPart(@SerialName("part_number") val partNumber: Int, val url: String)

@Serializable
data class CompleteUpload(val parts: List<CompletedPart>)

@Serializable
data class CompletedPart(@SerialName("part_number") val partNumber: Int, val etag: String)
