package com.poro.app.data.video

import com.poro.app.core.media.LocalImagePreparer
import com.poro.app.core.media.PrepareResult
import com.poro.app.core.media.PreparedImage
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.CapabilitiesInterceptor
import com.poro.app.core.network.PoroJson
import com.poro.app.core.network.PresignedUploader
import kotlinx.coroutines.test.runTest
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import retrofit2.Retrofit
import retrofit2.converter.kotlinx.serialization.asConverterFactory

class PhotoUploaderTest {

    private lateinit var api: MockWebServer
    private lateinit var storage: MockWebServer
    private lateinit var uploader: PhotoUploader
    private val sessions = object : UploadSessionStore {
        var session: UploadSession? = null
        override fun load() = session
        override fun save(session: UploadSession) { this.session = session }
        override fun clear() { session = null }
    }

    /** Fichiers préparés en mémoire : leur contenu est leur position répétée. */
    private val files = object : LocalImagePreparer {
        override suspend fun prepare(uris: List<String>) = PrepareResult.Ready(emptyList())
        override fun read(image: PreparedImage) = ByteArray(image.sizeBytes.toInt()) { image.path.last().code.toByte() }
        override fun discard(images: List<PreparedImage>) = Unit
    }
    private val images = listOf(PreparedImage("/p/1", 3), PreparedImage("/p/2", 5))

    @Before
    fun setUp() {
        api = MockWebServer()
        storage = MockWebServer()
        val videoApi = Retrofit.Builder()
            .baseUrl(api.url("/"))
            .client(OkHttpClient.Builder().addInterceptor(CapabilitiesInterceptor()).build())
            .addConverterFactory(PoroJson.asConverterFactory("application/json".toMediaType()))
            .build()
            .create(VideoApi::class.java)
        uploader = PhotoUploader(videoApi, PresignedUploader(OkHttpClient()), files, retryDelaysMs = listOf(0), sessions = sessions)
    }

    @After
    fun tearDown() {
        api.shutdown()
        storage.shutdown()
    }

    private fun json(code: Int, body: String) = MockResponse().setResponseCode(code).setHeader("Content-Type", "application/json").setBody(body)

    private fun initResponse() = json(
        201,
        """{"data":{"video_id":"p1","expires_in":3600,"images":[
        {"position":1,"url":"${storage.url("/i1")}","headers":{"Content-Type":"image/jpeg","Content-Length":"3"}},
        {"position":2,"url":"${storage.url("/i2")}","headers":{"Content-Type":"image/jpeg","Content-Length":"5"}}]},"error":null,"meta":{}}""",
    )

    private val processing = """{"data":{"id":"p1","media_type":"photo","status":"processing","moderation_status":"approved"},"error":null,"meta":{}}"""

    @Test
    fun `each image is put with its signed headers, then the post is completed`() = runTest {
        api.enqueue(initResponse())
        storage.enqueue(MockResponse().setResponseCode(200))
        storage.enqueue(MockResponse().setResponseCode(200))
        api.enqueue(json(200, processing))
        val progress = mutableListOf<Long>()

        val result = uploader.upload(images, " Marché #abidjan ") { sent, _ -> progress += sent }

        assertTrue((result as ApiResult.Success).data.isPhoto)
        assertEquals(listOf(3L, 8L), progress)
        val init = api.takeRequest()
        assertEquals("/api/v1/videos/photo-uploads", init.path)
        assertEquals("photo-posts", init.getHeader("Poro-Capabilities"))
        assertEquals(
            """{"images":[{"content_type":"image/jpeg","size_bytes":3},{"content_type":"image/jpeg","size_bytes":5}],"description":"Marché #abidjan"}""",
            init.body.readUtf8(),
        )
        val puts = (1..2).map { storage.takeRequest() }
        assertEquals(listOf("PUT", "PUT"), puts.map { it.method })
        assertEquals(listOf(3L, 5L), puts.map { it.bodySize })
        assertEquals(listOf("image/jpeg", "image/jpeg"), puts.map { it.getHeader("Content-Type") })
        assertEquals("/api/v1/videos/photo-uploads/p1/complete", api.takeRequest().path)
        assertEquals(null, sessions.session)
    }

    @Test
    fun `a refused image aborts the post and publishes nothing`() = runTest {
        api.enqueue(initResponse())
        storage.enqueue(MockResponse().setResponseCode(403))
        api.enqueue(MockResponse().setResponseCode(204))

        val result = uploader.upload(images, null) { _, _ -> }

        assertEquals(ApiErrorCodes.UPLOAD_REJECTED, (result as ApiResult.Failure).code)
        api.takeRequest()
        val abort = api.takeRequest()
        assertEquals("DELETE", abort.method)
        assertEquals("/api/v1/videos/uploads/p1", abort.path)
        assertEquals(null, sessions.session)
    }

    @Test
    fun `a lost complete answer is settled by reading the post, never by publishing again`() = runTest {
        api.enqueue(initResponse())
        storage.enqueue(MockResponse().setResponseCode(200))
        storage.enqueue(MockResponse().setResponseCode(200))
        api.enqueue(json(503, """{"data":null,"error":{"code":"unavailable","message":"x"},"meta":{}}"""))

        val first = uploader.upload(images, null) { _, _ -> }
        assertEquals(503, (first as ApiResult.Failure).httpStatus)
        assertEquals("the session is kept when the outcome is unknown", "p1", sessions.session?.videoId)

        api.enqueue(json(200, processing))
        val second = uploader.upload(images, null) { _, _ -> }

        assertTrue(second is ApiResult.Success)
        repeat(2) { api.takeRequest() }
        val read = api.takeRequest()
        assertEquals("GET", read.method)
        assertEquals("/api/v1/videos/p1", read.path)
        assertEquals("no second post was created", 3, api.requestCount)
    }
}
