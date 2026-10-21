package com.poro.app.data.moderation

import com.poro.app.core.network.ApiEnvelope
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.POST

/**
 * Signalements utilisateur (docs/api/moderation.openapi.yaml). La file staff n'est pas exposée
 * dans l'application : seuls les comptes authentifiés envoient un rapport.
 */
interface ModerationApi {
    @POST("api/v1/moderation/reports")
    suspend fun report(@Body body: ReportRequest): Response<ApiEnvelope<ReportDto>>
}

@Serializable
data class ReportRequest(
    @SerialName("target_type") val targetType: String,
    @SerialName("target_id") val targetId: String,
    val reason: String,
    val comment: String? = null,
)

@Serializable
data class ReportDto(
    @SerialName("report_id") val reportId: String = "",
    @SerialName("case_id") val caseId: String = "",
    val created: Boolean = true,
)

object ReportTargets {
    const val VIDEO = "video"
    const val COMMENT = "comment"
    const val USER = "user"
}
