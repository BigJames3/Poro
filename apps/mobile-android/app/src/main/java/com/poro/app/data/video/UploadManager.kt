package com.poro.app.data.video

import com.poro.app.core.media.LocalMedia
import com.poro.app.core.media.PreparedImage
import com.poro.app.core.network.ApiResult
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

/** Où en est la publication en cours ; une seule à la fois. */
sealed interface UploadState {
    data object Idle : UploadState

    data class Uploading(val title: String, val sentBytes: Long, val totalBytes: Long) : UploadState {
        val progress: Float get() = if (totalBytes == 0L) 0f else sentBytes.toFloat() / totalBytes
    }

    /** Envoyée ; le serveur la transcode (quelques minutes). */
    data class Processing(val video: VideoDto) : UploadState

    /** Traitement terminé : `ready`, ou `failed` (trop longue, illisible…). */
    data class Finished(val video: VideoDto) : UploadState

    /** L'envoi n'a pas abouti ; [failure] porte le code d'erreur à afficher. */
    data class Failed(val failure: ApiResult<Nothing>) : UploadState
}

/**
 * Publication de vidéos, à l'échelle de l'application : on peut quitter l'écran, et avec
 * WorkManager fermer l'app, pendant l'envoi. L'état est déduit du [scheduler], relu au redémarrage ;
 * le suivi du transcodage, lui, reprend à l'ouverture de l'app.
 */
class UploadManager(
    private val scheduler: UploadScheduler,
    private val videos: VideoRepository,
    private val scope: CoroutineScope,
    private val pollDelaysMs: List<Long> = listOf(5_000L, 10_000L, 20_000L, 30_000L) + List(26) { 60_000L },
) {
    private val _state = MutableStateFlow<UploadState>(UploadState.Idle)
    val state: StateFlow<UploadState> = _state.asStateFlow()

    private var processing: Job? = null
    private var followedVideoId: String? = null

    /** Résultat affiché en dernier, et celui que l'utilisateur a fermé (à ne pas réafficher). */
    private var shownUploadId: String? = null
    private var dismissedUploadId: String? = null

    val isBusy: Boolean get() = _state.value is UploadState.Uploading

    init {
        scope.launch { scheduler.upload.collect(::onScheduled) }
    }

    /** Démarre l'envoi ; refusé (false) si un autre envoi est en cours. */
    fun start(file: LocalMedia, title: String, description: String?): Boolean {
        if (isBusy) return false
        stopFollowing()
        _state.value = UploadState.Uploading(title, 0, file.sizeBytes)
        scheduler.enqueue(file, title, description)
        return true
    }

    /**
     * Démarre une publication photo ([images] préparées, dans l'ordre) ; refusé (false) si un
     * autre envoi est en cours. [label] s'affiche pendant l'envoi (légende ou « Photo »).
     */
    fun startPhotos(images: List<PreparedImage>, caption: String?, label: String): Boolean {
        if (isBusy) return false
        stopFollowing()
        _state.value = UploadState.Uploading(label, 0, images.sumOf { it.sizeBytes })
        scheduler.enqueuePhotos(images, caption, label)
        return true
    }

    /** Abandonne l'envoi en cours (les parties envoyées sont annulées côté serveur). */
    fun cancel() {
        stopFollowing()
        scheduler.cancel()
        _state.value = UploadState.Idle
    }

    /** L'utilisateur a pris connaissance du résultat : retour à l'état initial. */
    fun dismiss() {
        if (isBusy) return
        stopFollowing()
        // WorkManager peut réémettre l'envoi terminé avant de l'oublier : on l'ignorera.
        dismissedUploadId = shownUploadId
        scheduler.clear()
        _state.value = UploadState.Idle
    }

    private fun onScheduled(upload: ScheduledUpload) {
        val uploadId = when (upload) {
            is ScheduledUpload.Succeeded -> upload.uploadId
            is ScheduledUpload.Failed -> upload.uploadId
            else -> null
        }
        if (uploadId != null && uploadId == dismissedUploadId) return
        shownUploadId = uploadId
        when (upload) {
            // Rien de planifié : on garde un résultat affiché (Processing, Finished) jusqu'à sa fermeture.
            ScheduledUpload.None -> if (_state.value is UploadState.Uploading) _state.value = UploadState.Idle
            is ScheduledUpload.Running -> {
                stopFollowing()
                _state.value = UploadState.Uploading(upload.title, upload.sentBytes, upload.totalBytes)
            }
            is ScheduledUpload.Failed -> {
                stopFollowing()
                _state.value = UploadState.Failed(upload.failure)
            }
            is ScheduledUpload.Succeeded -> if (followedVideoId != upload.videoId) {
                followedVideoId = upload.videoId
                val video = VideoDto(id = upload.videoId, title = upload.title, status = VideoDto.STATUS_PROCESSING,
                    mediaType = if (upload.photos) VideoDto.MEDIA_PHOTO else VideoDto.MEDIA_VIDEO)
                _state.value = UploadState.Processing(video)
                processing = scope.launch { followProcessing(video) }
            }
        }
    }

    private fun stopFollowing() {
        processing?.cancel()
        processing = null
        followedVideoId = null
    }

    /** Interroge la vidéo avec un intervalle croissant jusqu'à `ready` ou `failed` (environ 30 min). */
    private suspend fun followProcessing(video: VideoDto) {
        for (wait in pollDelaysMs) {
            delay(wait)
            val current = (videos.video(video.id) as? ApiResult.Success)?.data ?: continue
            if (current.status == VideoDto.STATUS_READY || current.status == VideoDto.STATUS_FAILED) {
                _state.value = UploadState.Finished(current)
                return
            }
        }
        // Toujours en traitement : la notification « vidéo prête » prendra le relais.
    }
}
