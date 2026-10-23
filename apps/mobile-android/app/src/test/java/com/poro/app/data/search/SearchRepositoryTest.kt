package com.poro.app.data.search

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

class SearchRepositoryTest {

    private lateinit var server: MockWebServer
    private lateinit var repo: DefaultSearchRepository

    @Before
    fun setUp() {
        server = MockWebServer()
        val api = Retrofit.Builder()
            .baseUrl(server.url("/"))
            .client(OkHttpClient())
            .addConverterFactory(PoroJson.asConverterFactory("application/json".toMediaType()))
            .build()
            .create(SearchApi::class.java)
        repo = DefaultSearchRepository(api)
    }

    @After
    fun tearDown() {
        server.shutdown()
    }

    @Test
    fun `each kind asks for its type and decodes its items`() = runTest {
        server.enqueue(json("""{"data":{"type":"users","items":[{"user_id":"u1","username":"kofi","display_name":null,
            "avatar_url":null,"is_creator":true,"followers":1200}],"next_cursor":null},"error":null,"meta":{}}"""))
        server.enqueue(json("""{"data":{"type":"hashtags","items":[{"tag":"foot","videos_count":42}],"next_cursor":"n"},"error":null,"meta":{}}"""))

        val users = (repo.users("kofi", null) as ApiResult.Success).data
        val tags = (repo.hashtags("fo", "abc") as ApiResult.Success).data

        assertEquals(1200L, users.items.single().followers)
        assertEquals("n", tags.nextCursor)
        assertEquals("/api/v1/search?type=users&q=kofi&limit=20", server.takeRequest().path)
        assertEquals("/api/v1/search?type=hashtags&q=fo&cursor=abc&limit=20", server.takeRequest().path)
    }

    @Test
    fun `products and shops are decoded with their shop`() = runTest {
        server.enqueue(json("""{"data":{"type":"products","items":[{"product_id":"p1","title":"Pagne wax","image_url":null,
            "min_price":9000,"currency":"XOF","in_stock":false,"shop":{"shop_id":"s1","name":"Pagnes Awa","handle":"pagnes.awa",
            "logo_url":null,"country_code":"CI"}}],"next_cursor":null},"error":null,"meta":{}}"""))
        server.enqueue(json("""{"data":{"type":"shops","items":[{"shop_id":"s1","name":"Pagnes Awa","handle":"pagnes.awa",
            "logo_url":null,"country_code":"CI","currency":"XOF"}],"next_cursor":null},"error":null,"meta":{}}"""))

        val product = (repo.products("pagne", null) as ApiResult.Success).data.items.single()
        val shop = (repo.shops("awa", null) as ApiResult.Success).data.items.single()

        assertEquals(9000L, product.minPrice)
        assertEquals("pagnes.awa", product.shop.handle)
        assertEquals("XOF", shop.currency)
        assertEquals("/api/v1/search?type=products&q=pagne&limit=20", server.takeRequest().path)
        assertEquals("/api/v1/search?type=shops&q=awa&limit=20", server.takeRequest().path)
    }

    private fun json(body: String) = MockResponse().setHeader("Content-Type", "application/json").setBody(body)
}
