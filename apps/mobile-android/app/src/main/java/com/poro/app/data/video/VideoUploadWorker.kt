package com.poro.app.data.video

import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Context
import android.content.pm.ServiceInfo
import android.os.Build
import androidx.core.app.NotificationCompat
import androidx.work.CoroutineWorker
import androidx.work.ForegroundInfo
import androidx.work.WorkerParameters
import com.poro.app.PoroApp
import com.poro.app.R
import com.poro.app.core.media.LocalMedia
import com.poro.app.core.network.ApiResult
import java.io.File
import java.io.IOException
import kotlinx.coroutines.flow.first

/**
 * Envoi durable : copie locale puis [UploadManager.execute]. Un arrêt du processus relance
 * l'envoi depuis le début (nouveau `POST /uploads`) : le contrat n'expose pas la reprise
 * des parties déjà envoyées.
 */
class VideoUploadWorker(
    context: Context,
    params: WorkerParameters,
) : CoroutineWorker(context, params) {

    override suspend fun doWork(): Result {
        val app = applicationContext as? PoroApp ?: return Result.failure()
        val container = app.container
        val pending = container.pendingUploads.load() ?: return Result.success()
        val local = try {
            ensureCopy(container, pending)
        } catch (e: IOException) {
            return Result.retry()
        }
        try {
            setForeground(foreground(pending.title))
        } catch (_: IllegalStateException) {
            // POST_NOTIFICATIONS refusé : l'envoi continue sans notification visible.
        }
        if (!container.uploadManager.execute(local, pending.title, pending.description)) {
            return Result.success()
        }
        val terminal = container.uploadManager.state.first { it !is UploadState.Uploading }
        return when (terminal) {
            is UploadState.Failed -> if (terminal.failure is ApiResult.NetworkError) Result.retry() else Result.failure()
            UploadState.Idle -> Result.failure()
            else -> {
                container.pendingUploads.clear()
                localCopy(pending)?.delete()
                Result.success()
            }
        }
    }

    private suspend fun ensureCopy(container: com.poro.app.AppContainer, pending: PendingUpload): LocalMedia {
        val existing = pending.copiedPath?.let(::File)?.takeIf { it.isFile && it.length() == pending.sizeBytes }
        if (existing != null) return LocalMedia(existing.toURI().toString(), pending.mimeType, pending.sizeBytes)
        val dest = File(applicationContext.filesDir, "uploads").apply { mkdirs() }
        val copy = File(dest, "pending-upload")
        container.mediaReader.copyTo(pending.uri, copy)
        container.pendingUploads.save(pending.copy(copiedPath = copy.absolutePath, uri = copy.toURI().toString()))
        return LocalMedia(copy.toURI().toString(), pending.mimeType, copy.length())
    }

    private fun localCopy(pending: PendingUpload): File? = pending.copiedPath?.let(::File)

    private fun foreground(title: String): ForegroundInfo {
        val manager = applicationContext.getSystemService(NotificationManager::class.java)
        if (Build.VERSION.SDK_INT >= 26) {
            manager.createNotificationChannel(
                NotificationChannel(CHANNEL, applicationContext.getString(R.string.upload_notification_title), NotificationManager.IMPORTANCE_LOW),
            )
        }
        val notification = NotificationCompat.Builder(applicationContext, CHANNEL)
            .setContentTitle(applicationContext.getString(R.string.upload_notification_title))
            .setContentText(applicationContext.getString(R.string.upload_notification_text, title))
            .setSmallIcon(android.R.drawable.stat_sys_upload)
            .setOngoing(true)
            .setSilent(true)
            .build()
        return if (Build.VERSION.SDK_INT >= 29) {
            ForegroundInfo(NOTIFICATION_ID, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_DATA_SYNC)
        } else {
            ForegroundInfo(NOTIFICATION_ID, notification)
        }
    }

    private companion object {
        const val CHANNEL = "poro_uploads"
        const val NOTIFICATION_ID = 41
    }
}
