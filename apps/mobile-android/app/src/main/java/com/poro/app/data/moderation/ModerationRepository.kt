package com.poro.app.data.moderation

import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.apiCall

interface ModerationRepository {
    /** [comment] vide ou blanc : non envoyé. */
    suspend fun report(type: ReportTargetType, targetId: String, reason: ReportReason, comment: String?): ApiResult<ReportDto>
}

class DefaultModerationRepository(private val api: ModerationApi) : ModerationRepository {
    override suspend fun report(type: ReportTargetType, targetId: String, reason: ReportReason, comment: String?) =
        apiCall { api.report(ReportRequest(type.wire, targetId, reason.wire, comment?.trim()?.ifEmpty { null })) }
}
