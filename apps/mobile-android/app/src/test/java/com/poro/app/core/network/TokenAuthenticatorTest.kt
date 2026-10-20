package com.poro.app.core.network

import com.poro.app.core.security.InMemoryTokenStore
import com.poro.app.core.security.SessionTokens
import com.poro.app.data.auth.RefreshApi
import java.util.concurrent.CountDownLatch
import java.util.concurrent.Executors
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.mockwebserver.Dispatcher
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import okhttp3.mockwebserver.RecordedRequest
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import retrofit2.Retrofit
import retrofit2.converter.kotlinx.serialization.asConverterFactory

class TokenAuthenticatorTest {

    private lateinit var server: MockWebServer
    private val refreshCalls = AtomicInteger()
    private val expired = AtomicInteger()

    @Before
    fun setUp() {
        server = MockWebServer()
    }

    @After
    fun tearDown() {
        server.shutdown()
    }

    private fun client(store: InMemoryTokenStore): OkHttpClient {
        val base = OkHttpClient.Builder().addInterceptor(RequestIdInterceptor()).build()
        val refreshApi = Retrofit.Builder()
            .baseUrl(server.url("/"))
            .client(base)
            .addConverterFactory(PoroJson.asConverterFactory("application/json".toMediaType()))
            .build()
            .create(RefreshApi::class.java)
        return base.newBuilder()
            .addInterceptor(AuthInterceptor(store))
            .authenticator(TokenAuthenticator(store, { refreshApi }, { expired.incrementAndGet() }))
            .build()
    }

    /** /refresh rend « new-access/new-refresh » ; /me n'accepte que « new-access ». */
    private fun rotatingBackend(refreshDelayMs: Long = 0) = object : Dispatcher() {
        override fun dispatch(request: RecordedRequest): MockResponse = when (request.path) {
            "/api/v1/auth/refresh" -> {
                refreshCalls.incrementAndGet()
                Thread.sleep(refreshDelayMs)
                json(200, """{"data":{"access_token":"new-access","refresh_token":"new-refresh","expires_in":900},"error":null,"meta":{}}""")
            }
            else -> if (request.getHeader("Authorization") == "Bearer new-access") {
                json(200, """{"data":{"id":"u1"},"error":null,"meta":{}}""")
            } else {
                json(401, """{"data":null,"error":{"code":"unauthorized","message":"expired"},"meta":{}}""")
            }
        }
    }

    @Test
    fun `refreshes once on 401 and replays the request with the new token`() {
        server.dispatcher = rotatingBackend()
        val store = InMemoryTokenStore(SessionTokens("old-access", "old-refresh"))

        client(store).newCall(Request.Builder().url(server.url("/api/v1/auth/me")).build()).execute().use {
            assertEquals(200, it.code)
        }

        assertEquals(1, refreshCalls.get())
        assertEquals(SessionTokens("new-access", "new-refresh"), store.current())
        assertEquals(0, expired.get())
    }

    @Test
    fun `concurrent 401s trigger a single refresh`() {
        server.dispatcher = rotatingBackend(refreshDelayMs = 200)
        val store = InMemoryTokenStore(SessionTokens("old-access", "old-refresh"))
        val http = client(store)
        val pool = Executors.newFixedThreadPool(8)
        val start = CountDownLatch(1)
        val codes = (1..8).map {
            pool.submit<Int> {
                start.await()
                http.newCall(Request.Builder().url(server.url("/api/v1/auth/me")).build()).execute().use { r -> r.code }
            }
        }
        start.countDown()
        val results = codes.map { it.get(10, TimeUnit.SECONDS) }
        pool.shutdown()

        assertTrue("toutes les requêtes aboutissent : $results", results.all { it == 200 })
        assertEquals("un seul rafraîchissement : rejouer un refresh token révoquerait la session", 1, refreshCalls.get())
    }

    @Test
    fun `revoked session clears tokens and reports expiry`() {
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse = when (request.path) {
                "/api/v1/auth/refresh" -> json(401, """{"data":null,"error":{"code":"session_revoked","message":"revoked"},"meta":{}}""")
                else -> json(401, """{"data":null,"error":{"code":"unauthorized","message":"expired"},"meta":{}}""")
            }
        }
        val store = InMemoryTokenStore(SessionTokens("old-access", "old-refresh"))

        client(store).newCall(Request.Builder().url(server.url("/api/v1/auth/me")).build()).execute().use {
            assertEquals(401, it.code)
        }

        assertNull(store.current())
        assertEquals(1, expired.get())
    }

    @Test
    fun `network failure during refresh keeps the session`() {
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse = when (request.path) {
                "/api/v1/auth/refresh" -> MockResponse().setSocketPolicy(okhttp3.mockwebserver.SocketPolicy.DISCONNECT_AFTER_REQUEST)
                else -> json(401, """{"data":null,"error":{"code":"unauthorized","message":"expired"},"meta":{}}""")
            }
        }
        val tokens = SessionTokens("old-access", "old-refresh")
        val store = InMemoryTokenStore(tokens)

        client(store).newCall(Request.Builder().url(server.url("/api/v1/auth/me")).build()).execute().use {
            assertEquals(401, it.code)
        }

        assertEquals("la session peut encore être valide : jetons conservés", tokens, store.current())
        assertEquals(0, expired.get())
    }

    @Test
    fun `unreadable refresh response keeps the session without crashing`() {
        server.dispatcher = object : Dispatcher() {
            override fun dispatch(request: RecordedRequest): MockResponse = when (request.path) {
                "/api/v1/auth/refresh" -> MockResponse().setResponseCode(200).setBody("<html>proxy</html>")
                else -> json(401, """{"data":null,"error":{"code":"unauthorized","message":"expired"},"meta":{}}""")
            }
        }
        val tokens = SessionTokens("old-access", "old-refresh")
        val store = InMemoryTokenStore(tokens)

        client(store).newCall(Request.Builder().url(server.url("/api/v1/auth/me")).build()).execute().use {
            assertEquals(401, it.code)
        }

        assertEquals(tokens, store.current())
        assertEquals(0, expired.get())
    }

    @Test
    fun `anonymous request is not refreshed`() {
        server.dispatcher = rotatingBackend()
        val store = InMemoryTokenStore()

        client(store).newCall(Request.Builder().url(server.url("/api/v1/auth/me")).build()).execute().use {
            assertEquals(401, it.code)
        }
        assertEquals(0, refreshCalls.get())
    }

    private fun json(code: Int, body: String) =
        MockResponse().setResponseCode(code).setHeader("Content-Type", "application/json").setBody(body)
}
