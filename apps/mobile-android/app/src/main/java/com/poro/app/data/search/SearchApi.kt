package com.poro.app.data.search

import com.poro.app.core.network.ApiEnvelope
import com.poro.app.data.feed.ItemStats
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import retrofit2.Response
import retrofit2.http.GET
import retrofit2.http.Query

/**
 * Recherche de vidéos, comptes et hashtags (docs/api/search.openapi.yaml). La même route renvoie
 * des éléments différents selon `type` : une méthode typée par sorte de résultat.
 */
interface SearchApi {
    @GET("api/v1/search?type=videos")
    suspend fun videos(
        @Query("q") q: String,
        @Query("cursor") cursor: String?,
        @Query("limit") limit: Int,
    ): Response<ApiEnvelope<SearchPage<VideoHit>>>

    @GET("api/v1/search?type=users")
    suspend fun users(
        @Query("q") q: String,
        @Query("cursor") cursor: String?,
        @Query("limit") limit: Int,
    ): Response<ApiEnvelope<SearchPage<UserHit>>>

    @GET("api/v1/search?type=hashtags")
    suspend fun hashtags(
        @Query("q") q: String,
        @Query("cursor") cursor: String?,
        @Query("limit") limit: Int,
    ): Response<ApiEnvelope<SearchPage<HashtagHit>>>

    @GET("api/v1/search/suggest")
    suspend fun suggest(@Query("q") q: String): Response<ApiEnvelope<Suggestions>>
}

@Serializable
data class SearchPage<T>(
    val items: List<T> = emptyList(),
    @SerialName("next_cursor") val nextCursor: String? = null,
)

@Serializable
data class VideoHit(
    @SerialName("video_id") val videoId: String,
    val title: String = "",
    val hashtags: List<String> = emptyList(),
    @SerialName("thumbnail_url") val thumbnailUrl: String? = null,
    @SerialName("duration_ms") val durationMs: Long? = null,
    @SerialName("published_at") val publishedAt: String? = null,
    val stats: ItemStats = ItemStats(),
    val author: HitAuthor = HitAuthor(),
)

@Serializable
data class HitAuthor(
    @SerialName("user_id") val userId: String = "",
    val username: String? = null,
    @SerialName("display_name") val displayName: String? = null,
    @SerialName("avatar_url") val avatarUrl: String? = null,
)

@Serializable
data class UserHit(
    @SerialName("user_id") val userId: String,
    val username: String = "",
    @SerialName("display_name") val displayName: String? = null,
    @SerialName("avatar_url") val avatarUrl: String? = null,
    @SerialName("is_creator") val isCreator: Boolean = false,
    val followers: Long = 0,
)

@Serializable
data class HashtagHit(val tag: String, @SerialName("videos_count") val videosCount: Long = 0)

@Serializable
data class Suggestions(val users: List<UserHit> = emptyList(), val hashtags: List<HashtagHit> = emptyList())
