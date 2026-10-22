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
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import retrofit2.Retrofit
import retrofit2.converter.kotlinx.serialization.asConverterFactory

class VideoUploaderTest {

    private lateinit var api: MockWebServer
    private lateinit var storage: MockWebServer
    private val media = FakeMediaReader()
    private lateinit var uploader: VideoUploader

    /** 10 octets en parties de 4 : trois parties (4, 4, 2). */
    private val file = LocalMedia("content://video", "video/mp4", 10, durationMs = 42_000)

    @Before
    fun setUp() {
        api = MockWebServer()
        storage = MockWebServer()
        media.add(file.uri, file.mimeType, ByteArray(10) { it.toByte() })
        val videoApi = Retrofit.Builder()
            .baseUrl(api.url("/"))
            .client(OkHttpClient())
            .addConverterFactory(PoroJson.asConverterFactory("application/json".toMediaType()))
            .build()
            .create(VideoApi::class.java)
        uploader = VideoUploader(videoApi, PresignedUploader(OkHttpClient()), media, retryDelaysMs = listOf(0, 0))
    }

    @After
    fun tearDown() {
        api.shutdown()
        storage.shutdown()
    }

    private fun initResponse() = json(
        201,
        """{"data":{"video_id":"v1","upload_id":"u","key":"k","part_size":4,"parts":[
        {"part_number":1,"url":"${storage.url("/p1")}"},{"part_number":2,"url":"${storage.url("/p2")}"},
        {"part_number":3,"url":"${storage.url("/p3")}"}]},"error":null,"meta":{}}""",
    )

    private fun part(etag: String) = MockResponse().setResponseCode(200).setHeader("ETag", etag)

    @Test
    fun `parts are sent in order and assembled with their ETags`() = runTest {
        api.enqueue(initResponse())
        storage.enqueue(part("\"e1\""))
        storage.enqueue(part("\"e2\""))
        storage.enqueue(part("\"e3\""))
        api.enqueue(json(200, """{"data":{"id":"v1","status":"processing","moderation_status":"approved"},"error":null,"meta":{}}"""))
        val progress = mutableListOf<Long>()

        val result = uploader.upload(file, " Match du dimanche ", "", onProgress = { sent, _ -> progress += sent })

        assertEquals(VideoDto.STATUS_PROCESSING, (result as ApiResult.Success).data.status)
        assertEquals(listOf(4L, 8L, 10L), progress)
        assertEquals(
            """{"title":"Match du dimanche","content_type":"video/mp4","size_bytes":10}""",
            api.takeRequest().body.readUtf8(),
        )
        val sizes = (1..3).map { storage.takeRequest() }.map { it.bodySize }
        assertEquals(listOf(4L, 4L, 2L), sizes)
        val complete = api.takeRequest()
        assertEquals("/api/v1/videos/uploads/v1/complete", complete.path)
        assertEquals(
            """{"parts":[{"part_number":1,"etag":"\"e1\""},{"part_number":2,"etag":"\"e2\""},{"part_number":3,"etag":"\"e3\""}]}""",
            complete.body.readUtf8(),
        )
    }

    @Test
    fun `a failing part is retried alone`() = runTest {
        api.enqueue(initResponse())
        storage.enqueue(part("\"e1\""))
        storage.enqueue(MockResponse().setResponseCode(503))
        storage.enqueue(MockResponse().setResponseCode(500))
        storage.enqueue(part("\"e2\""))
        storage.enqueue(part("\"e3\""))
        api.enqueue(json(200, """{"data":{"id":"v1","status":"processing"},"error":null,"meta":{}}"""))

        val result = uploader.upload(file, "Titre", null) { _, _ -> }

        assertTrue(result is ApiResult.Success)
        val paths = (1..storage.requestCount).map { storage.takeRequest().path }
        assertEquals("partie 2 envoyée trois fois, les autres une fois", listOf("/p1", "/p2", "/p2", "/p2", "/p3"), paths)
    }

    @Test
    fun `a refused part aborts the upload on the server`() = runTest {
        api.enqueue(initResponse())
        storage.enqueue(part("\"e1\""))
        storage.enqueue(MockResponse().setResponseCode(403))
        api.enqueue(MockResponse().setResponseCode(204))

        val result = uploader.upload(file, "Titre", null) { _, _ -> }

        assertEquals(ApiErrorCodes.UPLOAD_REJECTED, (result as ApiResult.Failure).code)
        api.takeRequest()
        val abort = api.takeRequest()
        assertEquals("DELETE", abort.method)
        assertEquals("/api/v1/videos/uploads/v1", abort.path)
        assertNull("aucune requête vers l'API ne porte de jeton ici", abort.getHeader("Authorization"))
    }

    @Test
    fun `an init refusal stops before any part`() = runTest {
        api.enqueue(json(422, """{"data":null,"error":{"code":"size_invalid","message":"m"},"meta":{}}"""))

        val result = uploader.upload(file, "Titre", null) { _, _ -> }

        assertEquals(ApiErrorCodes.SIZE_INVALID, (result as ApiResult.Failure).code)
        assertEquals(0, storage.requestCount)
        assertEquals(1, api.requestCount)
    }

    private fun json(code: Int, body: String) =
        MockResponse().setResponseCode(code).setHeader("Content-Type", "application/json").setBody(body)
}
