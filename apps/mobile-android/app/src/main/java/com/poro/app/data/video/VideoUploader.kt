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
 * Mémoire de l'envoi en cours (une seule publication à la fois). Elle permet de reprendre le même
 * envoi après une coupure ou un arrêt du processus (le serveur liste les parties reçues et signe
 * de nouvelles URL), de ne jamais publier deux fois, et d'annuler un envoi remplacé par un autre.
 */
interface UploadSessionStore {
    fun load(): UploadSession?
    fun save(session: UploadSession)
    fun clear()
}

/**
 * Envoi d'une vidéo directement vers le stockage (docs/api/ANDROID-INTEGRATION.md, « Envoi de
 * vidéos ») : initialisation, une URL présignée par partie de 8 Mio, assemblage. Une partie qui
 * échoue est renvoyée seule (jamais tout le fichier). Une coupure réseau garde l'envoi pour le
 * reprendre là où il s'est arrêté ; un refus, une annulation ou un média illisible l'annule.
 */
class VideoUploader(
    private val api: VideoApi,
    private val uploader: PresignedUploader,
    private val media: LocalMediaReader,
    private val retryDelaysMs: List<Long> = listOf(2_000, 5_000, 10_000),
    private val sessions: UploadSessionStore? = null,
) : VideoUploadClient {

    /** Ce qu'il reste à faire pour une vidéo : parties reçues et parties à envoyer. */
    private class Plan(val videoId: String, val partSize: Long, val done: List<CompletedPart>, val toSend: List<UploadPart>)

    /** Issue du règlement d'un envoi laissé en suspens. */
    private sealed interface Settled {
        /** Même envoi, déjà terminé : la vidéo publiée. */
        data class Published(val video: ApiResult.Success<VideoDto>) : Settled
        /** Même envoi, encore ouvert : on le reprend. */
        class Resume(val plan: Plan) : Settled
        /** Rien à reprendre : on recommence. */
        data object Fresh : Settled
        /** Le serveur est injoignable : on réessaiera, session gardée. */
        data class Unreachable(val error: ApiResult.NetworkError) : Settled
    }

    override suspend fun upload(
        file: LocalMedia,
        title: String,
        description: String?,
        onProgress: (sentBytes: Long, totalBytes: Long) -> Unit,
    ): ApiResult<VideoDto> {
        val fingerprint = uploadFingerprint(file, title, description)
        var plan: Plan? = null
        // Un envoi a pu rester en suspens (coupure, processus arrêté) : on le règle d'abord.
        sessions?.load()?.let { previous ->
            when (val settled = settle(previous, sameUpload = previous.fingerprint == fingerprint)) {
                is Settled.Published -> {
                    // « complete » avait déjà réussi pour ce même envoi : ne pas publier une seconde fois.
                    sessions.clear()
                    onProgress(file.sizeBytes, file.sizeBytes)
                    return settled.video
                }
                is Settled.Resume -> plan = settled.plan
                Settled.Fresh -> sessions.clear()
                is Settled.Unreachable -> return settled.error
            }
        }
        val current = plan ?: when (
            val init = apiCall {
                api.initUpload(InitUploadRequest(title.trim(), description?.trim()?.ifEmpty { null }, null, file.mimeType, file.sizeBytes))
            }
        ) {
            is ApiResult.Success -> Plan(init.data.videoId, init.data.partSize, emptyList(), init.data.parts).also {
                sessions?.save(UploadSession(it.videoId, fingerprint))
            }
            is ApiResult.Failure -> return init
            is ApiResult.NetworkError -> return init
        }
        var completed = false
        // Envoi à reprendre plus tard (coupure réseau), ou réponse à « complete » perdue : le serveur
        // a peut-être publié. On garde la trace pour que le prochain essai reprenne ou relise la
        // vidéo, au lieu d'annuler puis de publier une seconde fois.
        var keepSession = false
        try {
            val etags = current.done.toMutableList()
            var sent = current.done.sumOf { partLength(current.partSize, it.partNumber, file.sizeBytes).toLong() }
            if (sent > 0) onProgress(sent, file.sizeBytes)
            for (part in current.toSend.sortedBy { it.partNumber }) {
                val length = partLength(current.partSize, part.partNumber, file.sizeBytes)
                if (length <= 0) break
                val bytes = try {
                    media.read(file.uri, (part.partNumber - 1) * current.partSize, length)
                } catch (e: SecurityException) {
                    // Autorisation de lecture perdue (processus arrêté, accès non durable).
                    return mediaUnreadable()
                } catch (e: FileNotFoundException) {
                    // Fichier supprimé ou déplacé depuis sa sélection : réessayer n'y changera rien.
                    return mediaUnreadable()
                } catch (e: IOException) {
                    return ApiResult.NetworkError(e)
                }
                var result = putWithRetries(part.url, bytes)
                if (result is UploadResult.Rejected && result.httpStatus == 403) {
                    // Les URL expirent au bout d'une heure : un envoi lent en redemande une, une fois.
                    when (val fresh = freshUrl(current.videoId, part.partNumber)) {
                        is ApiResult.Success -> result = putWithRetries(fresh.data, bytes)
                        is ApiResult.NetworkError -> {
                            keepSession = true
                            return fresh
                        }
                        is ApiResult.Failure -> Unit
                    }
                }
                val etag = when (result) {
                    is UploadResult.Success -> result.etag ?: return uploadFailed()
                    is UploadResult.Rejected -> return uploadFailed(result.httpStatus)
                    is UploadResult.NetworkError -> {
                        keepSession = true
                        return ApiResult.NetworkError(result.cause)
                    }
                }
                etags += CompletedPart(part.partNumber, etag)
                sent += bytes.size
                onProgress(sent, file.sizeBytes)
            }
            keepSession = true
            val done = apiCall { api.completeUpload(current.videoId, CompleteUpload(etags.sortedBy { it.partNumber })) }
            completed = done is ApiResult.Success
            // 409 : la vidéo n'est plus `uploading`, un premier « complete » (rejoué par OkHttp
            // après une coupure) a pu aboutir ; 5xx et réseau : on ne sait pas.
            keepSession = done is ApiResult.NetworkError ||
                (done is ApiResult.Failure && (done.httpStatus >= 500 || done.httpStatus == 409))
            return done
        } finally {
            withContext(NonCancellable) {
                if (keepSession) {
                    // Session gardée, rien d'annulé : le prochain essai la règle (settle).
                } else {
                    // Échec, annulation ou refus : les parties déjà envoyées ne doivent pas rester en suspens.
                    if (!completed) apiCall(ifEmpty = {}) { api.abortUpload(current.videoId) }
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

    /** Nouvelle URL signée pour une partie, si l'envoi est toujours ouvert. */
    private suspend fun freshUrl(videoId: String, partNumber: Int): ApiResult<String> =
        when (val resumed = apiCall { api.resumeUpload(videoId) }) {
            is ApiResult.Success -> resumed.data.parts.firstOrNull { it.partNumber == partNumber }?.let { ApiResult.Success(it.url) }
                ?: uploadFailed(403)
            is ApiResult.Failure -> resumed
            is ApiResult.NetworkError -> resumed
        }

    /**
     * Règle un envoi laissé en suspens. Le même envoi déjà terminé est renvoyé tel quel ; encore
     * ouvert, il est repris là où le stockage l'a laissé. Un autre envoi encore ouvert est annulé
     * (le serveur refuse l'annulation, 409, d'une vidéo qui n'est plus `uploading`).
     */
    private suspend fun settle(previous: UploadSession, sameUpload: Boolean): Settled {
        when (val current = apiCall { api.video(previous.videoId) }) {
            is ApiResult.NetworkError -> return Settled.Unreachable(current)
            is ApiResult.Failure -> return Settled.Fresh
            is ApiResult.Success -> if (current.data.status != VideoDto.STATUS_UPLOADING) {
                return if (sameUpload) Settled.Published(current) else Settled.Fresh
            }
        }
        if (sameUpload) {
            when (val resumed = apiCall { api.resumeUpload(previous.videoId) }) {
                is ApiResult.Success -> return Settled.Resume(
                    Plan(previous.videoId, resumed.data.partSize, resumed.data.uploadedParts, resumed.data.parts),
                )
                is ApiResult.NetworkError -> return Settled.Unreachable(resumed)
                // Plus reprenable (expiré entre-temps) : on l'annule et on recommence.
                is ApiResult.Failure -> Unit
            }
        }
        return when (val aborted = apiCall(ifEmpty = {}) { api.abortUpload(previous.videoId) }) {
            is ApiResult.NetworkError -> Settled.Unreachable(aborted)
            else -> Settled.Fresh
        }
    }

    private fun mediaUnreadable(): ApiResult.Failure =
        ApiResult.Failure(0, ApiErrorCodes.MEDIA_UNREADABLE, "local media no longer readable", null, null)

    private fun uploadFailed(httpStatus: Int = 0): ApiResult.Failure =
        ApiResult.Failure(httpStatus, ApiErrorCodes.UPLOAD_REJECTED, "storage refused a part", null, null)
}

/** Taille de la partie [partNumber] : [partSize] octets, le reste du fichier pour la dernière. */
private fun partLength(partSize: Long, partNumber: Int, totalBytes: Long): Int =
    minOf(partSize, totalBytes - (partNumber - 1) * partSize).toInt()

/** Identifie « le même envoi » : même fichier, même taille, même titre et même description. */
internal fun uploadFingerprint(file: LocalMedia, title: String, description: String?): String =
    listOf(file.uri, file.sizeBytes.toString(), title.trim(), description?.trim().orEmpty()).joinToString("\u0000")
