package com.poro.app.ui.upload

import androidx.lifecycle.SavedStateHandle
import com.poro.app.R
import com.poro.app.core.media.LocalMedia
import com.poro.app.core.media.LocalMediaReader
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PageDto
import com.poro.app.data.video.InProcessUploadScheduler
import com.poro.app.data.video.UploadManager
import com.poro.app.data.video.UploadState
import com.poro.app.data.video.VideoDto
import com.poro.app.data.video.VideoRepository
import com.poro.app.data.video.VideoUploadClient
import com.poro.app.testing.MainDispatcherRule
import com.poro.app.ui.auth.toErrorMessage
import java.io.IOException
import kotlinx.coroutines.CompletableDeferred
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
    /** URI dont l'autorisation de lecture a été perdue. */
    private val revoked = mutableSetOf<String>()
    private val reader = object : LocalMediaReader {
        override suspend fun describe(uri: String): LocalMedia? {
            if (uri in revoked) throw SecurityException("permission revoked")
            return files[uri]
        }
        override suspend fun read(uri: String, offset: Long, length: Int) = ByteArray(length)
    }
    private val videos = object : VideoRepository {
        override suspend fun video(id: String) = ApiResult.Success(VideoDto(id))
        override suspend fun mine(cursor: String?) = ApiResult.Success(PageDto<VideoDto>())
        override suspend fun byUser(userId: String, cursor: String?): ApiResult<PageDto<VideoDto>> = error("unused")
        override suspend fun delete(id: String) = ApiResult.Success(Unit)
    }
    private val started = mutableListOf<String>()

    /** Réponses successives de l'envoi ; vide : la vidéo v1 est acceptée. */
    private val results = ArrayDeque<ApiResult<VideoDto>>()

    /** Envoi suspendu jusqu'à ce que le test le libère (envoi « en cours »). */
    private var gate: CompletableDeferred<Unit>? = null

    private fun TestScope.viewModel(savedState: SavedStateHandle = SavedStateHandle()): UploadViewModel {
        val client = VideoUploadClient { file, title, description, _ ->
            started += "${file.uri}:$title:$description"
            gate?.await()
            results.removeFirstOrNull() ?: ApiResult.Success(VideoDto(id = "v1", status = VideoDto.STATUS_PROCESSING))
        }
        return UploadViewModel(UploadManager(InProcessUploadScheduler(client, backgroundScope), videos, backgroundScope, pollDelaysMs = emptyList()), reader, savedState)
    }

    private fun TestScope.readyToPublish(vm: UploadViewModel) {
        vm.onVideoPicked("ok")
        runCurrent()
        vm.onTitleChange("Match")
        vm.onDescriptionChange("#foot")
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
        vm.onTitleChange("  Match du dimanche ")
        vm.onDescriptionChange("#foot")
        vm.publish()
        runCurrent()

        assertEquals(listOf("ok:Match du dimanche:#foot"), started)
        assertNull(vm.form.value.file)
        assertEquals("", vm.form.value.title)
        assertTrue(vm.upload.value is UploadState.Processing)
    }

    @Test
    fun `a video can be published without a title`() = runTest(main.dispatcher) {
        val vm = viewModel()
        vm.onVideoPicked("ok")
        runCurrent()
        assertTrue("le titre est facultatif", vm.form.value.canPublish)
        vm.onTitleChange("   ")
        assertTrue(vm.form.value.canPublish)

        vm.publish()
        runCurrent()

        assertEquals(listOf("ok::null"), started)
    }

    @Test
    fun `a double tap on publish sends the video once`() = runTest(main.dispatcher) {
        gate = CompletableDeferred()
        val vm = viewModel()
        readyToPublish(vm)

        vm.publish()
        vm.publish()
        runCurrent()
        assertTrue(vm.upload.value is UploadState.Uploading)
        gate!!.complete(Unit)
        runCurrent()

        assertEquals(listOf("ok:Match:#foot"), started)
    }

    @Test
    fun `after a network failure the form is kept and retry sends the same video again`() = runTest(main.dispatcher) {
        results += ApiResult.NetworkError(IOException("offline"))
        val vm = viewModel()
        readyToPublish(vm)

        vm.publish()
        runCurrent()

        val failed = vm.upload.value as UploadState.Failed
        assertTrue(failed.canRetry())
        assertEquals("ok", vm.form.value.file?.uri)
        assertEquals("Match", vm.form.value.title)
        assertEquals("#foot", vm.form.value.description)

        vm.retry()
        vm.retry()
        runCurrent()

        assertEquals(listOf("ok:Match:#foot", "ok:Match:#foot"), started)
        assertTrue(vm.upload.value is UploadState.Processing)
        // Confirmé par le serveur : le formulaire repart à vide.
        assertNull(vm.form.value.file)
        assertEquals("", vm.form.value.title)
    }

    @Test
    fun `a refusal of the content is not retried as is`() = runTest(main.dispatcher) {
        results += ApiResult.Failure(422, ApiErrorCodes.INVALID_REQUEST, "title", null, null)
        val vm = viewModel()
        readyToPublish(vm)
        vm.publish()
        runCurrent()

        assertFalse((vm.upload.value as UploadState.Failed).canRetry())
        vm.retry()
        runCurrent()

        assertEquals(1, started.size)
        assertEquals("formulaire gardé pour corriger", "Match", vm.form.value.title)
        vm.dismissResult()
        assertEquals(UploadState.Idle, vm.upload.value)
        assertEquals("ok", vm.form.value.file?.uri)
    }

    @Test
    fun `a video lost during the upload asks to pick it again and keeps the text`() = runTest(main.dispatcher) {
        results += ApiResult.Failure(0, ApiErrorCodes.MEDIA_UNREADABLE, "unreadable", null, null)
        val vm = viewModel()
        readyToPublish(vm)
        vm.publish()
        runCurrent()

        val failed = vm.upload.value as UploadState.Failed
        assertTrue(failed.mediaLost)
        assertFalse(failed.canRetry())
        assertNull(vm.form.value.file)
        assertEquals(R.string.upload_error_unreadable, vm.form.value.fileError?.text)
        assertEquals("Match", vm.form.value.title)
        assertEquals("#foot", vm.form.value.description)
        assertEquals(R.string.upload_error_unreadable, failed.failure.toErrorMessage()?.text)
    }

    @Test
    fun `a restored video without read access asks to pick it again`() = runTest(main.dispatcher) {
        revoked += "ok"
        val vm = viewModel(
            SavedStateHandle().apply {
                this["uri"] = "ok"
                this["title"] = "Match"
                this["description"] = "#foot"
            },
        )
        runCurrent()

        assertNull(vm.form.value.file)
        assertEquals(R.string.upload_error_unreadable, vm.form.value.fileError?.text)
        assertEquals("Match", vm.form.value.title)
        assertFalse(vm.form.value.checkingFile)
    }
}

