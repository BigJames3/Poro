package com.poro.app.testing

import com.poro.app.core.media.LocalMedia
import com.poro.app.core.media.LocalMediaReader

/** Fichiers locaux en mémoire, indexés par URI. */
class FakeMediaReader : LocalMediaReader {
    val files = mutableMapOf<String, Pair<String, ByteArray>>()

    fun add(uri: String, mimeType: String, bytes: ByteArray) {
        files[uri] = mimeType to bytes
    }

    override suspend fun describe(uri: String): LocalMedia? =
        files[uri]?.let { (type, bytes) -> LocalMedia(uri, type, bytes.size.toLong()) }

    /** Autorisation de lecture perdue (comme après un arrêt du processus sans accès durable). */
    val revoked = mutableSetOf<String>()

    /** Fichier supprimé depuis sa sélection. */
    val deleted = mutableSetOf<String>()

    override suspend fun read(uri: String, offset: Long, length: Int): ByteArray {
        if (uri in revoked) throw SecurityException("permission revoked")
        if (uri in deleted) throw java.io.FileNotFoundException(uri)
        val bytes = files.getValue(uri).second
        return bytes.copyOfRange(offset.toInt(), minOf(bytes.size, offset.toInt() + length))
    }
}
