package com.poro.app.data.search

import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.apiCall

interface SearchRepository {
    suspend fun videos(q: String, cursor: String?): ApiResult<SearchPage<VideoHit>>

    suspend fun users(q: String, cursor: String?): ApiResult<SearchPage<UserHit>>

    suspend fun hashtags(q: String, cursor: String?): ApiResult<SearchPage<HashtagHit>>

    /** Saisie semi-automatique : 5 pseudos et 5 hashtags au plus ; « @ » et « # » ignorés. */
    suspend fun suggest(q: String): ApiResult<Suggestions>
}

class DefaultSearchRepository(private val api: SearchApi) : SearchRepository {
    override suspend fun videos(q: String, cursor: String?) = apiCall { api.videos(q, cursor, PAGE_SIZE) }

    override suspend fun users(q: String, cursor: String?) = apiCall { api.users(q, cursor, PAGE_SIZE) }

    override suspend fun hashtags(q: String, cursor: String?) = apiCall { api.hashtags(q, cursor, PAGE_SIZE) }

    override suspend fun suggest(q: String) = apiCall { api.suggest(q) }

    private companion object {
        const val PAGE_SIZE = 20
    }
}
