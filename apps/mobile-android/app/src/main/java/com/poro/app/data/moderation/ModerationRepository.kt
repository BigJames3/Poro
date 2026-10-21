package com.poro.app.data.moderation

import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.apiCall

interface ModerationRepository {
    /** Signale une vidéo, un commentaire ou un profil. Le doublon n'est pas une erreur (`created` false). */
    suspend fun report(targetType: String, targetId: String, reason: String, comment: String?): ApiResult<ReportDto>
}

class DefaultModerationRepository(private val api: ModerationApi) : ModerationRepository {
    override suspend fun report(targetType: String, targetId: String, reason: String, comment: String?) =
        apiCall {
            api.report(
                ReportRequest(
                    targetType = targetType,
                    targetId = targetId,
                    reason = reason,
                    comment = comment?.trim()?.take(COMMENT_MAX)?.ifEmpty { null },
                ),
            )
        }

    private companion object {
        const val COMMENT_MAX = 500
    }
}
