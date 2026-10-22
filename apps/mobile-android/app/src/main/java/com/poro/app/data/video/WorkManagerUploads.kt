package com.poro.app.data.video

import android.app.Notification
import android.content.Context
import android.content.Intent
import android.content.pm.ServiceInfo
import android.net.Uri
import android.os.Build
import androidx.core.app.NotificationChannelCompat
import androidx.core.app.NotificationCompat
import androidx.core.app.NotificationManagerCompat
import androidx.work.Constraints
import androidx.work.CoroutineWorker
import androidx.work.Data
import androidx.work.ExistingWorkPolicy
import androidx.work.ForegroundInfo
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.WorkInfo
import androidx.work.WorkManager
import androidx.work.WorkerParameters
import androidx.work.workDataOf
import com.poro.app.PoroApp
import com.poro.app.R
import com.poro.app.core.media.LocalMedia
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import java.io.IOException
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map

/**
 * Envoi par WorkManager : il continue quand l'app est fermée ou son processus arrêté, en service
 * de premier plan (notification de progression), et reprend quand le réseau revient.
 */
class WorkManagerUploadScheduler(private val context: Context) : UploadScheduler {
    private val workManager = WorkManager.getInstance(context)

    override val upload: Flow<ScheduledUpload> =
        workManager.getWorkInfosForUniqueWorkFlow(UNIQUE_NAME).map { infos -> infos.lastOrNull()?.toScheduled() ?: ScheduledUpload.None }

    override fun enqueue(file: LocalMedia, title: String, description: String?) {
        // Sans autorisation durable, le fichier choisi ne serait plus lisible après un arrêt du processus.
        try {
            context.contentResolver.takePersistableUriPermission(Uri.parse(file.uri), Intent.FLAG_GRANT_READ_URI_PERMISSION)
        } catch (e: SecurityException) {
            // Le sélecteur n'accorde pas d'accès durable : l'envoi marche tant que le processus vit.
        }
        val request = OneTimeWorkRequestBuilder<VideoUploadWorker>()
            .setInputData(
                workDataOf(
                    KEY_URI to file.uri,
                    KEY_MIME to file.mimeType,
                    KEY_SIZE to file.sizeBytes,
                    KEY_TITLE to title,
                    KEY_DESCRIPTION to description,
                ),
            )
            .setConstraints(Constraints.Builder().setRequiredNetworkType(NetworkType.CONNECTED).build())
            .addTag(TITLE_TAG_PREFIX + title)
            .build()
        workManager.enqueueUniqueWork(UNIQUE_NAME, ExistingWorkPolicy.REPLACE, request)
    }

    override fun cancel() {
        workManager.cancelUniqueWork(UNIQUE_NAME)
    }

    override fun clear() {
        workManager.pruneWork()
    }

    private fun WorkInfo.toScheduled(): ScheduledUpload {
        val title = outputData.getString(KEY_TITLE) ?: progress.getString(KEY_TITLE)
            ?: tags.firstOrNull { it.startsWith(TITLE_TAG_PREFIX) }?.removePrefix(TITLE_TAG_PREFIX).orEmpty()
        return when (state) {
            WorkInfo.State.ENQUEUED, WorkInfo.State.RUNNING, WorkInfo.State.BLOCKED ->
                ScheduledUpload.Running(title, progress.getLong(KEY_SENT, 0), progress.getLong(KEY_TOTAL, 0))
            WorkInfo.State.SUCCEEDED -> outputData.getString(KEY_VIDEO_ID)
                ?.let { ScheduledUpload.Succeeded(id.toString(), it, title) }
                ?: ScheduledUpload.Failed(id.toString(), unexpected())
            WorkInfo.State.FAILED -> ScheduledUpload.Failed(id.toString(), outputData.toFailure())
            WorkInfo.State.CANCELLED -> ScheduledUpload.None
        }
    }

    internal companion object {
        const val UNIQUE_NAME = "video-upload"
        const val TITLE_TAG_PREFIX = "upload-title:"
        const val KEY_URI = "uri"
        const val KEY_MIME = "mime"
        const val KEY_SIZE = "size"
        const val KEY_TITLE = "title"
        const val KEY_DESCRIPTION = "description"
        const val KEY_SENT = "sent"
        const val KEY_TOTAL = "total"
        const val KEY_VIDEO_ID = "video_id"
        const val KEY_STATUS = "http_status"
        const val KEY_CODE = "code"
        const val KEY_MESSAGE = "message"
        const val KEY_REQUEST_ID = "request_id"
        const val KEY_NETWORK = "network"
    }
}

/** Échec relu depuis la sortie du worker ; un plantage du worker n'en laisse aucune. */
private fun Data.toFailure(): ApiResult<Nothing> {
    val code = getString(WorkManagerUploadScheduler.KEY_CODE)
    return when {
        getBoolean(WorkManagerUploadScheduler.KEY_NETWORK, false) -> ApiResult.NetworkError(IOException("upload interrupted"))
        code != null -> ApiResult.Failure(
            getInt(WorkManagerUploadScheduler.KEY_STATUS, 0),
            code,
            getString(WorkManagerUploadScheduler.KEY_MESSAGE).orEmpty(),
            getString(WorkManagerUploadScheduler.KEY_REQUEST_ID),
            null,
        )
        else -> unexpected()
    }
}

private fun unexpected(): ApiResult<Nothing> = ApiResult.Failure(0, ApiErrorCodes.UPLOAD_REJECTED, "upload worker failed", null, null)

class VideoUploadWorker(context: Context, params: WorkerParameters) : CoroutineWorker(context, params) {

    override suspend fun doWork(): Result {
        val keys = WorkManagerUploadScheduler
        val uri = inputData.getString(keys.KEY_URI) ?: return Result.failure()
        val title = inputData.getString(keys.KEY_TITLE).orEmpty()
        val file = LocalMedia(uri, inputData.getString(keys.KEY_MIME).orEmpty(), inputData.getLong(keys.KEY_SIZE, 0))
        val client = (applicationContext as PoroApp).container.videoUploadClient

        try {
            setForeground(foregroundInfo(title, 0, file.sizeBytes))
        } catch (e: IllegalStateException) {
            // Premier plan refusé (app en arrière-plan, Android 12+) : l'envoi continue sans notification.
        }
        setProgress(workDataOf(keys.KEY_TITLE to title, keys.KEY_SENT to 0L, keys.KEY_TOTAL to file.sizeBytes))

        var lastPercent = -1
        val result = client.upload(file, title, inputData.getString(keys.KEY_DESCRIPTION)) { sent, total ->
            setProgressAsync(workDataOf(keys.KEY_TITLE to title, keys.KEY_SENT to sent, keys.KEY_TOTAL to total))
            val percent = if (total == 0L) 0 else (sent * 100 / total).toInt()
            if (percent != lastPercent) {
                lastPercent = percent
                setForegroundAsync(foregroundInfo(title, sent, total))
            }
        }
        return when (result) {
            is ApiResult.Success -> {
                releaseUri(uri)
                Result.success(workDataOf(keys.KEY_VIDEO_ID to result.data.id, keys.KEY_TITLE to title))
            }
            is ApiResult.Failure -> {
                releaseUri(uri)
                Result.failure(
                    workDataOf(
                        keys.KEY_TITLE to title,
                        keys.KEY_STATUS to result.httpStatus,
                        keys.KEY_CODE to result.code,
                        keys.KEY_MESSAGE to result.message,
                        keys.KEY_REQUEST_ID to result.requestId,
                    ),
                )
            }
            // Réseau perdu : WorkManager relance quand il revient (nouvel envoi, l'ancien est annulé).
            is ApiResult.NetworkError -> if (runAttemptCount < MAX_ATTEMPTS) {
                Result.retry()
            } else {
                releaseUri(uri)
                Result.failure(workDataOf(keys.KEY_TITLE to title, keys.KEY_NETWORK to true))
            }
        }
    }

    override suspend fun getForegroundInfo(): ForegroundInfo = foregroundInfo(inputData.getString(WorkManagerUploadScheduler.KEY_TITLE).orEmpty(), 0, 0)

    private fun releaseUri(uri: String) {
        try {
            applicationContext.contentResolver.releasePersistableUriPermission(Uri.parse(uri), Intent.FLAG_GRANT_READ_URI_PERMISSION)
        } catch (e: SecurityException) {
            // Aucune autorisation durable n'avait été obtenue.
        }
    }

    private fun foregroundInfo(title: String, sent: Long, total: Long): ForegroundInfo {
        val manager = NotificationManagerCompat.from(applicationContext)
        manager.createNotificationChannel(
            NotificationChannelCompat.Builder(CHANNEL_ID, NotificationManagerCompat.IMPORTANCE_LOW)
                .setName(applicationContext.getString(R.string.upload_channel_name))
                .build(),
        )
        val percent = if (total == 0L) 0 else (sent * 100 / total).toInt()
        val notification: Notification = NotificationCompat.Builder(applicationContext, CHANNEL_ID)
            .setSmallIcon(android.R.drawable.stat_sys_upload)
            .setContentTitle(applicationContext.getString(R.string.upload_notification_title))
            .setContentText(title)
            .setProgress(100, percent, total == 0L)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .build()
        return if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            ForegroundInfo(NOTIFICATION_ID, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC)
        } else {
            ForegroundInfo(NOTIFICATION_ID, notification)
        }
    }

    private companion object {
        const val CHANNEL_ID = "video_upload"
        const val NOTIFICATION_ID = 4201
        const val MAX_ATTEMPTS = 5
    }
}
