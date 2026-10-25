package com.poro.app.data.video

import com.poro.app.core.media.LocalMedia
import com.poro.app.core.media.PreparedImage
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PageDto
import com.poro.app.testing.FakeAuthRepository.Companion.failure
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class UploadManagerTest {

    private val file = LocalMedia("content://v", "video/mp4", 100)

    /** Statuts successifs renvoyés par GET /videos/{id}. */
    private class FakeVideos(vararg statuses: String) : VideoRepository {
        val queue = ArrayDeque(statuses.toList())
        override suspend fun video(id: String) = ApiResult.Success(VideoDto(id = id, status = queue.removeFirst()))
        override suspend fun mine(cursor: String?) = ApiResult.Success(PageDto<VideoDto>())
        override suspend fun byUser(userId: String, cursor: String?): ApiResult<PageDto<VideoDto>> = error("unused")
        override suspend fun delete(id: String) = ApiResult.Success(Unit)
    }

    @Test
    fun `progress, then processing until the video is ready`() = runTest {
        val gate = CompletableDeferred<Unit>()
        val client = VideoUploadClient { _, title, _, onProgress ->
            onProgress(50, 100)
            gate.await()
            onProgress(100, 100)
            ApiResult.Success(VideoDto(id = "v1", title = title, status = VideoDto.STATUS_PROCESSING))
        }
        val manager = UploadManager(InProcessUploadScheduler(client, backgroundScope), FakeVideos("processing", "ready"), backgroundScope, pollDelaysMs = listOf(1_000, 1_000, 1_000))

        assertTrue(manager.start(file, "Match", null))
        runCurrent()
        assertEquals(0.5f, (manager.state.value as UploadState.Uploading).progress)
        assertFalse("un seul envoi à la fois", manager.start(file, "Autre", null))

        gate.complete(Unit)
        runCurrent()
        assertTrue(manager.state.value is UploadState.Processing)
        advanceTimeBy(2_001)
        runCurrent()
        assertEquals(VideoDto.STATUS_READY, (manager.state.value as UploadState.Finished).video.status)
    }

    @Test
    fun `a failed upload keeps its error until dismissed`() = runTest {
        val client = VideoUploadClient { _, _, _, _ -> failure(422, ApiErrorCodes.INVALID_MEDIA) }
        val manager = UploadManager(InProcessUploadScheduler(client, backgroundScope), FakeVideos(), backgroundScope)

        manager.start(file, "Match", null)
        runCurrent()
        assertEquals(ApiErrorCodes.INVALID_MEDIA, ((manager.state.value as UploadState.Failed).failure as ApiResult.Failure).code)

        manager.dismiss()
        assertEquals(UploadState.Idle, manager.state.value)
    }

    @Test
    fun `cancel stops the upload`() = runTest {
        val never = CompletableDeferred<Unit>()
        val client = VideoUploadClient { _, _, _, _ ->
            never.await()
            ApiResult.Success(VideoDto(id = "v1"))
        }
        val manager = UploadManager(InProcessUploadScheduler(client, backgroundScope), FakeVideos(), backgroundScope)
        manager.start(file, "Match", null)
        runCurrent()

        manager.cancel()
        runCurrent()

        assertEquals(UploadState.Idle, manager.state.value)
        assertTrue("on peut relancer", manager.start(file, "Match", null))
    }

    /** Planificateur piloté par le test, comme WorkManager relu au redémarrage de l'app. */
    private class FakeScheduler(initial: ScheduledUpload) : UploadScheduler {
        val flow = MutableStateFlow(initial)
        val calls = mutableListOf<String>()
        override val upload: Flow<ScheduledUpload> = flow
        override fun enqueue(file: LocalMedia, title: String, description: String?) { calls += "enqueue:$title" }
        override fun enqueuePhotos(images: List<PreparedImage>, caption: String?, label: String) {
            calls += "photos:${images.size}:$label"
        }
        override fun cancel() { calls += "cancel" }
        override fun clear() { calls += "clear" }
    }

    @Test
    fun `an upload still running after a restart is shown again`() = runTest {
        val scheduler = FakeScheduler(ScheduledUpload.Running("Match", 30, 100))
        val manager = UploadManager(scheduler, FakeVideos(), backgroundScope)
        runCurrent()

        assertEquals(0.3f, (manager.state.value as UploadState.Uploading).progress)
        assertFalse(manager.start(file, "Autre", null))
    }

    @Test
    fun `a dismissed result is not shown again when the scheduler repeats it`() = runTest {
        val scheduler = FakeScheduler(ScheduledUpload.Succeeded("w1", "v1", "Match"))
        val manager = UploadManager(scheduler, FakeVideos(), backgroundScope, pollDelaysMs = emptyList())
        runCurrent()
        assertEquals("v1", (manager.state.value as UploadState.Processing).video.id)

        manager.dismiss()
        // WorkManager réémet l'envoi terminé (même identifiant) avant de l'oublier.
        scheduler.flow.value = ScheduledUpload.Succeeded("w1", "v1", "Match (relu)")
        runCurrent()
        assertEquals(UploadState.Idle, manager.state.value)
        assertEquals(listOf("clear"), scheduler.calls)

        // Un nouvel envoi terminé, lui, s'affiche.
        scheduler.flow.value = ScheduledUpload.Succeeded("w2", "v2", "Finale")
        runCurrent()
        assertEquals("v2", (manager.state.value as UploadState.Processing).video.id)
    }

    @Test
    fun `a photo post uses the same queue and is followed as a photo post`() = runTest {
        val scheduler = FakeScheduler(ScheduledUpload.None)
        val manager = UploadManager(scheduler, FakeVideos(VideoDto.STATUS_READY), backgroundScope, pollDelaysMs = listOf(1))
        runCurrent()
        val images = listOf(PreparedImage("/p/1.jpg", 300), PreparedImage("/p/2.jpg", 200))

        assertTrue(manager.startPhotos(images, "Marché", "Marché"))
        assertEquals(500L, (manager.state.value as UploadState.Uploading).totalBytes)
        assertFalse("une seule publication à la fois", manager.start(file, "Match", null))
        assertEquals(listOf("photos:2:Marché"), scheduler.calls)

        scheduler.flow.value = ScheduledUpload.Succeeded("w1", "p1", "Marché", photos = true)
        runCurrent()
        assertTrue((manager.state.value as UploadState.Processing).video.isPhoto)
    }
}
