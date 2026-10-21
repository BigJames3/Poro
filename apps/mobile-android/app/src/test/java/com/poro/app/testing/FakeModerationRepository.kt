package com.poro.app.testing

import com.poro.app.core.network.ApiResult
import com.poro.app.data.moderation.ModerationRepository
import com.poro.app.data.moderation.ReportDto

/** Signalements en mémoire : chaque appel est noté dans [calls]. */
class FakeModerationRepository : ModerationRepository {
    val calls = mutableListOf<String>()
    var result: ApiResult<ReportDto> = ApiResult.Success(ReportDto("r1", "c1", created = true))

    override suspend fun report(targetType: String, targetId: String, reason: String, comment: String?): ApiResult<ReportDto> {
        calls += "report:$targetType:$targetId:$reason:${comment.orEmpty()}"
        return result
    }
}
