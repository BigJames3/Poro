package com.poro.app.core.media

import java.io.IOException

/** Fichier choisi par l'utilisateur (sélecteur de photos ou de vidéos), désigné par son URI. */
data class LocalMedia(val uri: String, val mimeType: String, val sizeBytes: Long)

/**
 * Accès aux fichiers choisis par l'utilisateur. Interface pour les tests : les ViewModels ne
 * touchent jamais au ContentResolver. Une vidéo de 256 Mio est lue par morceaux, jamais en entier.
 */
interface LocalMediaReader {
    /** Type MIME et taille, ou null si le fichier est illisible ou de taille inconnue. */
    suspend fun describe(uri: String): LocalMedia?

    /** Lit au plus [length] octets à partir de [offset]. */
    @Throws(IOException::class)
    suspend fun read(uri: String, offset: Long, length: Int): ByteArray
}
