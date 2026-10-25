package com.poro.app.data.video

import com.poro.app.core.media.LocalImagePreparer
import com.poro.app.core.media.PreparedImage
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PresignedUploader
import com.poro.app.core.network.UploadResult
import com.poro.app.core.network.apiCall
import java.io.IOException
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.delay
import kotlinx.coroutines.withContext

/** Envoi d'une publication photo ; interface pour tester le gestionnaire d'envoi sans réseau. */
fun interface PhotoUploadClient {
    /**
     * Envoie [images] dans l'ordre ; [onProgress] reçoit les octets confirmés par le stockage.
     * Renvoie la publication en `processing` (le traitement des images est asynchrone).
     */
    suspend fun upload(
        images: List<PreparedImage>,
        caption: String?,
        onProgress: (sentBytes: Long, totalBytes: Long) -> Unit,
    ): ApiResult<VideoDto>
}

/**
 * Publication photo (ADR-0020) : création avec la taille et le type de chaque image, un `PUT`
 * signé par image (avec exactement ses en-têtes signés), puis finalisation. Rien n'est publié
 * tant que la finalisation n'a pas réussi, et le serveur refuse toute publication partielle.
 * Comme pour une vidéo, une réponse de finalisation perdue est réglée en relisant la publication
 * au prochain essai, jamais en publiant une seconde fois.
 */
class PhotoUploader(
    private val api: VideoApi,
    private val uploader: PresignedUploader,
    private val files: LocalImagePreparer,
    private val retryDelaysMs: List<Long> = listOf(2_000, 5_000, 10_000),
    private val sessions: UploadSessionStore? = null,
) : PhotoUploadClient {
    override suspend fun upload(
        images: List<PreparedImage>,
        caption: String?,
        onProgress: (sentBytes: Long, totalBytes: Long) -> Unit,
    ): ApiResult<VideoDto> {
        val total = images.sumOf { it.sizeBytes }
        val fingerprint = photoFingerprint(images, caption)
        sessions?.load()?.let { previous ->
            when (val settled = settle(previous, sameUpload = previous.fingerprint == fingerprint)) {
                null -> sessions.clear()
                is ApiResult.Success -> {
                    sessions.clear()
                    onProgress(total, total)
                    return settled
                }
                is ApiResult.Failure -> return settled
                is ApiResult.NetworkError -> return settled
            }
        }
        val init = apiCall {
            api.initPhotoUpload(
                InitPhotoUploadRequest(images.map { PhotoSpec(it.contentType, it.sizeBytes) }, caption?.trim()?.ifEmpty { null }),
            )
        }
        val session = when (init) {
            is ApiResult.Success -> init.data
            is ApiResult.Failure -> return init
            is ApiResult.NetworkError -> return init
        }
        sessions?.save(UploadSession(session.videoId, fingerprint))
        var completed = false
        var outcomeUnknown = false
        try {
            var sent = 0L
            for (target in session.images.sortedBy { it.position }) {
                val image = images.getOrNull(target.position - 1) ?: return uploadFailed()
                val bytes = try {
                    files.read(image)
                } catch (e: IOException) {
                    return mediaUnreadable()
                }
                when (val result = putWithRetries(target, bytes)) {
                    is UploadResult.Success -> Unit
                    is UploadResult.Rejected -> return uploadFailed(result.httpStatus)
                    is UploadResult.NetworkError -> return ApiResult.NetworkError(result.cause)
                }
                sent += bytes.size
                onProgress(sent, total)
            }
            outcomeUnknown = true
            val done = apiCall { api.completePhotoUpload(session.videoId) }
            completed = done is ApiResult.Success
            // 409 : déjà finalisée (une première réponse perdue) ; 5xx et réseau : on ne sait pas.
            outcomeUnknown = done is ApiResult.NetworkError ||
                (done is ApiResult.Failure && (done.httpStatus >= 500 || done.httpStatus == 409))
            return done
        } finally {
            withContext(NonCancellable) {
                if (!outcomeUnknown) {
                    if (!completed) apiCall(ifEmpty = {}) { api.abortUpload(session.videoId) }
                    sessions?.clear()
                }
            }
        }
    }

    private suspend fun putWithRetries(target: PhotoTarget, bytes: ByteArray): UploadResult {
        var result = uploader.putSigned(target.url, target.headers, bytes)
        for (wait in retryDelaysMs) {
            if (result !is UploadResult.NetworkError && !(result is UploadResult.Rejected && result.httpStatus >= 500)) break
            delay(wait)
            result = uploader.putSigned(target.url, target.headers, bytes)
        }
        return result
    }

    /** Même règlement que pour une vidéo : publication déjà finalisée, échec réseau, ou rien. */
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
        ApiResult.Failure(0, ApiErrorCodes.MEDIA_UNREADABLE, "prepared image no longer readable", null, null)

    private fun uploadFailed(httpStatus: Int = 0): ApiResult.Failure =
        ApiResult.Failure(httpStatus, ApiErrorCodes.UPLOAD_REJECTED, "storage refused an image", null, null)
}

/** « Le même envoi » : mêmes fichiers préparés, dans le même ordre, avec la même légende. */
internal fun photoFingerprint(images: List<PreparedImage>, caption: String?): String =
    (images.map { it.path + ":" + it.sizeBytes } + caption?.trim().orEmpty()).joinToString("\u0000")
