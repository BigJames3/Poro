package com.poro.app.core.network

import kotlinx.coroutines.test.runTest
import kotlinx.serialization.Serializable
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import okhttp3.mockwebserver.SocketPolicy
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import retrofit2.Response
import retrofit2.Retrofit
import retrofit2.converter.kotlinx.serialization.asConverterFactory
import retrofit2.http.GET

class ApiResultTest {

    @Serializable
    data class Item(val id: String)

    interface TestApi {
        @GET("item")
        suspend fun item(): Response<ApiEnvelope<Item>>

        @GET("empty")
        suspend fun empty(): Response<ApiEnvelope<Unit>>
    }

    private lateinit var server: MockWebServer
    private lateinit var api: TestApi

    @Before
    fun setUp() {
        server = MockWebServer()
        val client = OkHttpClient.Builder().addInterceptor(RequestIdInterceptor()).build()
        api = Retrofit.Builder()
            .baseUrl(server.url("/"))
            .client(client)
            .addConverterFactory(PoroJson.asConverterFactory("application/json".toMediaType()))
            .build()
            .create(TestApi::class.java)
    }

    @After
    fun tearDown() {
        server.shutdown()
    }

    @Test
    fun `success unwraps data and ignores unknown fields`() = runTest {
        server.enqueue(json(200, """{"data":{"id":"v1","extra":true},"error":null,"meta":{"request_id":"r"}}"""))

        assertEquals(ApiResult.Success(Item("v1")), apiCall { api.item() })
    }

    @Test
    fun `error envelope exposes the stable code, request id and Retry-After`() = runTest {
        server.enqueue(
            json(429, """{"data":null,"error":{"code":"otp_throttled","message":"slow down"},"meta":{"request_id":"req-42"}}""")
                .setHeader("Retry-After", "60"),
        )

        val result = apiCall { api.item() } as ApiResult.Failure
        assertEquals(429, result.httpStatus)
        assertEquals("otp_throttled", result.code)
        assertEquals("req-42", result.requestId)
        assertEquals(60L, result.retryAfterSeconds)
    }

    @Test
    fun `non envelope error is reported as unexpected_response with the client request id`() = runTest {
        server.enqueue(MockResponse().setResponseCode(502).setBody("<html>Bad gateway</html>"))

        val result = apiCall { api.item() } as ApiResult.Failure
        assertEquals(ApiErrorCodes.UNEXPECTED_RESPONSE, result.code)
        assertNotNull("le X-Request-ID envoyé sert de référence", result.requestId)
    }

    @Test
    fun `204 without body succeeds when the caller expects no data`() = runTest {
        server.enqueue(MockResponse().setResponseCode(204))

        assertEquals(ApiResult.Success(Unit), apiCall(ifEmpty = { Unit }) { api.empty() })
    }

    @Test
    fun `network failure becomes NetworkError`() = runTest {
        server.enqueue(MockResponse().setSocketPolicy(SocketPolicy.DISCONNECT_AT_START))

        assertTrue(apiCall { api.item() } is ApiResult.NetworkError)
    }

    @Test
    fun `every request carries a UUID X-Request-ID`() = runTest {
        server.enqueue(json(200, """{"data":{"id":"v1"},"error":null,"meta":{}}"""))
        apiCall { api.item() }

        val id = server.takeRequest().getHeader(RequestIdInterceptor.HEADER)
        assertNotNull(id)
        java.util.UUID.fromString(id) // lève une exception si ce n'est pas un UUID
    }

    @Test
    fun `retry delay follows Retry-After, else backs off exponentially up to a minute`() {
        fun failure(status: Int, retryAfter: Long?) = ApiResult.Failure(status, ApiErrorCodes.RATE_LIMITED, "", null, retryAfter)

        assertEquals(42L, failure(429, 42).retryDelaySeconds(consecutive = 3))
        assertEquals(listOf(5L, 10L, 20L, 40L, 60L, 60L), (1..6).map { failure(429, null).retryDelaySeconds(it) })
        assertNull("pas d'attente imposée hors 429", failure(503, 30).retryDelaySeconds(1))
    }

    private fun json(code: Int, body: String) =
        MockResponse().setResponseCode(code).setHeader("Content-Type", "application/json").setBody(body)
}
