package com.poro.app.data.video

import com.poro.app.core.media.LocalMedia
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
 * Publication de vidéos, à l'échelle de l'application : on peut quitter l'écran pendant l'envoi.
 * Limite connue : l'envoi ne survit pas à l'arrêt du processus (pas encore de WorkManager).
 */
class UploadManager(
    private val client: VideoUploadClient,
    private val videos: VideoRepository,
    private val scope: CoroutineScope,
    private val pollDelaysMs: List<Long> = listOf(5_000L, 10_000L, 20_000L, 30_000L) + List(26) { 60_000L },
) {
    private val _state = MutableStateFlow<UploadState>(UploadState.Idle)
    val state: StateFlow<UploadState> = _state.asStateFlow()

    private var job: Job? = null

    val isBusy: Boolean get() = _state.value is UploadState.Uploading

    /** Démarre l'envoi ; refusé (false) si un autre envoi est en cours. */
    fun start(file: LocalMedia, title: String, description: String?): Boolean {
        if (isBusy) return false
        job?.cancel()
        _state.value = UploadState.Uploading(title, 0, file.sizeBytes)
        job = scope.launch {
            val result = client.upload(file, title, description) { sent, total ->
                _state.value = UploadState.Uploading(title, sent, total)
            }
            when (result) {
                is ApiResult.Success -> followProcessing(result.data)
                is ApiResult.Failure -> _state.value = UploadState.Failed(result)
                is ApiResult.NetworkError -> _state.value = UploadState.Failed(result)
            }
        }
        return true
    }

    /** Abandonne l'envoi en cours (les parties envoyées sont annulées côté serveur). */
    fun cancel() {
        job?.cancel()
        _state.value = UploadState.Idle
    }

    /** L'utilisateur a pris connaissance du résultat : retour à l'état initial. */
    fun dismiss() {
        if (!isBusy) {
            job?.cancel()
            _state.value = UploadState.Idle
        }
    }

    /** Interroge la vidéo avec un intervalle croissant jusqu'à `ready` ou `failed` (environ 30 min). */
    private suspend fun followProcessing(video: VideoDto) {
        _state.value = UploadState.Processing(video)
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
