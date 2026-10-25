package com.poro.app.core.network

import kotlinx.coroutines.test.runTest
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.Assert.assertEquals
import org.junit.Test

class CapabilitiesInterceptorTest {
    @Test
    fun `every api call declares photo posts`() = runTest {
        val server = MockWebServer()
        server.enqueue(MockResponse())
        val client = OkHttpClient.Builder().addInterceptor(CapabilitiesInterceptor()).build()
        client.newCall(Request.Builder().url(server.url("/api/v1/feed/for-you")).build()).execute().close()
        assertEquals("photo-posts", server.takeRequest().getHeader("Poro-Capabilities"))
        server.shutdown()
    }
}
