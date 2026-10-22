package com.poro.app.data.moderation

import com.poro.app.core.network.ApiEnvelope
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.POST

/** Signalements (docs/api/moderation.openapi.yaml) ; la file de modération n'est pas pour l'app. */
interface ModerationApi {
    /** 201 pour un nouveau signalement, 200 si ce compte avait déjà signalé la cible. */
    @POST("api/v1/moderation/reports")
    suspend fun report(@Body body: ReportRequest): Response<ApiEnvelope<ReportDto>>
}

/** Ce que l'on peut signaler : produits et boutiques ne sont pas pris en charge par l'API. */
enum class ReportTargetType(val wire: String) {
    Video("video"),
    Comment("comment"),
    User("user"),
}

/** Motifs du contrat, dans l'ordre affiché ; aucune autre valeur n'est acceptée. */
enum class ReportReason(val wire: String) {
    Spam("spam"),
    Nudity("nudity"),
    Violence("violence"),
    Harassment("harassment"),
    Hate("hate"),
    Fraud("fraud"),
    Other("other"),
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
    @SerialName("report_id") val reportId: String,
    @SerialName("case_id") val caseId: String,
    /** false : la même cible avait déjà été signalée par ce compte. */
    val created: Boolean,
)
