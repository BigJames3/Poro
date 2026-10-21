package com.poro.app.data.feed

import com.poro.app.core.network.ApiEnvelope
import com.poro.app.core.network.PageDto
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import retrofit2.Response
import retrofit2.http.GET
import retrofit2.http.Query

/** Fils de vidéos prêtes (docs/api/feed.openapi.yaml) ; 30 éléments par page au plus. */
interface FeedApi {
    @GET("api/v1/feed/for-you")
    suspend fun forYou(@Query("cursor") cursor: String?, @Query("limit") limit: Int): Response<ApiEnvelope<PageDto<FeedItem>>>

    @GET("api/v1/feed/following")
    suspend fun following(@Query("cursor") cursor: String?, @Query("limit") limit: Int): Response<ApiEnvelope<PageDto<FeedItem>>>

    @GET("api/v1/feed/trending")
    suspend fun trending(@Query("cursor") cursor: String?, @Query("limit") limit: Int): Response<ApiEnvelope<PageDto<FeedItem>>>
}

/** Élément compact (réseaux lents) : pas de description ni de `liked_by_me`, lus ailleurs. */
@Serializable
data class FeedItem(
    @SerialName("video_id") val videoId: String,
    val title: String = "",
    val hashtags: List<String> = emptyList(),
    @SerialName("duration_ms") val durationMs: Long = 0,
    @SerialName("published_at") val publishedAt: String? = null,
    @SerialName("thumbnail_url") val thumbnailUrl: String? = null,
    @SerialName("hls_url") val hlsUrl: String? = null,
    val stats: ItemStats = ItemStats(),
    val author: Author = Author(),
)

@Serializable
data class ItemStats(val likes: Long = 0, val comments: Long = 0, val shares: Long = 0)

/** Champs de profil null tant que le feed n'a pas reçu le premier événement de profil de l'auteur. */
@Serializable
data class Author(
    val id: String = "",
    val username: String? = null,
    @SerialName("display_name") val displayName: String? = null,
    @SerialName("avatar_url") val avatarUrl: String? = null,
)
