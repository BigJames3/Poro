package com.poro.app.core.media

import android.content.Context
import android.media.MediaMetadataRetriever
import android.net.Uri
import android.provider.OpenableColumns
import java.io.IOException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

class ContentResolverMediaReader(context: Context) : LocalMediaReader {
    private val appContext = context.applicationContext
    private val resolver = appContext.contentResolver

    override suspend fun describe(uri: String): LocalMedia? = withContext(Dispatchers.IO) {
        val parsed = Uri.parse(uri)
        val type = resolver.getType(parsed) ?: return@withContext null
        val size = resolver.query(parsed, arrayOf(OpenableColumns.SIZE), null, null, null)?.use { cursor ->
            if (cursor.moveToFirst() && !cursor.isNull(0)) cursor.getLong(0) else null
        } ?: return@withContext null
        LocalMedia(uri, type, size, if (type.startsWith("video/")) durationOf(parsed) else null)
    }

    /** Durée lue dans les métadonnées du conteneur ; null si le fichier ne la donne pas. */
    private fun durationOf(uri: Uri): Long? {
        val retriever = MediaMetadataRetriever()
        return try {
            retriever.setDataSource(appContext, uri)
            retriever.extractMetadata(MediaMetadataRetriever.METADATA_KEY_DURATION)?.toLongOrNull()
        } catch (e: RuntimeException) {
            null
        } finally {
            retriever.release()
        }
    }

    override suspend fun read(uri: String, offset: Long, length: Int): ByteArray = withContext(Dispatchers.IO) {
        val stream = resolver.openInputStream(Uri.parse(uri)) ?: throw IOException("unreadable media")
        stream.use { input ->
            var skipped = 0L
            while (skipped < offset) {
                val n = input.skip(offset - skipped)
                if (n <= 0) throw IOException("media shorter than expected")
                skipped += n
            }
            val buffer = ByteArray(length)
            var read = 0
            while (read < length) {
                val n = input.read(buffer, read, length - read)
                if (n < 0) break
                read += n
            }
            if (read == length) buffer else buffer.copyOf(read)
        }
    }
}
