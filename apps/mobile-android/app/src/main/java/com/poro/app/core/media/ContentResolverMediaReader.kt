package com.poro.app.core.media

import android.content.Context
import android.net.Uri
import android.provider.OpenableColumns
import java.io.IOException
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

class ContentResolverMediaReader(context: Context) : LocalMediaReader {
    private val resolver = context.applicationContext.contentResolver

    override suspend fun describe(uri: String): LocalMedia? = withContext(Dispatchers.IO) {
        val parsed = Uri.parse(uri)
        val type = resolver.getType(parsed) ?: return@withContext null
        val size = resolver.query(parsed, arrayOf(OpenableColumns.SIZE), null, null, null)?.use { cursor ->
            if (cursor.moveToFirst() && !cursor.isNull(0)) cursor.getLong(0) else null
        } ?: return@withContext null
        LocalMedia(uri, type, size)
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
