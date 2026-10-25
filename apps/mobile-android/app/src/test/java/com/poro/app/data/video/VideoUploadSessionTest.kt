package com.poro.app.data.video

import com.poro.app.core.media.LocalMedia
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PoroJson
import com.poro.app.core.network.PresignedUploader
import com.poro.app.testing.FakeMediaReader
import kotlinx.coroutines.test.runTest
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import okhttp3.mockwebserver.SocketPolicy
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import retrofit2.Retrofit
import retrofit2.converter.kotlinx.serialization.asConverterFactory

/**
 * Envoi interrompu (coupure, arrêt du processus) : l'app règle l'envoi laissé en suspens avant
 * d'en créer un autre. Le même envoi est repris là où le stockage l'a laissé ; un autre est annulé
 * (ni doublon publié, ni orphelin `uploading`).
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
        // Sans rejeu automatique d'OkHttp : une coupure du stockage reste une coupure.
        val storageClient = OkHttpClient.Builder().retryOnConnectionFailure(false).build()
        uploader = VideoUploader(videoApi, PresignedUploader(storageClient), media, retryDelaysMs = emptyList(), sessions = store)
    }

    @After
    fun tearDown() {
        api.shutdown()
        storage.shutdown()
    }

    private fun json(code: Int, body: String) = MockResponse().setResponseCode(code).setHeader("Content-Type", "application/json").setBody(body)
    private fun video(id: String, status: String) = json(200, """{"data":{"id":"$id","status":"$status"},"error":null,"meta":{}}""")
    private fun resume(id: String, uploaded: String, missing: String) = json(
        200,
        """{"data":{"video_id":"$id","part_size":4,"uploaded_parts":[$uploaded],"parts":[$missing]},"error":null,"meta":{}}""",
    )
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
    fun `another upload left in progress is aborted before a new one starts`() = runTest {
        store.session = UploadSession("v0", uploadFingerprint(file, "Autre", null))
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

    @Test
    fun `a lost answer to complete keeps the session and the retry does not publish twice`() = runTest {
        api.enqueue(init("v1"))
        storage.enqueue(MockResponse().setResponseCode(200).setHeader("ETag", "\"e1\""))
        // Le serveur a traité « complete », mais la connexion tombe avant la réponse. OkHttp peut
        // rejouer la requête : le serveur répond alors 409 (la vidéo n'est plus `uploading`).
        api.enqueue(MockResponse().setSocketPolicy(SocketPolicy.DISCONNECT_AFTER_REQUEST))
        api.enqueue(json(409, """{"data":null,"error":{"code":"conflict","message":"not uploading"},"meta":{}}"""))

        val first = uploader.upload(file, "Match", null) { _, _ -> }

        assertTrue(first.toString(), first !is ApiResult.Success)
        assertEquals("issue inconnue : rien d'annulé, session gardée", "v1", store.session?.videoId)
        val firstRequests = (1..api.requestCount).map { api.takeRequest().let { r -> "${r.method} ${r.path}" } }
        assertTrue(firstRequests.toString(), firstRequests.none { it.startsWith("DELETE") })

        // Nouvel essai (WorkManager ou « Réessayer ») : la vidéo est relue, pas republiée.
        val before = api.requestCount
        api.dispatcher = object : okhttp3.mockwebserver.Dispatcher() {
            override fun dispatch(request: okhttp3.mockwebserver.RecordedRequest) =
                if (request.method == "GET" && request.path == "/api/v1/videos/v1") video("v1", VideoDto.STATUS_PROCESSING) else MockResponse().setResponseCode(500)
        }
        val second = uploader.upload(file, "Match", null) { _, _ -> }

        assertEquals("v1", (second as ApiResult.Success).data.id)
        assertEquals("GET /api/v1/videos/v1", api.takeRequest().let { "${it.method} ${it.path}" })
        assertEquals("aucun second envoi", before + 1, api.requestCount)
        assertNull(store.session)
    }

    @Test
    fun `when complete never reached the server the retry resumes and completes it`() = runTest {
        api.enqueue(init("v1"))
        storage.enqueue(MockResponse().setResponseCode(200).setHeader("ETag", "\"e1\""))
        api.enqueue(MockResponse().setResponseCode(503))
        uploader.upload(file, "Match", null) { _, _ -> }
        repeat(api.requestCount) { api.takeRequest() }
        assertEquals("v1", store.session?.videoId)

        api.enqueue(video("v1", VideoDto.STATUS_UPLOADING))
        api.enqueue(resume("v1", uploaded = """{"part_number":1,"etag":"\"e1\""}""", missing = ""))
        api.enqueue(video("v1", VideoDto.STATUS_PROCESSING))
        val retry = uploader.upload(file, "Match", null) { _, _ -> }

        assertEquals("v1", (retry as ApiResult.Success).data.id)
        assertEquals("GET /api/v1/videos/v1", api.takeRequest().let { "${it.method} ${it.path}" })
        assertEquals("POST /api/v1/videos/uploads/v1/resume", api.takeRequest().let { "${it.method} ${it.path}" })
        val complete = api.takeRequest()
        assertEquals("POST /api/v1/videos/uploads/v1/complete", "${complete.method} ${complete.path}")
        assertEquals("""{"parts":[{"part_number":1,"etag":"\"e1\""}]}""", complete.body.readUtf8())
        assertEquals("rien renvoyé au stockage : une seule partie, au premier essai", 1, storage.requestCount)
        assertNull(store.session)
    }

    @Test
    fun `a network cut keeps the upload and the retry sends only the missing parts`() = runTest {
        val big = LocalMedia("content://big", "video/mp4", 8)
        media.add(big.uri, big.mimeType, ByteArray(8) { it.toByte() })
        api.enqueue(
            json(
                201,
                """{"data":{"video_id":"v1","upload_id":"u","key":"k","part_size":4,"parts":[
                {"part_number":1,"url":"${storage.url("/p1")}"},{"part_number":2,"url":"${storage.url("/p2")}"}]},"error":null,"meta":{}}""",
            ),
        )
        storage.enqueue(MockResponse().setResponseCode(200).setHeader("ETag", "\"e1\""))
        storage.enqueue(MockResponse().setSocketPolicy(SocketPolicy.DISCONNECT_AFTER_REQUEST))

        val first = uploader.upload(big, "Match", null) { _, _ -> }

        assertTrue(first.toString(), first is ApiResult.NetworkError)
        assertEquals("coupure : envoi gardé pour la reprise", "v1", store.session?.videoId)
        val firstRequests = (1..api.requestCount).map { api.takeRequest().let { r -> "${r.method} ${r.path}" } }
        assertTrue("rien d'annulé", firstRequests.none { it.startsWith("DELETE") })
        repeat(storage.requestCount) { storage.takeRequest() }
        val storageBefore = storage.requestCount

        api.enqueue(video("v1", VideoDto.STATUS_UPLOADING))
        api.enqueue(resume("v1", uploaded = """{"part_number":1,"etag":"\"e1\""}""", missing = """{"part_number":2,"url":"${storage.url("/p2-fresh")}"}"""))
        storage.enqueue(MockResponse().setResponseCode(200).setHeader("ETag", "\"e2\""))
        api.enqueue(video("v1", VideoDto.STATUS_PROCESSING))
        val progress = mutableListOf<Long>()
        val retry = uploader.upload(big, "Match", null) { sent, _ -> progress += sent }

        assertEquals("v1", (retry as ApiResult.Success).data.id)
        assertEquals("/p2-fresh", storage.takeRequest().path)
        assertEquals("seule la partie manquante est renvoyée", storageBefore + 1, storage.requestCount)
        assertEquals("la progression repart des parties reçues", listOf(4L, 8L), progress)
        repeat(2) { api.takeRequest() }
        assertEquals(
            """{"parts":[{"part_number":1,"etag":"\"e1\""},{"part_number":2,"etag":"\"e2\""}]}""",
            api.takeRequest().body.readUtf8(),
        )
        assertNull(store.session)
    }

    @Test
    fun `an upload that can no longer be resumed is aborted and started again`() = runTest {
        store.session = UploadSession("v0", uploadFingerprint(file, "Match", null))
        api.enqueue(video("v0", VideoDto.STATUS_UPLOADING))
        api.enqueue(json(409, """{"data":null,"error":{"code":"conflict","message":"not open"},"meta":{}}"""))
        api.enqueue(MockResponse().setResponseCode(204))
        api.enqueue(init("v1"))
        storage.enqueue(MockResponse().setResponseCode(200).setHeader("ETag", "\"e1\""))
        api.enqueue(video("v1", VideoDto.STATUS_PROCESSING))

        val result = uploader.upload(file, "Match", null) { _, _ -> }

        assertEquals("v1", (result as ApiResult.Success).data.id)
        val requests = (1..api.requestCount).map { api.takeRequest().let { r -> "${r.method} ${r.path}" } }
        assertEquals(
            listOf(
                "GET /api/v1/videos/v0", "POST /api/v1/videos/uploads/v0/resume", "DELETE /api/v1/videos/uploads/v0",
                "POST /api/v1/videos/uploads", "POST /api/v1/videos/uploads/v1/complete",
            ),
            requests,
        )
    }

    @Test
    fun `a video no longer readable stops with a clear code and aborts the upload`() = runTest {
        for (lose in listOf<(String) -> Unit>({ media.revoked += it }, { media.deleted += it })) {
            media.revoked.clear()
            media.deleted.clear()
            lose(file.uri)
            api.enqueue(init("v1"))
            api.enqueue(MockResponse().setResponseCode(204))

            val result = uploader.upload(file, "Match", null) { _, _ -> }

            assertEquals(ApiErrorCodes.MEDIA_UNREADABLE, (result as ApiResult.Failure).code)
            assertEquals("POST /api/v1/videos/uploads", api.takeRequest().let { "${it.method} ${it.path}" })
            assertEquals("DELETE /api/v1/videos/uploads/v1", api.takeRequest().let { "${it.method} ${it.path}" })
            assertNull(store.session)
        }
    }
}

