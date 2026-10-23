package com.poro.app.data.feed

import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PoroJson
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

class FeedRepositoryTest {

    private lateinit var server: MockWebServer
    private lateinit var repo: DefaultFeedRepository

    @Before
    fun setUp() {
        server = MockWebServer()
        val api = Retrofit.Builder()
            .baseUrl(server.url("/"))
            .client(OkHttpClient())
            .addConverterFactory(PoroJson.asConverterFactory("application/json".toMediaType()))
            .build()
            .create(FeedApi::class.java)
        repo = DefaultFeedRepository(api)
    }

    @After
    fun tearDown() {
        server.shutdown()
    }

    @Test
    fun `feed items are decoded with author and counters`() = runTest {
        server.enqueue(
            MockResponse().setHeader("Content-Type", "application/json").setBody(
                """{"data":{"items":[{"video_id":"v1","title":"Match du dimanche","hashtags":["foot"],"duration_ms":42000,
                "published_at":"2026-10-20T10:00:00Z","thumbnail_url":"http://localhost:9000/t.jpg","hls_url":"http://localhost:9000/m.m3u8",
                "stats":{"likes":12,"comments":3,"shares":1},"author":{"id":"u1","username":null,"display_name":null,"avatar_url":null}}],
                "next_cursor":"c2"},"error":null,"meta":{}}""",
            ),
        )

        val page = (repo.page(FeedKind.ForYou, cursor = null) as ApiResult.Success).data

        assertEquals("c2", page.nextCursor)
        val item = page.items.single()
        assertEquals(12L, item.stats.likes)
        assertEquals("u1", item.author.id)
        assertEquals("/api/v1/feed/for-you?limit=10", server.takeRequest().path)
    }

    @Test
    fun `each kind has its route and passes the cursor`() = runTest {
        repeat(2) { server.enqueue(MockResponse().setHeader("Content-Type", "application/json").setBody("""{"data":{"items":[],"next_cursor":null},"error":null,"meta":{}}""")) }

        repo.page(FeedKind.Following, cursor = "abc")
        repo.page(FeedKind.Trending, cursor = null)

        assertEquals("/api/v1/feed/following?cursor=abc&limit=10", server.takeRequest().path)
        assertEquals("/api/v1/feed/trending?limit=10", server.takeRequest().path)
    }

    @Test
    fun `the guest feed reads the public trending route`() = runTest {
        server.enqueue(MockResponse().setHeader("Content-Type", "application/json").setBody("""{"data":{"items":[],"next_cursor":null},"error":null,"meta":{}}"""))

        repo.page(FeedKind.Guest, cursor = "abc")

        assertEquals("/api/v1/feed/public/trending?cursor=abc&limit=10", server.takeRequest().path)
    }
}
