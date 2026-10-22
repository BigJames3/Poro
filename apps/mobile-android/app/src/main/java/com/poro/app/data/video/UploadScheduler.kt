package com.poro.app.data.video

import com.poro.app.core.media.LocalMedia
import com.poro.app.core.network.ApiResult
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

/** L'envoi en cours tel que le planificateur le voit : il peut avoir survécu à un arrêt de l'app. */
sealed interface ScheduledUpload {
    data object None : ScheduledUpload

    /** En attente (réseau) ou en cours ; [totalBytes] 0 tant que l'envoi n'a pas commencé. */
    data class Running(val title: String, val sentBytes: Long, val totalBytes: Long) : ScheduledUpload

    /** Envoyée : la vidéo [videoId] est en traitement côté serveur. [uploadId] identifie l'envoi. */
    data class Succeeded(val uploadId: String, val videoId: String, val title: String) : ScheduledUpload

    data class Failed(val uploadId: String, val failure: ApiResult<Nothing>) : ScheduledUpload
}

/**
 * Exécute l'envoi d'une vidéo, une seule à la fois. En production, WorkManager : l'envoi continue
 * après la fermeture de l'app ou l'arrêt du processus ([WorkManagerUploadScheduler]).
 */
interface UploadScheduler {
    /** Dernier état connu ; relu au redémarrage de l'app. */
    val upload: Flow<ScheduledUpload>

    fun enqueue(file: LocalMedia, title: String, description: String?)

    /** Abandon : les parties déjà envoyées sont annulées côté serveur. */
    fun cancel()

    /** L'utilisateur a pris connaissance du résultat : on l'oublie. */
    fun clear()
}

/** Envoi dans le processus de l'app (tests, et repli) : perdu si le processus s'arrête. */
class InProcessUploadScheduler(
    private val client: VideoUploadClient,
    private val scope: CoroutineScope,
) : UploadScheduler {
    private val state = MutableStateFlow<ScheduledUpload>(ScheduledUpload.None)
    override val upload: Flow<ScheduledUpload> = state.asStateFlow()

    private var job: Job? = null
    private var count = 0

    override fun enqueue(file: LocalMedia, title: String, description: String?) {
        job?.cancel()
        val uploadId = (++count).toString()
        state.value = ScheduledUpload.Running(title, 0, file.sizeBytes)
        job = scope.launch {
            val result = client.upload(file, title, description) { sent, total ->
                state.value = ScheduledUpload.Running(title, sent, total)
            }
            state.value = when (result) {
                is ApiResult.Success -> ScheduledUpload.Succeeded(uploadId, result.data.id, title)
                is ApiResult.Failure -> ScheduledUpload.Failed(uploadId, result)
                is ApiResult.NetworkError -> ScheduledUpload.Failed(uploadId, result)
            }
        }
    }

    override fun cancel() {
        job?.cancel()
        state.value = ScheduledUpload.None
    }

    override fun clear() {
        if (state.value !is ScheduledUpload.Running) state.value = ScheduledUpload.None
    }
}
