package com.poro.app.data.notification

import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PageDto
import com.poro.app.core.network.apiCall

interface NotificationRepository {
    suspend fun page(cursor: String?): ApiResult<PageDto<NotificationDto>>

    suspend fun unreadCount(): ApiResult<Int>

    suspend fun markRead(id: String): ApiResult<ReadReceipt>

    suspend fun readAll(): ApiResult<ReadAll>

    suspend fun preferences(): ApiResult<PreferencesDto>

    suspend fun updatePreferences(preferences: PreferencesDto): ApiResult<PreferencesDto>
}

class DefaultNotificationRepository(private val api: NotificationApi) : NotificationRepository {
    override suspend fun page(cursor: String?) = apiCall { api.list(cursor, PAGE_SIZE) }

    override suspend fun unreadCount(): ApiResult<Int> = when (val result = apiCall { api.unreadCount() }) {
        is ApiResult.Success -> ApiResult.Success(result.data.count)
        is ApiResult.Failure -> result
        is ApiResult.NetworkError -> result
    }

    override suspend fun markRead(id: String) = apiCall { api.markRead(id) }

    override suspend fun readAll() = apiCall { api.readAll() }

    override suspend fun preferences() = apiCall { api.preferences() }

    override suspend fun updatePreferences(preferences: PreferencesDto) = apiCall { api.updatePreferences(preferences) }

    private companion object {
        const val PAGE_SIZE = 20
    }
}
