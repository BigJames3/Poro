package com.poro.app.data.video

import com.poro.app.core.media.LocalMedia
import com.poro.app.core.media.LocalMediaReader
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PresignedUploader
import com.poro.app.core.network.UploadResult
import com.poro.app.core.network.apiCall
import java.io.FileNotFoundException
import java.io.IOException
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.delay
import kotlinx.coroutines.withContext

/** Envoi d'une vidéo ; interface pour tester le gestionnaire d'envoi sans réseau. */
fun interface VideoUploadClient {
    /**
     * Envoie [file] ; [onProgress] reçoit les octets confirmés par le stockage. Renvoie la vidéo
     * en `processing` (le transcodage est asynchrone).
     */
    suspend fun upload(
        file: LocalMedia,
        title: String,
        description: String?,
        onProgress: (sentBytes: Long, totalBytes: Long) -> Unit,
    ): ApiResult<VideoDto>
}

/** Envoi en cours, mémorisé sur l'appareil : il survit à un arrêt du processus. */
data class UploadSession(val videoId: String, val fingerprint: String)

/**
 * Mémoire de l'envoi en cours (une seule publication à la fois). Le serveur n'offre aucune
 * reprise (ni relecture de session, ni nouvelles URL présignées) : cette trace sert seulement à
 * ne pas publier deux fois, et à annuler un envoi laissé en suspens par un processus arrêté.
 */
interface UploadSessionStore {
    fun load(): UploadSession?
    fun save(session: UploadSession)
    fun clear()
}

/**
 * Envoi d'une vidéo directement vers le stockage (docs/api/ANDROID-INTEGRATION.md, « Envoi de
 * vidéos ») : initialisation, une URL présignée par partie de 8 Mio, assemblage. Une partie qui
 * échoue est renvoyée seule (jamais tout le fichier) ; un envoi abandonné est annulé côté serveur.
 */
class VideoUploader(
    private val api: VideoApi,
    private val uploader: PresignedUploader,
    private val media: LocalMediaReader,
    private val retryDelaysMs: List<Long> = listOf(2_000, 5_000, 10_000),
    private val sessions: UploadSessionStore? = null,
) : VideoUploadClient {
    override suspend fun upload(
        file: LocalMedia,
        title: String,
        description: String?,
        onProgress: (sentBytes: Long, totalBytes: Long) -> Unit,
    ): ApiResult<VideoDto> {
        val fingerprint = uploadFingerprint(file, title, description)
        // Un processus arrêté a pu laisser un envoi en suspens : on le règle avant d'en créer un autre.
        sessions?.load()?.let { previous ->
            when (val settled = settle(previous, sameUpload = previous.fingerprint == fingerprint)) {
                null -> sessions.clear()
                is ApiResult.Success -> {
                    // « complete » avait déjà réussi pour ce même envoi : ne pas publier une seconde fois.
                    sessions.clear()
                    onProgress(file.sizeBytes, file.sizeBytes)
                    return settled
                }
                is ApiResult.Failure -> return settled
                is ApiResult.NetworkError -> return settled
            }
        }
        val init = apiCall {
            api.initUpload(InitUploadRequest(title.trim(), description?.trim()?.ifEmpty { null }, null, file.mimeType, file.sizeBytes))
        }
        val session = when (init) {
            is ApiResult.Success -> init.data
            is ApiResult.Failure -> return init
            is ApiResult.NetworkError -> return init
        }
        sessions?.save(UploadSession(session.videoId, fingerprint))
        var completed = false
        // Réponse à « complete » perdue : le serveur a peut-être publié. On garde la trace pour que
        // le prochain essai relise la vidéo au lieu d'annuler puis de publier une seconde fois.
        var outcomeUnknown = false
        try {
            val etags = ArrayList<CompletedPart>(session.parts.size)
            var sent = 0L
            for (part in session.parts.sortedBy { it.partNumber }) {
                val offset = (part.partNumber - 1) * session.partSize
                val length = minOf(session.partSize, file.sizeBytes - offset).toInt()
                if (length <= 0) break
                val bytes = try {
                    media.read(file.uri, offset, length)
                } catch (e: SecurityException) {
                    // Autorisation de lecture perdue (processus arrêté, accès non durable).
                    return mediaUnreadable()
                } catch (e: FileNotFoundException) {
                    // Fichier supprimé ou déplacé depuis sa sélection : réessayer n'y changera rien.
                    return mediaUnreadable()
                } catch (e: IOException) {
                    return ApiResult.NetworkError(e)
                }
                val etag = when (val result = putWithRetries(part.url, bytes)) {
                    is UploadResult.Success -> result.etag ?: return uploadFailed()
                    is UploadResult.Rejected -> return uploadFailed(result.httpStatus)
                    is UploadResult.NetworkError -> return ApiResult.NetworkError(result.cause)
                }
                etags += CompletedPart(part.partNumber, etag)
                sent += bytes.size
                onProgress(sent, file.sizeBytes)
            }
            outcomeUnknown = true
            val done = apiCall { api.completeUpload(session.videoId, CompleteUpload(etags)) }
            completed = done is ApiResult.Success
            // 409 : la vidéo n'est plus `uploading`, un premier « complete » (rejoué par OkHttp
            // après une coupure) a pu aboutir ; 5xx et réseau : on ne sait pas.
            outcomeUnknown = done is ApiResult.NetworkError ||
                (done is ApiResult.Failure && (done.httpStatus >= 500 || done.httpStatus == 409))
            return done
        } finally {
            withContext(NonCancellable) {
                if (outcomeUnknown) {
                    // Session gardée, rien d'annulé : le prochain essai la règle (settle).
                } else {
                    // Échec, annulation ou refus : les parties déjà envoyées ne doivent pas rester en suspens.
                    if (!completed) apiCall(ifEmpty = {}) { api.abortUpload(session.videoId) }
                    sessions?.clear()
                }
            }
        }
    }

    private suspend fun putWithRetries(url: String, bytes: ByteArray): UploadResult {
        var result = uploader.putPart(url, bytes)
        for (wait in retryDelaysMs) {
            // Un refus (URL expirée, signature) ne se corrige pas en réessayant ; le réseau, si.
            if (result !is UploadResult.NetworkError && !(result is UploadResult.Rejected && result.httpStatus >= 500)) break
            delay(wait)
            result = uploader.putPart(url, bytes)
        }
        return result
    }

    /**
     * Règle un envoi laissé par un processus arrêté. Résultat : la vidéo déjà publiée (même envoi,
     * « complete » réussi), un échec réseau (on réessaiera), ou null (rien à reprendre, on recommence).
     * L'annulation ne touche qu'un envoi encore `uploading` : le serveur la refuse (409) sinon.
     */
    private suspend fun settle(previous: UploadSession, sameUpload: Boolean): ApiResult<VideoDto>? {
        when (val current = apiCall { api.video(previous.videoId) }) {
            is ApiResult.NetworkError -> return current
            is ApiResult.Failure -> return null
            is ApiResult.Success -> if (current.data.status != VideoDto.STATUS_UPLOADING) {
                return if (sameUpload) current else null
            }
        }
        return when (val aborted = apiCall(ifEmpty = {}) { api.abortUpload(previous.videoId) }) {
            is ApiResult.NetworkError -> aborted
            else -> null
        }
    }

    private fun mediaUnreadable(): ApiResult.Failure =
        ApiResult.Failure(0, ApiErrorCodes.MEDIA_UNREADABLE, "local media no longer readable", null, null)

    private fun uploadFailed(httpStatus: Int = 0): ApiResult.Failure =
        ApiResult.Failure(httpStatus, ApiErrorCodes.UPLOAD_REJECTED, "storage refused a part", null, null)
}

/** Identifie « le même envoi » : même fichier, même taille, même titre et même description. */
internal fun uploadFingerprint(file: LocalMedia, title: String, description: String?): String =
    listOf(file.uri, file.sizeBytes.toString(), title.trim(), description?.trim().orEmpty()).joinToString("\u0000")
