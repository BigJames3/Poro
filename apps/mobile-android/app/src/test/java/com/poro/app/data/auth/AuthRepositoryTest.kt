package com.poro.app.data.auth

import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PoroJson
import com.poro.app.core.security.InMemoryTokenStore
import com.poro.app.core.security.SessionTokens
import com.poro.app.core.security.TokenStorageException
import com.poro.app.core.security.TokenStore
import java.security.ProviderException
import kotlinx.coroutines.Dispatchers
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

class AuthRepositoryTest {

    private lateinit var server: MockWebServer
    private lateinit var api: AuthApi

    @Before
    fun setUp() {
        server = MockWebServer()
        api = Retrofit.Builder()
            .baseUrl(server.url("/"))
            .client(OkHttpClient())
            .addConverterFactory(PoroJson.asConverterFactory("application/json".toMediaType()))
            .build()
            .create(AuthApi::class.java)
    }

    @After
    fun tearDown() {
        server.shutdown()
    }

    private fun repository(store: TokenStore) =
        DefaultAuthRepository(api, store, deviceName = { "Tecno Spark 20" }, io = Dispatchers.Unconfined)

    @Test
    fun `requestOtp posts the E164 number and returns the code lifetime`() = runTest {
        server.enqueue(json(200, """{"data":{"message":"otp sent","expires_in":300},"error":null,"meta":{}}"""))

        val result = repository(InMemoryTokenStore()).requestOtp("+2250700000000")

        assertEquals(ApiResult.Success(CodeSent(expiresIn = 300)), result)
        val request = server.takeRequest()
        assertEquals("/api/v1/auth/otp/request", request.path)
        assertEquals("""{"phone":"+2250700000000"}""", request.body.readUtf8())
    }

    @Test
    fun `requestOtp exposes the stable code and Retry-After of a throttled number`() = runTest {
        server.enqueue(error(429, ApiErrorCodes.OTP_THROTTLED).setHeader("Retry-After", "42"))

        val result = repository(InMemoryTokenStore()).requestOtp("+2250700000000") as ApiResult.Failure

        assertEquals(ApiErrorCodes.OTP_THROTTLED, result.code)
        assertEquals(42L, result.retryAfterSeconds)
    }

    @Test
    fun `requestOtp keeps every documented error code`() = runTest {
        val cases = listOf(
            400 to ApiErrorCodes.INVALID_REQUEST,
            422 to ApiErrorCodes.PHONE_COUNTRY_NOT_SUPPORTED,
            422 to ApiErrorCodes.PHONE_UNREACHABLE,
            429 to ApiErrorCodes.RATE_LIMITED,
            503 to ApiErrorCodes.SMS_UNAVAILABLE,
        )
        val repo = repository(InMemoryTokenStore())
        for ((status, code) in cases) {
            server.enqueue(error(status, code))
            val result = repo.requestOtp("+2250700000000") as ApiResult.Failure
            assertEquals(status, result.httpStatus)
            assertEquals(code, result.code)
        }
    }

    @Test
    fun `verifyOtp sends the device name and saves the session`() = runTest {
        server.enqueue(json(200, SIGNED_IN))
        val store = InMemoryTokenStore()

        val result = repository(store).verifyOtp("+2250700000000", "123456")

        assertTrue(result is ApiResult.Success)
        assertEquals(true, (result as ApiResult.Success).data.deletionCancelled)
        assertEquals(SessionTokens("access-1", "refresh-1"), store.current())
        assertEquals(
            """{"phone":"+2250700000000","code":"123456","device_name":"Tecno Spark 20"}""",
            server.takeRequest().body.readUtf8(),
        )
    }

    @Test
    fun `verifyOtp errors leave the device signed out`() = runTest {
        val cases = listOf(
            401 to ApiErrorCodes.OTP_INVALID,
            401 to ApiErrorCodes.OTP_EXPIRED,
            403 to ApiErrorCodes.ACCOUNT_BLOCKED,
            429 to ApiErrorCodes.OTP_TOO_MANY_ATTEMPTS,
        )
        val store = InMemoryTokenStore()
        val repo = repository(store)
        for ((status, code) in cases) {
            server.enqueue(error(status, code))
            assertEquals(code, (repo.verifyOtp("+2250700000000", "000000") as ApiResult.Failure).code)
        }
        assertNull(store.current())
    }

    @Test
    fun `a session that cannot be stored is reported instead of crashing`() = runTest {
        server.enqueue(json(200, SIGNED_IN))
        val store = object : TokenStore by InMemoryTokenStore() {
            override fun save(tokens: SessionTokens) = throw TokenStorageException(ProviderException("keystore"))
        }

        val result = repository(store).verifyOtp("+2250700000000", "123456") as ApiResult.Failure

        assertEquals(ApiErrorCodes.STORAGE_FAILED, result.code)
        assertNull(store.current())
    }

    @Test
    fun `logout accepts 204 and clears the session`() = runTest {
        server.enqueue(MockResponse().setResponseCode(204))
        val store = InMemoryTokenStore(SessionTokens("access", "refresh"))

        repository(store).logout()

        assertEquals("/api/v1/auth/logout", server.takeRequest().path)
        assertNull(store.current())
    }

    @Test
    fun `logout clears the session even when offline`() = runTest {
        server.enqueue(MockResponse().setSocketPolicy(SocketPolicy.DISCONNECT_AT_START))
        val store = InMemoryTokenStore(SessionTokens("access", "refresh"))

        repository(store).logout()

        assertNull(store.current())
    }

    private fun json(code: Int, body: String) =
        MockResponse().setResponseCode(code).setHeader("Content-Type", "application/json").setBody(body)

    private fun error(status: Int, code: String) =
        json(status, """{"data":null,"error":{"code":"$code","message":"m"},"meta":{"request_id":"req-1"}}""")

    private companion object {
        const val SIGNED_IN = """{"data":{"access_token":"access-1","refresh_token":"refresh-1","expires_in":900,
            "token_type":"Bearer","user":{"id":"u1","phone":"+2250700000000"},"deletion_cancelled":true},"error":null,"meta":{}}"""
    }
}
