package com.poro.app.ui.shop

import com.poro.app.R
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.data.order.CartDto
import com.poro.app.data.order.CartLine
import com.poro.app.data.order.CartShop
import com.poro.app.data.order.OrderDto
import com.poro.app.data.shop.ProductDto
import com.poro.app.data.shop.ProductImage
import com.poro.app.data.shop.VariantDto
import com.poro.app.testing.FakeMediaReader
import com.poro.app.testing.FakeOrderRepository
import com.poro.app.testing.FakeShopRepository
import com.poro.app.testing.MainDispatcherRule
import java.io.IOException
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class ShopViewModelsTest {

    @get:Rule
    val main = MainDispatcherRule()

    private val shops = FakeShopRepository()
    private val orders = FakeOrderRepository()

    @Test
    fun `an unknown handle shows shop not found`() = runTest {
        shops.shop = ApiResult.Failure(404, ApiErrorCodes.SHOP_NOT_FOUND, "no shop", null, null)
        val vm = ShopHomeViewModel(shops, orders)

        vm.onHandleChange("@Inconnu")
        vm.find()
        runCurrent()

        assertEquals(R.string.shop_not_found, vm.state.value.error?.text)
        assertTrue("shop:inconnu" in shops.calls)
    }

    @Test
    fun `the product page preselects the first variant in stock and adds it to the cart`() = runTest {
        shops.product = ApiResult.Success(
            ProductDto("p1", variants = listOf(VariantDto("v1", "S", 5000, inStock = false), VariantDto("v2", "M", 5000))),
        )
        val vm = ProductViewModel("p1", shops, orders)
        runCurrent()
        assertEquals("v2", vm.state.value.variantId)

        vm.setQuantity(150)
        vm.addToCart()

        assertEquals(ProductEvent.AddedToCart, vm.events.first())
        assertTrue("cart" in orders.calls)
        assertTrue("setQuantity:v2:99" in orders.calls)
    }

    @Test
    fun `adding to cart sums the quantity already in the cart`() = runTest {
        shops.product = ApiResult.Success(ProductDto("p1", variants = listOf(VariantDto("v2", "M", 5000))))
        orders.cart = ApiResult.Success(CartDto(shops = listOf(CartShop(items = listOf(CartLine("v2", quantity = 1))))))
        val vm = ProductViewModel("p1", shops, orders)
        runCurrent()

        vm.setQuantity(1)
        vm.addToCart()

        assertEquals(ProductEvent.AddedToCart, vm.events.first())
        assertTrue("setQuantity:v2:2" in orders.calls)
    }

    @Test
    fun `adding to cart does not replace the line if the cart cannot be read`() = runTest {
        shops.product = ApiResult.Success(ProductDto("p1", variants = listOf(VariantDto("v2", "M", 5000))))
        orders.cart = ApiResult.NetworkError(IOException())
        val vm = ProductViewModel("p1", shops, orders)
        runCurrent()

        vm.addToCart()
        runCurrent()

        assertEquals(R.string.error_network, vm.state.value.error?.text)
        assertFalse(orders.calls.any { it.startsWith("setQuantity") })
    }

    @Test
    fun `a network error does not pretend the account has no shop`() = runTest {
        val vm = ShopHomeViewModel(shops, orders)
        vm.refresh()
        runCurrent()
        assertEquals(true, vm.state.value.hasShop)

        shops.shop = ApiResult.NetworkError(IOException())
        vm.refresh()
        runCurrent()
        assertEquals(true, vm.state.value.hasShop)

        shops.shop = ApiResult.Failure(404, ApiErrorCodes.SHOP_NOT_FOUND, "none", null, null)
        vm.refresh()
        runCurrent()
        assertEquals(false, vm.state.value.hasShop)
    }

    @Test
    fun `an unread shop stays unknown rather than offering to open one`() = runTest {
        shops.shop = ApiResult.NetworkError(IOException())
        val vm = ShopHomeViewModel(shops, orders)
        vm.refresh()
        runCurrent()
        assertNull(vm.state.value.hasShop)
    }

    @Test
    fun `the editor accepts ten product photos`() {
        val nine = ProductEditorUiState(images = List(9) { ProductImage("i$it", "u") })
        val ten = ProductEditorUiState(images = List(10) { ProductImage("i$it", "u") })
        assertTrue(nine.canAddImage)
        assertFalse(ten.canAddImage)
    }

    @Test
    fun `lowering a cart line to zero removes it and unavailable lines block checkout`() = runTest {
        orders.cart = ApiResult.Success(
            CartDto(
                shops = listOf(CartShop(items = listOf(CartLine("v1", quantity = 1)))),
                unavailable = listOf(CartLine("v9", available = false)),
            ),
        )
        val vm = CartViewModel(orders)
        runCurrent()
        assertFalse(vm.state.value.canCheckout)

        vm.setQuantity("v1", 0)
        runCurrent()

        assertTrue("removeItem:v1" in orders.calls)
    }

    @Test
    fun `a seller ships with the tracking number and a buyer confirms receipt`() = runTest {
        val seller = OrderViewModel("o1", orders, asSeller = true)
        runCurrent()
        seller.onTrackingChange("TRK-42")
        orders.order = ApiResult.Success(OrderDto("o1", status = OrderDto.SHIPPED))
        seller.ship()
        runCurrent()
        assertEquals(OrderDto.SHIPPED, seller.state.value.order.valueOrNull?.status)

        val buyer = OrderViewModel("o1", orders, asSeller = false)
        runCurrent()
        buyer.confirmReceipt()
        runCurrent()

        assertEquals(listOf("sellerOrder:o1", "ship:o1:TRK-42", "order:o1", "confirmReceipt:o1"), orders.calls)
    }

    @Test
    fun `the editor sends stock changes of existing variants only`() = runTest {
        shops.product = ApiResult.Success(
            ProductDto(
                "p1",
                title = "Pagne",
                currency = "XOF",
                variants = listOf(
                    VariantDto("v1", "M", 5000, stockOnHand = 3, stockReserved = 1),
                    VariantDto("v2", "L", 6000, stockOnHand = 4),
                ),
            ),
        )
        val vm = ProductEditorViewModel("p1", "XOF", shops, FakeMediaReader())
        runCurrent()
        val state = vm.state.value
        assertEquals("5000", state.variants[0].price)
        assertEquals(1, state.variants[0].reserved)

        vm.onVariantChange(1, state.variants[1].copy(stock = "10"))
        vm.addVariant()
        vm.onVariantChange(2, VariantDraft(title = "XL", price = "7 000", stock = "2"))
        vm.save()
        runCurrent()

        assertEquals(listOf("myProduct:p1", "updateProduct:p1:Pagne", "setStock:p1:v2:10"), shops.calls)
        assertEquals(7000L, shops.savedVariants!![2].price)
        assertEquals(2, shops.savedVariants!![2].stock)
    }

    @Test
    fun `an invalid price blocks saving`() {
        val draft = VariantDraft(title = "M", price = "12,5", stock = "1")
        assertFalse(draft.isValid("XOF"))
        assertTrue(draft.isValid("NGN"))
        assertEquals(1250L, draft.priceIn("NGN"))
    }

    @Test
    fun `a logo that is not an image is refused before any upload`() = runTest {
        val media = FakeMediaReader().apply { add("content://doc", "application/pdf", ByteArray(10)) }
        val vm = MyShopViewModel(shops, media)

        vm.onLogoPicked("content://doc")
        runCurrent()

        assertEquals(R.string.error_image_invalid, vm.state.value.error?.text)
        assertFalse(shops.calls.any { it.startsWith("uploadLogo") })
    }

    @Test
    fun `an account without a shop gets the opening form`() = runTest {
        shops.shop = ApiResult.Failure(404, ApiErrorCodes.SHOP_NOT_FOUND, "none", null, null)
        val vm = MyShopViewModel(shops, FakeMediaReader())

        vm.load()
        runCurrent()

        assertFalse(vm.state.value.loading)
        assertEquals(null, vm.state.value.shop)
        assertEquals(null, vm.state.value.error)
    }
}
