package com.poro.app.data.order

import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PoroJson
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

class OrderRepositoryTest {

    private lateinit var server: MockWebServer
    private lateinit var repo: DefaultOrderRepository

    @Before
    fun setUp() {
        server = MockWebServer()
        val api = Retrofit.Builder()
            .baseUrl(server.url("/"))
            .client(OkHttpClient())
            .addConverterFactory(PoroJson.asConverterFactory("application/json".toMediaType()))
            .build()
            .create(OrderApi::class.java)
        repo = DefaultOrderRepository(api)
    }

    @After
    fun tearDown() {
        server.shutdown()
    }

    @Test
    fun `the cart is decoded per shop with its unavailable lines`() = runTest {
        server.enqueue(
            json(
                """{"data":{"shops":[{"shop":{"shop_id":"s1","name":"Kofi","handle":"kofi","currency":"XOF"},
                "items":[{"variant_id":"v1","product_id":"p1","title":"Pagne","variant_title":"M","quantity":2,
                "unit_price":5000,"line_total":10000,"in_stock":true,"available":true}],"subtotal":10000}],
                "unavailable":[{"variant_id":"v9","quantity":1,"available":false}],"item_count":3},"error":null,"meta":{}}""",
            ),
        )

        val cart = (repo.setQuantity("v1", 2) as ApiResult.Success).data

        val request = server.takeRequest()
        assertEquals("PUT", request.method)
        assertEquals("/api/v1/cart/items/v1", request.path)
        assertEquals("""{"quantity":2}""", request.body.readUtf8())
        assertEquals(10000L, cart.shops.single().subtotal)
        assertEquals("v9", cart.unavailable.single().variantId)
        assertEquals(3, cart.itemCount)
    }

    @Test
    fun `preview pays on delivery and confirm is explicit`() = runTest {
        server.enqueue(json("""{"data":{"preview_id":"pv1","totals":[{"currency":"XOF","amount":10000}]},"error":null,"meta":{}}"""))
        server.enqueue(json("""{"data":{"checkout_id":"c1","orders":[{"order_id":"o1","status":"confirmed"}]},"error":null,"meta":{}}"""))

        val preview = (repo.preview(PreviewRequest(Delivery("Awa", "+2250701020304", "Abidjan", "Cocody"))) as ApiResult.Success).data
        val placed = (repo.confirm(preview.previewId) as ApiResult.Success).data

        val previewBody = server.takeRequest().body.readUtf8()
        assertEquals(
            """{"delivery":{"full_name":"Awa","phone":"+2250701020304","city":"Abidjan","address":"Cocody"},""" +
                """"payment_method":"cash_on_delivery","save_address":false}""",
            previewBody,
        )
        assertEquals("""{"preview_id":"pv1","confirmed":true}""", server.takeRequest().body.readUtf8())
        assertEquals("o1", placed.orders.single().orderId)
    }

    @Test
    fun `clearing the cart hits DELETE cart`() = runTest {
        server.enqueue(json("""{"data":{"shops":[],"unavailable":[],"item_count":0},"error":null,"meta":{}}"""))
        val cart = (repo.clearCart() as ApiResult.Success).data
        val request = server.takeRequest()
        assertEquals("DELETE", request.method)
        assertEquals("/api/v1/cart", request.path)
        assertEquals(0, cart.itemCount)
    }

    @Test
    fun `a new address is posted with its label`() = runTest {
        server.enqueue(json("""{"data":{"address_id":"a1","label":"Maison","is_default":true,"full_name":"Awa","phone":"+2250701020304","city":"Abidjan"},"error":null,"meta":{}}"""))
        val saved = (repo.saveAddress(AddressInput("Awa", "+2250701020304", "Abidjan", "Cocody", label = "Maison"), null) as ApiResult.Success).data
        val request = server.takeRequest()
        assertEquals("POST", request.method)
        assertEquals("/api/v1/addresses", request.path)
        assertEquals("a1", saved.addressId)
        assertTrue(request.body.readUtf8().contains("\"label\":\"Maison\""))
    }

    @Test
    fun `a blank tracking number is not sent`() = runTest {
        server.enqueue(json("""{"data":{"order_id":"o1","status":"shipped"},"error":null,"meta":{}}"""))

        val order = (repo.ship("o1", "  ") as ApiResult.Success).data

        val request = server.takeRequest()
        assertEquals("/api/v1/seller/orders/o1/ship", request.path)
        assertEquals("{}", request.body.readUtf8())
        assertNull(order.total)
        assertEquals(OrderDto.SHIPPED, order.status)
    }

    private fun json(body: String) = MockResponse().setHeader("Content-Type", "application/json").setBody(body)
}
