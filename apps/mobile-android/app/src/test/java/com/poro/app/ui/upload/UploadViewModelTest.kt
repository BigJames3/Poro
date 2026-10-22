package com.poro.app.ui.upload

import androidx.lifecycle.SavedStateHandle
import com.poro.app.R
import com.poro.app.core.media.LocalMedia
import com.poro.app.core.media.LocalMediaReader
import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PageDto
import com.poro.app.data.video.InProcessUploadScheduler
import com.poro.app.data.video.UploadManager
import com.poro.app.data.video.UploadState
import com.poro.app.data.video.VideoDto
import com.poro.app.data.video.VideoRepository
import com.poro.app.data.video.VideoUploadClient
import com.poro.app.testing.MainDispatcherRule
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class UploadViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    private val files = mapOf(
        "ok" to LocalMedia("ok", "video/mp4", 10_000_000, durationMs = 60_000),
        "long" to LocalMedia("long", "video/mp4", 10_000_000, durationMs = 200_000),
        "big" to LocalMedia("big", "video/quicktime", 300L * 1024 * 1024, durationMs = 60_000),
        "avi" to LocalMedia("avi", "video/x-msvideo", 1_000, durationMs = 1_000),
    )
    private val reader = object : LocalMediaReader {
        override suspend fun describe(uri: String) = files[uri]
        override suspend fun read(uri: String, offset: Long, length: Int) = ByteArray(length)
    }
    private val videos = object : VideoRepository {
        override suspend fun video(id: String) = ApiResult.Success(VideoDto(id))
        override suspend fun mine(cursor: String?) = ApiResult.Success(PageDto<VideoDto>())
        override suspend fun delete(id: String) = ApiResult.Success(Unit)
    }
    private val started = mutableListOf<String>()

    private fun TestScope.viewModel(): UploadViewModel {
        val client = VideoUploadClient { file, title, description, _ ->
            started += "${file.uri}:$title:$description"
            ApiResult.Success(VideoDto(id = "v1", status = VideoDto.STATUS_PROCESSING))
        }
        return UploadViewModel(UploadManager(InProcessUploadScheduler(client, backgroundScope), videos, backgroundScope, pollDelaysMs = emptyList()), reader, SavedStateHandle())
    }

    @Test
    fun `videos are checked before any upload`() = runTest(main.dispatcher) {
        val vm = viewModel()
        for ((uri, error) in listOf("long" to R.string.upload_error_duration, "big" to R.string.upload_error_size, "avi" to R.string.upload_error_format)) {
            vm.onVideoPicked(uri)
            runCurrent()
            assertEquals(uri, error, vm.form.value.fileError?.text)
            assertNull(vm.form.value.file)
        }
    }

    @Test
    fun `a valid video with a title is published and the form resets`() = runTest(main.dispatcher) {
        val vm = viewModel()
        vm.onVideoPicked("ok")
        runCurrent()
        assertFalse("titre requis", vm.form.value.canPublish)

        vm.onTitleChange("  Match du dimanche ")
        vm.onDescriptionChange("#foot")
        vm.publish()
        runCurrent()

        assertEquals(listOf("ok:Match du dimanche:#foot"), started)
        assertNull(vm.form.value.file)
        assertEquals("", vm.form.value.title)
        assertTrue(vm.upload.value is UploadState.Processing)
    }
}
