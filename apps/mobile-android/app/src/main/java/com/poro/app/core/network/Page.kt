package com.poro.app.core.network

import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable

/** Page d'une liste paginée par curseur opaque : `next_cursor` null à la dernière page. */
@Serializable
data class PageDto<T>(
    val items: List<T> = emptyList(),
    @SerialName("next_cursor") val nextCursor: String? = null,
)
