package com.poro.app.data.user

import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PoroJson
import com.poro.app.core.network.PresignedUploader
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

class ProfileRepositoryTest {

    private lateinit var api: MockWebServer
    private lateinit var storage: MockWebServer
    private lateinit var repo: DefaultProfileRepository

    @Before
    fun setUp() {
        api = MockWebServer()
        storage = MockWebServer()
        val userApi = Retrofit.Builder()
            .baseUrl(api.url("/"))
            .client(OkHttpClient())
            .addConverterFactory(PoroJson.asConverterFactory("application/json".toMediaType()))
            .build()
            .create(UserApi::class.java)
        repo = DefaultProfileRepository(userApi, PresignedUploader(OkHttpClient()))
    }

    @After
    fun tearDown() {
        api.shutdown()
        storage.shutdown()
    }

    @Test
    fun `update sends every field and clears blank ones with null`() = runTest {
        api.enqueue(json(200, PROFILE))

        val result = repo.update(username = "awa.kone", displayName = "  ", bio = " Danseuse ")

        assertTrue(result is ApiResult.Success)
        val request = api.takeRequest()
        assertEquals("PATCH", request.method)
        assertEquals("""{"username":"awa.kone","display_name":null,"bio":"Danseuse"}""", request.body.readUtf8())
    }

    @Test
    fun `an empty username is left unchanged`() = runTest {
        api.enqueue(json(200, PROFILE))

        repo.update(username = null, displayName = "Awa", bio = "")

        assertEquals("""{"display_name":"Awa","bio":null}""", api.takeRequest().body.readUtf8())
    }

    @Test
    fun `avatar upload posts the presigned form then confirms the key`() = runTest {
        api.enqueue(
            json(
                201,
                """{"data":{"upload_url":"${storage.url("/avatars")}","fields":{"key":"tmp/abc","policy":"p"},
                "upload_key":"tmp/abc","max_bytes":5242880,"expires_in":300},"error":null,"meta":{}}""",
            ),
        )
        storage.enqueue(MockResponse().setResponseCode(204))
        api.enqueue(json(200, PROFILE.replace("\"avatar_url\":null", "\"avatar_url\":\"https://cdn/a.webp\"")))

        val result = repo.uploadAvatar(byteArrayOf(1, 2, 3), "image/png") as ApiResult.Success

        assertEquals("https://cdn/a.webp", result.data.avatarUrl)
        assertEquals("""{"content_type":"image/png"}""", api.takeRequest().body.readUtf8())
        val form = storage.takeRequest()
        assertNull("jamais de jeton vers le stockage", form.getHeader("Authorization"))
        val body = form.body.readUtf8()
        assertTrue(body.contains("name=\"key\"") && body.contains("tmp/abc") && body.contains("name=\"file\""))
        assertEquals("""{"upload_key":"tmp/abc"}""", api.takeRequest().body.readUtf8())
    }

    @Test
    fun `a storage refusal stops before the confirmation`() = runTest {
        api.enqueue(
            json(201, """{"data":{"upload_url":"${storage.url("/avatars")}","fields":{},"upload_key":"k"},"error":null,"meta":{}}"""),
        )
        storage.enqueue(MockResponse().setResponseCode(400))

        val result = repo.uploadAvatar(byteArrayOf(1), "image/jpeg") as ApiResult.Failure

        assertEquals(ApiErrorCodes.UPLOAD_REJECTED, result.code)
        assertEquals(1, api.requestCount)
    }

    private fun json(code: Int, body: String) =
        MockResponse().setResponseCode(code).setHeader("Content-Type", "application/json").setBody(body)

    private companion object {
        const val PROFILE = """{"data":{"user_id":"u1","username":"awa.kone","display_name":null,"bio":"Danseuse",
            "avatar_url":null,"is_creator":false,"creator_since":null},"error":null,"meta":{}}"""
    }
}
