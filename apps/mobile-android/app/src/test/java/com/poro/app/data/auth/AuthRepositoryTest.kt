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

    @Test
    fun `register sends the account fields and saves the session`() = runTest {
        server.enqueue(json(201, SIGNED_IN))
        val store = InMemoryTokenStore()

        val result = repository(store).register("awa@poro.africa", "s3cret-pass", "Awa Koné")

        assertTrue(result is ApiResult.Success)
        assertEquals(SessionTokens("access-1", "refresh-1"), store.current())
        val request = server.takeRequest()
        assertEquals("/api/v1/auth/email/register", request.path)
        assertEquals(
            """{"email":"awa@poro.africa","password":"s3cret-pass","full_name":"Awa Koné","device_name":"Tecno Spark 20"}""",
            request.body.readUtf8(),
        )
    }

    @Test
    fun `login saves the session, wrong credentials and lockouts do not`() = runTest {
        server.enqueue(error(401, ApiErrorCodes.INVALID_CREDENTIALS))
        server.enqueue(error(429, ApiErrorCodes.LOGIN_THROTTLED).setHeader("Retry-After", "120"))
        server.enqueue(json(200, SIGNED_IN))
        val store = InMemoryTokenStore()
        val repo = repository(store)

        assertEquals(ApiErrorCodes.INVALID_CREDENTIALS, (repo.login("awa@poro.africa", "bad") as ApiResult.Failure).code)
        val locked = repo.login("awa@poro.africa", "bad") as ApiResult.Failure
        assertEquals(ApiErrorCodes.LOGIN_THROTTLED, locked.code)
        assertEquals(120L, locked.retryAfterSeconds)
        assertNull(store.current())

        assertTrue(repo.login("awa@poro.africa", "good") is ApiResult.Success)
        assertEquals(SessionTokens("access-1", "refresh-1"), store.current())
        assertEquals("/api/v1/auth/email/login", server.takeRequest().path)
    }

    @Test
    fun `email verification is requested and confirmed with the session token`() = runTest {
        server.enqueue(json(202, """{"data":{"expires_in":900},"error":null,"meta":{}}"""))
        server.enqueue(json(200, """{"data":{"id":"u1","email":"awa@poro.africa","email_verified":true},"error":null,"meta":{}}"""))
        val repo = repository(InMemoryTokenStore(SessionTokens("access", "refresh")))

        assertEquals(ApiResult.Success(CodeSent(expiresIn = 900)), repo.requestEmailVerification())
        val confirmed = repo.confirmEmailVerification("123456") as ApiResult.Success

        assertTrue(confirmed.data.emailVerified)
        assertEquals("/api/v1/auth/email/verification/request", server.takeRequest().path)
        val confirm = server.takeRequest()
        assertEquals("/api/v1/auth/email/verification/confirm", confirm.path)
        assertEquals("""{"code":"123456"}""", confirm.body.readUtf8())
    }

    @Test
    fun `password reset accepts 204 and reports a rejected code`() = runTest {
        server.enqueue(json(202, """{"data":{"expires_in":900},"error":null,"meta":{}}"""))
        server.enqueue(error(422, ApiErrorCodes.CODE_INVALID))
        server.enqueue(MockResponse().setResponseCode(204))
        val repo = repository(InMemoryTokenStore())

        assertTrue(repo.forgotPassword("awa@poro.africa") is ApiResult.Success)
        assertEquals(ApiErrorCodes.CODE_INVALID, (repo.resetPassword("awa@poro.africa", "000000", "new-pass-1") as ApiResult.Failure).code)
        assertEquals(ApiResult.Success(Unit), repo.resetPassword("awa@poro.africa", "123456", "new-pass-1"))

        assertEquals("""{"email":"awa@poro.africa"}""", server.takeRequest().body.readUtf8())
        server.takeRequest()
        val reset = server.takeRequest()
        assertEquals("/api/v1/auth/email/password/reset", reset.path)
        assertEquals("""{"email":"awa@poro.africa","code":"123456","new_password":"new-pass-1"}""", reset.body.readUtf8())
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
