package com.poro.app.data.social

import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PoroJson
import com.poro.app.core.network.PresignedUploader
import com.poro.app.data.user.DefaultProfileRepository
import com.poro.app.data.user.UserApi
import kotlinx.coroutines.test.runTest
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Before
import org.junit.Test
import retrofit2.Retrofit
import retrofit2.converter.kotlinx.serialization.asConverterFactory

/** Lectures groupées : découpage aux limites du contrat (50 vidéos, 100 comptes) et décodage. */
class SocialBatchTest {

    private lateinit var server: MockWebServer
    private lateinit var retrofit: Retrofit

    @Before
    fun setUp() {
        server = MockWebServer()
        retrofit = Retrofit.Builder()
            .baseUrl(server.url("/"))
            .client(OkHttpClient())
            .addConverterFactory(PoroJson.asConverterFactory("application/json".toMediaType()))
            .build()
    }

    @After
    fun tearDown() {
        server.shutdown()
    }

    @Test
    fun `video counters are read 50 at a time`() = runTest {
        val repo = DefaultSocialRepository(retrofit.create(SocialApi::class.java))
        server.enqueue(json("""{"data":{"items":[{"video_id":"v0","likes":3,"comments":1,"shares":0,"liked_by_me":true}]},"error":null,"meta":{}}"""))
        server.enqueue(json("""{"data":{"items":[]},"error":null,"meta":{}}"""))
        val ids = List(51) { "v$it" }

        val stats = (repo.videosStats(ids + "v0") as ApiResult.Success).data

        assertEquals(1, stats.size)
        assertEquals(true, stats.single().likedByMe)
        assertEquals("/api/v1/videos/stats?ids=" + ids.take(50).joinToString("%2C"), server.takeRequest().path)
        assertEquals("/api/v1/videos/stats?ids=v50", server.takeRequest().path)
    }

    @Test
    fun `public profiles are read in one request`() = runTest {
        val repo = DefaultProfileRepository(retrofit.create(UserApi::class.java), PresignedUploader(OkHttpClient()))
        server.enqueue(
            json(
                """{"data":{"items":[{"user_id":"u2","username":"kofi","display_name":"Kofi","bio":null,
                "avatar_url":null,"is_creator":false,"creator_since":null}]},"error":null,"meta":{}}""",
            ),
        )

        val profiles = (repo.profiles(listOf("u2", "u3", "u2")) as ApiResult.Success).data

        assertEquals("kofi", profiles.single().username)
        assertEquals("/api/v1/users?ids=u2%2Cu3", server.takeRequest().path)
    }

    private fun json(body: String) = MockResponse().setHeader("Content-Type", "application/json").setBody(body)
}
