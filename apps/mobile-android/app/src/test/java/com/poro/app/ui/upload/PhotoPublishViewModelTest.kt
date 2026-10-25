package com.poro.app.ui.upload

import androidx.lifecycle.SavedStateHandle
import com.poro.app.R
import com.poro.app.core.media.LocalImagePreparer
import com.poro.app.core.media.LocalMedia
import com.poro.app.core.media.LocalMediaReader
import com.poro.app.core.media.PrepareResult
import com.poro.app.core.media.PreparedImage
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PageDto
import com.poro.app.data.video.InProcessUploadScheduler
import com.poro.app.data.video.PhotoUploadClient
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
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class PhotoPublishViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    private val reader = object : LocalMediaReader {
        override suspend fun describe(uri: String): LocalMedia? = null
        override suspend fun read(uri: String, offset: Long, length: Int) = ByteArray(length)
    }
    private val videos = object : VideoRepository {
        override suspend fun video(id: String) = ApiResult.Success(VideoDto(id))
        override suspend fun mine(cursor: String?) = ApiResult.Success(PageDto<VideoDto>())
        override suspend fun delete(id: String) = ApiResult.Success(Unit)
    }

    /** Préparation programmée : chaque URI devient un fichier, sauf celles marquées illisibles. */
    private class FakePreparer : LocalImagePreparer {
        val unsupported = mutableSetOf<String>()
        val prepared = mutableListOf<List<String>>()
        val discarded = mutableListOf<List<PreparedImage>>()

        override suspend fun prepare(uris: List<String>): PrepareResult {
            prepared += uris
            uris.forEachIndexed { i, uri ->
                if (uri in unsupported) return PrepareResult.Failed(i + 1, PrepareResult.Reason.Unsupported)
            }
            return PrepareResult.Ready(uris.map { PreparedImage("/files/$it.jpg", 1_000) })
        }

        override fun read(image: PreparedImage) = ByteArray(image.sizeBytes.toInt())
        override fun discard(images: List<PreparedImage>) { discarded += images }
    }

    private val preparer = FakePreparer()
    private val sent = mutableListOf<String>()
    private val results = ArrayDeque<ApiResult<VideoDto>>()

    private fun TestScope.viewModel(savedState: SavedStateHandle = SavedStateHandle()): UploadViewModel {
        val video = VideoUploadClient { _, _, _, _ -> error("no video in these tests") }
        val photos = PhotoUploadClient { images, caption, _ ->
            sent += images.joinToString(",") { it.path } + "|" + caption
            results.removeFirstOrNull() ?: ApiResult.Success(VideoDto(id = "p1", status = VideoDto.STATUS_PROCESSING, mediaType = "photo"))
        }
        val manager = UploadManager(InProcessUploadScheduler(video, backgroundScope, photos), videos, backgroundScope, pollDelaysMs = emptyList())
        return UploadViewModel(manager, reader, savedState, preparer)
    }

    @Test
    fun `one to ten photos, in the chosen order, with an optional caption`() = runTest(main.dispatcher) {
        val vm = viewModel()
        vm.setMode(PublishMode.Photo)
        assertFalse("aucune photo", vm.form.value.canPublish)

        vm.onPhotosPicked(listOf("a", "b", "c"))
        vm.movePhoto(2, 0)
        vm.removePhoto(1)
        assertEquals(listOf("c", "b"), vm.form.value.photos)
        assertTrue("la légende est facultative", vm.form.value.canPublish)

        vm.publish()
        runCurrent()
        assertEquals(listOf("/files/c.jpg,/files/b.jpg|null"), sent)
        assertTrue(vm.upload.value is UploadState.Processing)
        assertEquals("le formulaire repart à vide une fois la publication acceptée", emptyList<String>(), vm.form.value.photos)
    }

    @Test
    fun `more than ten photos are not added, and the user is told`() = runTest(main.dispatcher) {
        val vm = viewModel()
        vm.setMode(PublishMode.Photo)
        vm.onPhotosPicked((1..8).map { "p$it" })
        vm.onPhotosPicked(listOf("p8", "x1", "x2", "x3"))

        assertEquals(10, vm.form.value.photos.size)
        assertEquals("un doublon n'est pas ajouté deux fois", 1, vm.form.value.photos.count { it == "p8" })
        assertEquals(R.string.upload_photo_too_many, vm.form.value.photoError?.text)
    }

    @Test
    fun `an unreadable photo stops everything before sending and names its position`() = runTest(main.dispatcher) {
        val vm = viewModel()
        vm.setMode(PublishMode.Photo)
        vm.onPhotosPicked(listOf("a", "heic"))
        preparer.unsupported += "heic"

        vm.publish()
        runCurrent()

        assertEquals(emptyList<String>(), sent)
        assertEquals(R.string.upload_photo_unsupported, vm.form.value.photoError?.text)
        assertEquals(2, vm.form.value.photoError?.arg)
        assertEquals("les photos restent pour corriger", listOf("a", "heic"), vm.form.value.photos)
    }

    @Test
    fun `a network failure keeps the photos and retry prepares them again`() = runTest(main.dispatcher) {
        val vm = viewModel()
        vm.setMode(PublishMode.Photo)
        vm.onPhotosPicked(listOf("a"))
        vm.onCaptionChange(" Marché #abidjan ")
        results += ApiResult.NetworkError(java.io.IOException("offline"))

        vm.publish()
        runCurrent()
        assertTrue(vm.upload.value is UploadState.Failed)
        assertEquals(listOf("a"), vm.form.value.photos)

        vm.retry()
        runCurrent()
        assertEquals(2, preparer.prepared.size)
        assertEquals("/files/a.jpg|Marché #abidjan", sent.last())
        assertTrue(vm.upload.value is UploadState.Processing)
    }

    @Test
    fun `a lost prepared file is retried from the chosen photos`() = runTest(main.dispatcher) {
        val vm = viewModel()
        vm.setMode(PublishMode.Photo)
        vm.onPhotosPicked(listOf("a"))
        results += ApiResult.Failure(0, ApiErrorCodes.MEDIA_UNREADABLE, "gone", null, null)

        vm.publish()
        runCurrent()
        assertEquals("les photos choisies restent", listOf("a"), vm.form.value.photos)
        vm.retry()
        runCurrent()
        assertTrue(vm.upload.value is UploadState.Processing)
    }

    @Test
    fun `the photo form survives a process restart`() = runTest(main.dispatcher) {
        val saved = SavedStateHandle()
        viewModel(saved).apply {
            setMode(PublishMode.Photo)
            onPhotosPicked(listOf("a", "b"))
            onCaptionChange("Légende")
        }
        val restored = viewModel(saved)
        assertEquals(PublishMode.Photo, restored.form.value.mode)
        assertEquals(listOf("a", "b"), restored.form.value.photos)
        assertEquals("Légende", restored.form.value.caption)
    }
}
