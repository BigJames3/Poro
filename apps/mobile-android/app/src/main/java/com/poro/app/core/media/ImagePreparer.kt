package com.poro.app.core.media

import java.io.IOException

/**
 * Une image prête à envoyer : JPEG réduit, écrit dans un fichier privé de l'application et gardé
 * jusqu'à ce que le serveur confirme la publication (ou qu'elle échoue pour de bon).
 */
data class PreparedImage(val path: String, val sizeBytes: Long, val contentType: String = JPEG) {
    companion object {
        const val JPEG = "image/jpeg"
    }
}

/** Résultat de la préparation d'une publication photo, image par image dans l'ordre choisi. */
sealed interface PrepareResult {
    data class Ready(val images: List<PreparedImage>) : PrepareResult

    /** L'image [position] (à partir de 1) ne peut pas être envoyée ; rien n'est gardé. */
    data class Failed(val position: Int, val reason: Reason) : PrepareResult

    enum class Reason {
        /** Format que l'appareil ne sait pas décoder (HEIC sur un Android ancien, fichier abîmé). */
        Unsupported,

        /** Fichier supprimé ou autorisation perdue depuis la sélection. */
        Unreadable,
    }
}

/**
 * Prépare les photos choisies (ADR-0020, décision du 2026-10-24) : décodage, orientation EXIF
 * appliquée, côté long réduit à [MAX_SIDE] px, JPEG qualité [QUALITY]. Le serveur refait tous les
 * contrôles ; réduire ici épargne le forfait de données. HEIC n'est accepté que si l'appareil sait
 * le décoder, et il part alors en JPEG. Interface pour les tests : aucun ViewModel ne touche aux
 * fichiers.
 */
interface LocalImagePreparer {
    suspend fun prepare(uris: List<String>): PrepareResult

    @Throws(IOException::class)
    fun read(image: PreparedImage): ByteArray

    /** Supprime les fichiers d'une publication terminée (publiée ou abandonnée). */
    fun discard(images: List<PreparedImage>)

    companion object {
        const val MAX_SIDE = 2048
        const val QUALITY = 92

        /** Limite du serveur par image, après préparation. */
        const val MAX_BYTES = 10L * 1024 * 1024
    }
}
