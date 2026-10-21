package com.poro.app.data.video

import com.poro.app.core.media.LocalMedia
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PageDto
import com.poro.app.testing.FakeAuthRepository.Companion.failure
import kotlinx.coroutines.CompletableDeferred
import kotlinx.coroutines.ExperimentalCoroutinesApi
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
        val manager = UploadManager(client, FakeVideos("processing", "ready"), backgroundScope, pollDelaysMs = listOf(1_000, 1_000, 1_000))

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
        val manager = UploadManager(client, FakeVideos(), backgroundScope)

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
        val manager = UploadManager(client, FakeVideos(), backgroundScope)
        manager.start(file, "Match", null)
        runCurrent()

        manager.cancel()
        runCurrent()

        assertEquals(UploadState.Idle, manager.state.value)
        assertTrue("on peut relancer", manager.start(file, "Match", null))
    }
}
