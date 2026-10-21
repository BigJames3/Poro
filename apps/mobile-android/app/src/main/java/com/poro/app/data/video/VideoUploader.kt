package com.poro.app.data.video

import com.poro.app.core.media.LocalMedia
import com.poro.app.core.media.LocalMediaReader
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PresignedUploader
import com.poro.app.core.network.UploadResult
import com.poro.app.core.network.apiCall
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
) : VideoUploadClient {
    override suspend fun upload(
        file: LocalMedia,
        title: String,
        description: String?,
        onProgress: (sentBytes: Long, totalBytes: Long) -> Unit,
    ): ApiResult<VideoDto> {
        val init = apiCall {
            api.initUpload(InitUploadRequest(title.trim(), description?.trim()?.ifEmpty { null }, null, file.mimeType, file.sizeBytes))
        }
        val session = when (init) {
            is ApiResult.Success -> init.data
            is ApiResult.Failure -> return init
            is ApiResult.NetworkError -> return init
        }
        var completed = false
        try {
            val etags = ArrayList<CompletedPart>(session.parts.size)
            var sent = 0L
            for (part in session.parts.sortedBy { it.partNumber }) {
                val offset = (part.partNumber - 1) * session.partSize
                val length = minOf(session.partSize, file.sizeBytes - offset).toInt()
                if (length <= 0) break
                val bytes = try {
                    media.read(file.uri, offset, length)
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
            val done = apiCall { api.completeUpload(session.videoId, CompleteUpload(etags)) }
            completed = done is ApiResult.Success
            return done
        } finally {
            // Échec, annulation ou refus : les parties déjà envoyées ne doivent pas rester en suspens.
            if (!completed) {
                withContext(NonCancellable) { apiCall(ifEmpty = {}) { api.abortUpload(session.videoId) } }
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

    private fun uploadFailed(httpStatus: Int = 0): ApiResult.Failure =
        ApiResult.Failure(httpStatus, ApiErrorCodes.UPLOAD_REJECTED, "storage refused a part", null, null)
}
