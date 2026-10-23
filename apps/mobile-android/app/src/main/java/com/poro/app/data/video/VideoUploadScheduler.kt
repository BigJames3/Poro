package com.poro.app.data.video

import android.content.Context
import androidx.work.Constraints
import androidx.work.ExistingWorkPolicy
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.OutOfQuotaPolicy
import androidx.work.WorkManager
import com.poro.app.core.media.LocalMedia
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch

/** Enfile un envoi unique ; l'exécution réelle est dans [VideoUploadWorker]. */
interface UploadDurability {
    fun enqueue(file: LocalMedia, title: String, description: String?)
    fun cancel()
}

class VideoUploadScheduler(
    context: Context,
    private val store: PendingUploadStore,
    private val scope: CoroutineScope,
) : UploadDurability {
    private val appContext = context.applicationContext
    private var enqueueJob: Job? = null

    override fun enqueue(file: LocalMedia, title: String, description: String?) {
        enqueueJob?.cancel()
        enqueueJob = scope.launch {
            store.save(PendingUpload(file.uri, title, description, file.mimeType, file.sizeBytes))
            val request = OneTimeWorkRequestBuilder<VideoUploadWorker>()
                .setExpedited(OutOfQuotaPolicy.RUN_AS_NON_EXPEDITED_WORK_REQUEST)
                .setConstraints(Constraints.Builder().setRequiredNetworkType(NetworkType.CONNECTED).build())
                .build()
            WorkManager.getInstance(appContext).enqueueUniqueWork(UNIQUE_NAME, ExistingWorkPolicy.REPLACE, request)
        }
    }

    override fun cancel() {
        enqueueJob?.cancel()
        WorkManager.getInstance(appContext).cancelUniqueWork(UNIQUE_NAME)
        scope.launch { store.clear() }
    }

    companion object {
        const val UNIQUE_NAME = "poro-video-upload"
    }
}
