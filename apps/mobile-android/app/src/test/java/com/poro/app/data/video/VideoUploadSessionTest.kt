package com.poro.app.data.video

import com.poro.app.core.media.LocalMedia
import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PoroJson
import com.poro.app.core.network.PresignedUploader
import com.poro.app.testing.FakeMediaReader
import kotlinx.coroutines.test.runTest
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import retrofit2.Retrofit
import retrofit2.converter.kotlinx.serialization.asConverterFactory

/**
 * Envoi interrompu par l'arrêt du processus : le serveur n'a pas de reprise, l'app règle l'envoi
 * laissé en suspens avant d'en créer un autre (ni doublon publié, ni orphelin `uploading`).
 */
class VideoUploadSessionTest {

    private lateinit var api: MockWebServer
    private lateinit var storage: MockWebServer
    private val media = FakeMediaReader()
    private val store = MemoryStore()
    private lateinit var uploader: VideoUploader
    private val file = LocalMedia("content://video", "video/mp4", 4)

    private class MemoryStore : UploadSessionStore {
        var session: UploadSession? = null
        override fun load() = session
        override fun save(session: UploadSession) { this.session = session }
        override fun clear() { session = null }
    }

    @Before
    fun setUp() {
        api = MockWebServer()
        storage = MockWebServer()
        media.add(file.uri, file.mimeType, ByteArray(4))
        val videoApi = Retrofit.Builder()
            .baseUrl(api.url("/"))
            .client(OkHttpClient())
            .addConverterFactory(PoroJson.asConverterFactory("application/json".toMediaType()))
            .build()
            .create(VideoApi::class.java)
        uploader = VideoUploader(videoApi, PresignedUploader(OkHttpClient()), media, retryDelaysMs = emptyList(), sessions = store)
    }

    @After
    fun tearDown() {
        api.shutdown()
        storage.shutdown()
    }

    private fun json(code: Int, body: String) = MockResponse().setResponseCode(code).setHeader("Content-Type", "application/json").setBody(body)
    private fun video(id: String, status: String) = json(200, """{"data":{"id":"$id","status":"$status"},"error":null,"meta":{}}""")
    private fun init(id: String) = json(
        201,
        """{"data":{"video_id":"$id","upload_id":"u","key":"k","part_size":4,"parts":[{"part_number":1,"url":"${storage.url("/p1")}"}]},"error":null,"meta":{}}""",
    )

    @Test
    fun `a completed upload is not published twice after a restart`() = runTest {
        // Le processus est mort après un « complete » réussi, avant que WorkManager l'enregistre.
        store.session = UploadSession("v1", uploadFingerprint(file, "Match", null))
        api.enqueue(video("v1", VideoDto.STATUS_PROCESSING))

        val result = uploader.upload(file, "Match", null) { _, _ -> }

        assertEquals("v1", (result as ApiResult.Success).data.id)
        assertEquals(1, api.requestCount)
        assertEquals("/api/v1/videos/v1", api.takeRequest().path)
        assertNull(store.session)
    }

    @Test
    fun `an upload left in progress is aborted before a new one starts`() = runTest {
        store.session = UploadSession("v0", uploadFingerprint(file, "Match", null))
        api.enqueue(video("v0", VideoDto.STATUS_UPLOADING))
        api.enqueue(MockResponse().setResponseCode(204))
        api.enqueue(init("v1"))
        storage.enqueue(MockResponse().setResponseCode(200).setHeader("ETag", "\"e1\""))
        api.enqueue(video("v1", VideoDto.STATUS_PROCESSING))

        val result = uploader.upload(file, "Match", null) { _, _ -> }

        assertEquals("v1", (result as ApiResult.Success).data.id)
        assertEquals("GET /api/v1/videos/v0", api.takeRequest().let { "${it.method} ${it.path}" })
        assertEquals("DELETE /api/v1/videos/uploads/v0", api.takeRequest().let { "${it.method} ${it.path}" })
        assertEquals("POST /api/v1/videos/uploads", api.takeRequest().let { "${it.method} ${it.path}" })
        assertNull("terminé : plus rien à reprendre", store.session)
    }

    @Test
    fun `a published video of another upload is neither returned nor aborted`() = runTest {
        // Ancien envoi différent (autre titre) déjà publié : on l'oublie, sans toucher à la vidéo.
        store.session = UploadSession("v0", uploadFingerprint(file, "Autre", null))
        api.enqueue(video("v0", VideoDto.STATUS_READY))
        api.enqueue(init("v1"))
        storage.enqueue(MockResponse().setResponseCode(200).setHeader("ETag", "\"e1\""))
        api.enqueue(video("v1", VideoDto.STATUS_PROCESSING))

        val result = uploader.upload(file, "Match", null) { _, _ -> }

        assertEquals("v1", (result as ApiResult.Success).data.id)
        val methods = (1..api.requestCount).map { api.takeRequest().method }
        assertTrue("aucune annulation", "DELETE" !in methods)
    }

    @Test
    fun `without network the leftover is kept for the next attempt`() = runTest {
        store.session = UploadSession("v0", uploadFingerprint(file, "Match", null))
        api.shutdown()

        val result = uploader.upload(file, "Match", null) { _, _ -> }

        assertTrue(result is ApiResult.NetworkError)
        assertEquals("v0", store.session?.videoId)
    }

    @Test
    fun `the session is remembered during the upload and forgotten after a failure`() = runTest {
        api.enqueue(init("v1"))
        storage.enqueue(MockResponse().setResponseCode(403))
        api.enqueue(MockResponse().setResponseCode(204))

        val result = uploader.upload(file, "Match", null) { _, _ -> }

        assertTrue(result is ApiResult.Failure)
        assertNull("envoi annulé côté serveur, rien à reprendre", store.session)
    }
}
