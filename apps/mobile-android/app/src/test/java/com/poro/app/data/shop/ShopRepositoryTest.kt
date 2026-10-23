package com.poro.app.data.shop

import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PoroJson
import com.poro.app.core.network.PresignedUploader
import kotlinx.coroutines.test.runTest
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.int
import kotlinx.serialization.json.jsonArray
import kotlinx.serialization.json.jsonObject
import kotlinx.serialization.json.jsonPrimitive
import kotlinx.serialization.json.long
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.mockwebserver.MockResponse
import okhttp3.mockwebserver.MockWebServer
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import retrofit2.Retrofit
import retrofit2.converter.kotlinx.serialization.asConverterFactory

class ShopRepositoryTest {

    private lateinit var server: MockWebServer
    private lateinit var repo: DefaultShopRepository

    @Before
    fun setUp() {
        server = MockWebServer()
        val api = Retrofit.Builder()
            .baseUrl(server.url("/"))
            .client(OkHttpClient())
            .addConverterFactory(PoroJson.asConverterFactory("application/json".toMediaType()))
            .build()
            .create(ShopApi::class.java)
        repo = DefaultShopRepository(api, PresignedUploader(OkHttpClient()))
    }

    @After
    fun tearDown() {
        server.shutdown()
    }

    @Test
    fun `a shop is looked up by its lowercase handle without the at sign`() = runTest {
        server.enqueue(json("""{"data":{"shop_id":"s1","handle":"kofi.store","name":"Kofi","currency":"XOF"},"error":null,"meta":{}}"""))

        val shop = (repo.shop("@Kofi.Store") as ApiResult.Success).data

        assertEquals("XOF", shop.currency)
        assertEquals("/api/v1/shops/kofi.store", server.takeRequest().path)
    }

    @Test
    fun `deleting the logo hits DELETE shops me logo`() = runTest {
        server.enqueue(json("""{"data":{"shop_id":"s1","handle":"kofi.store","name":"Kofi","currency":"XOF"},"error":null,"meta":{}}"""))
        repo.deleteLogo()
        val request = server.takeRequest()
        assertEquals("DELETE", request.method)
        assertEquals("/api/v1/shops/me/logo", request.path)
    }

    @Test
    fun `updating a product sends the initial stock of new variants only and clears an empty description`() = runTest {
        server.enqueue(json(PRODUCT))

        repo.updateProduct(
            "p1",
            " Pagne ",
            "  ",
            listOf(VariantInput("v1", "M", 5000, stock = 3), VariantInput(null, "L", 6000, stock = 2)),
        )

        val request = server.takeRequest()
        assertEquals("PATCH", request.method)
        assertEquals("/api/v1/shops/me/products/p1", request.path)
        val body = PoroJson.parseToJsonElement(request.body.readUtf8()).jsonObject
        assertEquals("Pagne", body.getValue("title").jsonPrimitive.content)
        assertEquals(JsonNull, body.getValue("description"))
        val (existing, added) = body.getValue("variants").jsonArray.map { it.jsonObject }
        assertEquals("v1", existing.getValue("variant_id").jsonPrimitive.content)
        assertFalse("stock" in existing)
        assertEquals(6000L, added.getValue("price").jsonPrimitive.long)
        assertEquals(2, added.getValue("stock").jsonPrimitive.int)
        assertFalse("variant_id" in added)
    }

    @Test
    fun `stock and status have their own requests and deleting accepts an empty answer`() = runTest {
        server.enqueue(json(PRODUCT))
        server.enqueue(json(PRODUCT))
        server.enqueue(MockResponse().setResponseCode(204))

        repo.setStock("p1", "v1", 7)
        repo.setProductStatus("p1", ProductDto.ACTIVE)
        val deleted = repo.deleteProduct("p1")

        val stock = server.takeRequest()
        assertEquals("/api/v1/shops/me/products/p1/variants/v1/stock", stock.path)
        assertEquals("""{"stock_on_hand":7}""", stock.body.readUtf8())
        assertEquals("""{"status":"active"}""", server.takeRequest().body.readUtf8())
        assertEquals("DELETE", server.takeRequest().method)
        assertTrue(deleted is ApiResult.Success)
    }

    private fun json(body: String) = MockResponse().setHeader("Content-Type", "application/json").setBody(body)

    private companion object {
        const val PRODUCT = """{"data":{"product_id":"p1","title":"Pagne","currency":"XOF","variants":[]},"error":null,"meta":{}}"""
    }
}
