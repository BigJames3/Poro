package com.poro.app.data.social

import com.poro.app.core.network.ApiEnvelope
import com.poro.app.core.network.PageDto
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.PATCH
import retrofit2.http.POST
import retrofit2.http.PUT
import retrofit2.http.Path
import retrofit2.http.Query

/**
 * Service social (docs/api/social.openapi.yaml) : j'aime, commentaires, abonnements, partages.
 * Les écritures sont idempotentes : répéter un j'aime ou un abonnement ne change rien.
 */
interface SocialApi {
    @PUT("api/v1/videos/{videoId}/like")
    suspend fun like(@Path("videoId") videoId: String): Response<ApiEnvelope<VideoStatsDto>>

    @DELETE("api/v1/videos/{videoId}/like")
    suspend fun unlike(@Path("videoId") videoId: String): Response<ApiEnvelope<VideoStatsDto>>

    @GET("api/v1/videos/{videoId}/stats")
    suspend fun videoStats(@Path("videoId") videoId: String): Response<ApiEnvelope<VideoStatsDto>>

    @GET("api/v1/videos/{videoId}/comments")
    suspend fun comments(
        @Path("videoId") videoId: String,
        @Query("cursor") cursor: String?,
        @Query("limit") limit: Int,
    ): Response<ApiEnvelope<PageDto<CommentDto>>>

    @GET("api/v1/comments/{commentId}/replies")
    suspend fun replies(
        @Path("commentId") commentId: String,
        @Query("cursor") cursor: String?,
        @Query("limit") limit: Int,
    ): Response<ApiEnvelope<PageDto<CommentDto>>>

    @POST("api/v1/videos/{videoId}/comments")
    suspend fun postComment(@Path("videoId") videoId: String, @Body body: NewComment): Response<ApiEnvelope<CommentDto>>

    @PATCH("api/v1/comments/{commentId}")
    suspend fun editComment(@Path("commentId") commentId: String, @Body body: CommentText): Response<ApiEnvelope<CommentDto>>

    /** 204 sans corps ; l'auteur ou le propriétaire de la vidéo. */
    @DELETE("api/v1/comments/{commentId}")
    suspend fun deleteComment(@Path("commentId") commentId: String): Response<ApiEnvelope<Unit>>

    @PUT("api/v1/comments/{commentId}/like")
    suspend fun likeComment(@Path("commentId") commentId: String): Response<ApiEnvelope<CommentLikeDto>>

    @DELETE("api/v1/comments/{commentId}/like")
    suspend fun unlikeComment(@Path("commentId") commentId: String): Response<ApiEnvelope<CommentLikeDto>>

    @PUT("api/v1/users/{userId}/follow")
    suspend fun follow(@Path("userId") userId: String): Response<ApiEnvelope<UserStatsDto>>

    @DELETE("api/v1/users/{userId}/follow")
    suspend fun unfollow(@Path("userId") userId: String): Response<ApiEnvelope<UserStatsDto>>

    @GET("api/v1/users/{userId}/stats")
    suspend fun userStats(@Path("userId") userId: String): Response<ApiEnvelope<UserStatsDto>>

    @POST("api/v1/videos/{videoId}/shares")
    suspend fun share(@Path("videoId") videoId: String, @Body body: ShareRequest): Response<ApiEnvelope<ShareResultDto>>
}

@Serializable
data class VideoStatsDto(
    @SerialName("video_id") val videoId: String = "",
    val likes: Long = 0,
    val comments: Long = 0,
    val shares: Long = 0,
    @SerialName("liked_by_me") val likedByMe: Boolean = false,
)

@Serializable
data class UserStatsDto(
    @SerialName("user_id") val userId: String = "",
    val followers: Long = 0,
    val following: Long = 0,
    @SerialName("followed_by_me") val followedByMe: Boolean = false,
)

/** Un commentaire supprimé reste (sans auteur ni texte) tant qu'il a des réponses visibles. */
@Serializable
data class CommentDto(
    val id: String,
    @SerialName("video_id") val videoId: String = "",
    @SerialName("user_id") val userId: String? = null,
    @SerialName("parent_id") val parentId: String? = null,
    val content: String? = null,
    @SerialName("likes_count") val likesCount: Long = 0,
    @SerialName("replies_count") val repliesCount: Long = 0,
    @SerialName("liked_by_me") val likedByMe: Boolean = false,
    val edited: Boolean = false,
    val deleted: Boolean = false,
    @SerialName("created_at") val createdAt: String? = null,
)

@Serializable
data class NewComment(val content: String, @SerialName("parent_id") val parentId: String? = null)

@Serializable
data class CommentText(val content: String)

@Serializable
data class CommentLikeDto(
    @SerialName("comment_id") val commentId: String = "",
    @SerialName("likes_count") val likesCount: Long = 0,
    @SerialName("liked_by_me") val likedByMe: Boolean = false,
)

/** Canal du partage : `whatsapp`, `copy_link` ou `other`. */
@Serializable
data class ShareRequest(val channel: String)

@Serializable
data class ShareResultDto(
    @SerialName("share_id") val shareId: String = "",
    @SerialName("shares_count") val sharesCount: Long = 0,
)
