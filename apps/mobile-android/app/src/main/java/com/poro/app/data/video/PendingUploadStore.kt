package com.poro.app.data.video

import android.content.Context
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import com.poro.app.core.network.PoroJson
import kotlinx.coroutines.flow.first
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString

private val Context.pendingUploadStore by preferencesDataStore(name = "poro_pending_upload")

@Serializable
data class PendingUpload(
    val uri: String,
    val title: String,
    val description: String? = null,
    @SerialName("mime_type") val mimeType: String,
    @SerialName("size_bytes") val sizeBytes: Long,
    @SerialName("copied_path") val copiedPath: String? = null,
)

/** Métadonnées de l'envoi en cours : survivent à l'arrêt du processus (pas les jetons). */
class PendingUploadStore(context: Context) {
    private val store = context.applicationContext.pendingUploadStore

    suspend fun load(): PendingUpload? {
        val raw = store.data.first()[KEY] ?: return null
        return runCatching { PoroJson.decodeFromString<PendingUpload>(raw) }.getOrNull()
    }

    suspend fun save(pending: PendingUpload) {
        store.edit { it[KEY] = PoroJson.encodeToString(pending) }
    }

    suspend fun clear() {
        store.edit { it.remove(KEY) }
    }

    private companion object {
        val KEY = stringPreferencesKey("pending")
    }
}
