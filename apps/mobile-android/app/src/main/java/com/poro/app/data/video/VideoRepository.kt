package com.poro.app.data.video

import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PageDto
import com.poro.app.core.network.apiCall

/** Lecture et suppression des vidéos ; l'envoi passe par [VideoUploader]. */
interface VideoRepository {
    suspend fun video(id: String): ApiResult<VideoDto>

    suspend fun mine(cursor: String?): ApiResult<PageDto<VideoDto>>

    suspend fun delete(id: String): ApiResult<Unit>
}

class DefaultVideoRepository(private val api: VideoApi) : VideoRepository {
    override suspend fun video(id: String): ApiResult<VideoDto> = apiCall { api.video(id) }

    override suspend fun mine(cursor: String?): ApiResult<PageDto<VideoDto>> = apiCall { api.mine(cursor, PAGE_SIZE) }

    override suspend fun delete(id: String): ApiResult<Unit> = apiCall(ifEmpty = {}) { api.delete(id) }

    private companion object {
        const val PAGE_SIZE = 20
    }
}
