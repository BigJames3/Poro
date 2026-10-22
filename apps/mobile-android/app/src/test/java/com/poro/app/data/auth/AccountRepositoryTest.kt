package com.poro.app.data.auth

import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PoroJson
import com.poro.app.core.security.InMemoryTokenStore
import com.poro.app.core.security.SessionTokens
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.test.runTest
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Before
import org.junit.Test
import retrofit2.Retrofit
import retrofit2.converter.kotlinx.serialization.asConverterFactory

class AccountRepositoryTest {

    private lateinit var server: MockWebServer
    private val tokens = InMemoryTokenStore(SessionTokens("access", "refresh"))
    private lateinit var repo: DefaultAccountRepository

    @Before
    fun setUp() {
        server = MockWebServer()
        val api = Retrofit.Builder()
            .baseUrl(server.url("/"))
            .client(OkHttpClient())
            .addConverterFactory(PoroJson.asConverterFactory("application/json".toMediaType()))
            .build()
            .create(AuthApi::class.java)
        repo = DefaultAccountRepository(api, tokens, io = Dispatchers.Unconfined)
    }

    @After
    fun tearDown() {
        server.shutdown()
    }

    @Test
    fun `a scheduled deletion signs this device out`() = runTest {
        server.enqueue(json(202, """{"data":{"deletion_scheduled_at":"2026-11-20T08:00:00Z"},"error":null,"meta":{}}"""))

        val result = repo.deleteAccount(password = null, otpCode = "123456") as ApiResult.Success

        assertEquals("2026-11-20T08:00:00Z", result.data.deletionScheduledAt)
        assertEquals("""{"otp_code":"123456"}""", server.takeRequest().body.readUtf8())
        assertNull(tokens.current())
    }

    @Test
    fun `a refused proof keeps the session`() = runTest {
        server.enqueue(json(401, """{"data":null,"error":{"code":"invalid_credentials","message":"m"},"meta":{}}"""))

        val result = repo.deleteAccount(password = "bad", otpCode = null) as ApiResult.Failure

        assertEquals(ApiErrorCodes.INVALID_CREDENTIALS, result.code)
        assertNotNull(tokens.current())
    }

    @Test
    fun `sessions are listed and revoked`() = runTest {
        server.enqueue(
            json(
                200,
                """{"data":[{"id":"s1","device":"Tecno Spark 20","ip_address":"1.2.3.4","created_at":"2026-10-01T10:00:00Z",
                "last_active_at":"2026-10-20T10:00:00Z","current":true},{"id":"s2","device":null,"ip_address":null,
                "created_at":"2026-09-01T10:00:00Z","last_active_at":"2026-09-02T10:00:00Z","current":false}],"error":null,"meta":{}}""",
            ),
        )
        server.enqueue(MockResponse().setResponseCode(204))

        val sessions = (repo.sessions() as ApiResult.Success).data
        assertEquals(listOf(true, false), sessions.map { it.current })
        assertEquals(ApiResult.Success(Unit), repo.revokeSession("s2"))

        server.takeRequest()
        val revoke = server.takeRequest()
        assertEquals("DELETE", revoke.method)
        assertEquals("/api/v1/auth/sessions/s2", revoke.path)
    }

    private fun json(code: Int, body: String) =
        MockResponse().setResponseCode(code).setHeader("Content-Type", "application/json").setBody(body)
}
