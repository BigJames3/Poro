package com.poro.app.core.network

import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json

/**
 * Enveloppe de toutes les réponses de l'API PORO : `{ "data": …, "error": …, "meta": … }`.
 * Voir docs/api/ANDROID-INTEGRATION.md, « Format des réponses ».
 */
@Serializable
data class ApiEnvelope<T>(
    val data: T? = null,
    val error: ApiErrorBody? = null,
    val meta: ApiMeta? = null,
)

@Serializable
data class ApiErrorBody(
    /** Énumération stable du contrat : l'interface se branche dessus, jamais sur [message]. */
    val code: String,
    /** Texte libre, en anglais, destiné aux logs : ne jamais l'afficher tel quel. */
    val message: String = "",
)

@Serializable
data class ApiMeta(
    @kotlinx.serialization.SerialName("request_id") val requestId: String? = null,
)

/** Configuration JSON partagée : tolérante aux champs ajoutés plus tard par le backend. */
val PoroJson: Json = Json {
    ignoreUnknownKeys = true
    explicitNulls = false
    coerceInputValues = true
}
