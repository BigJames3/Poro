package com.poro.app.data.feed

import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PageDto
import com.poro.app.core.network.apiCall

/** [Guest] : le fil « Pour toi » d'un invité, servi par les tendances publiques. */
enum class FeedKind { ForYou, Following, Trending, Guest }

/** Fils proposés à un compte connecté. */
val SIGNED_IN_FEEDS = listOf(FeedKind.ForYou, FeedKind.Following, FeedKind.Trending)

interface FeedRepository {
    /** Page suivante du fil [kind] ; [cursor] null pour la première. */
    suspend fun page(kind: FeedKind, cursor: String?): ApiResult<PageDto<FeedItem>>
}

class DefaultFeedRepository(private val api: FeedApi) : FeedRepository {
    override suspend fun page(kind: FeedKind, cursor: String?): ApiResult<PageDto<FeedItem>> = apiCall {
        when (kind) {
            FeedKind.ForYou -> api.forYou(cursor, PAGE_SIZE)
            FeedKind.Following -> api.following(cursor, PAGE_SIZE)
            FeedKind.Trending -> api.trending(cursor, PAGE_SIZE)
            FeedKind.Guest -> api.publicTrending(cursor, PAGE_SIZE)
        }
    }

    private companion object {
        const val PAGE_SIZE = 10
    }
}
