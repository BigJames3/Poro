package com.poro.app.data.notification

import com.poro.app.core.network.ApiEnvelope
import com.poro.app.core.network.PageDto
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.GET
import retrofit2.http.PATCH
import retrofit2.http.POST
import retrofit2.http.Path
import retrofit2.http.Query

/**
 * Notifications dans l'application et préférences (docs/api/notification.openapi.yaml).
 * L'enregistrement des appareils pour les notifications push attend le projet Firebase.
 */
interface NotificationApi {
    @GET("api/v1/notifications")
    suspend fun list(@Query("cursor") cursor: String?, @Query("limit") limit: Int): Response<ApiEnvelope<PageDto<NotificationDto>>>

    @GET("api/v1/notifications/unread-count")
    suspend fun unreadCount(): Response<ApiEnvelope<UnreadCount>>

    @PATCH("api/v1/notifications/{id}/read")
    suspend fun markRead(@Path("id") id: String): Response<ApiEnvelope<ReadReceipt>>

    @POST("api/v1/notifications/read-all")
    suspend fun readAll(): Response<ApiEnvelope<ReadAll>>

    @GET("api/v1/notifications/preferences")
    suspend fun preferences(): Response<ApiEnvelope<PreferencesDto>>

    @PATCH("api/v1/notifications/preferences")
    suspend fun updatePreferences(@Body body: PreferencesDto): Response<ApiEnvelope<PreferencesDto>>
}

@Serializable
data class NotificationDto(
    val id: String,
    /** like, comment, reply, follow, video_ready, order_new, order_confirmed, order_shipped, order_cancelled, order_completed. */
    val type: String = "",
    val title: String = "",
    val body: String = "",
    /** Cible de navigation : video_id, comment_id, user_id, order_id et role (buyer ou seller). */
    val data: Map<String, String> = emptyMap(),
    val actor: ActorDto? = null,
    @SerialName("actor_count") val actorCount: Int = 1,
    @SerialName("entity_type") val entityType: String = "",
    @SerialName("entity_id") val entityId: String = "",
    val read: Boolean = false,
    @SerialName("created_at") val createdAt: String? = null,
    @SerialName("last_activity_at") val lastActivityAt: String? = null,
)

@Serializable
data class ActorDto(
    val id: String,
    val username: String? = null,
    @SerialName("display_name") val displayName: String? = null,
    @SerialName("avatar_url") val avatarUrl: String? = null,
)

@Serializable
data class UnreadCount(val count: Int = 0)

@Serializable
data class ReadReceipt(val id: String = "", @SerialName("read_at") val readAt: String? = null)

@Serializable
data class ReadAll(val updated: Int = 0)

/** Tout est activé par défaut ; `orders: false` coupe seulement le push des commandes. */
@Serializable
data class PreferencesDto(
    @SerialName("push_enabled") val pushEnabled: Boolean = true,
    val likes: Boolean = true,
    val comments: Boolean = true,
    val follows: Boolean = true,
    @SerialName("video_ready") val videoReady: Boolean = true,
    val orders: Boolean = true,
)
