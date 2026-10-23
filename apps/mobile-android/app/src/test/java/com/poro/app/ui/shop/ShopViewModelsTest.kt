package com.poro.app.ui.shop

import com.poro.app.R
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.data.order.CartDto
import com.poro.app.data.order.CartLine
import com.poro.app.data.order.CartShop
import com.poro.app.data.order.OrderDto
import com.poro.app.data.shop.ProductDto
import com.poro.app.data.shop.ShopDto
import com.poro.app.data.shop.ShopRef
import com.poro.app.data.shop.VariantDto
import com.poro.app.testing.FakeMediaReader
import com.poro.app.testing.FakeOrderRepository
import com.poro.app.testing.FakeShopRepository
import com.poro.app.testing.MainDispatcherRule
import com.poro.app.ui.common.Loadable
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
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

        assertEquals(ProductEvent.AddedToCart(99), vm.events.first())
        assertTrue("setQuantity:v2:99" in orders.calls)
    }

    @Test
    fun `adding to the cart adds to the quantity already there`() = runTest {
        shops.product = ApiResult.Success(ProductDto("p1", variants = listOf(VariantDto("v1", "M", 5000))))
        orders.cart = ApiResult.Success(CartDto(shops = listOf(CartShop(items = listOf(CartLine("v1", quantity = 2))))))
        val vm = ProductViewModel("p1", shops, orders)
        runCurrent()
        assertEquals(2, vm.state.value.selectedInCart)

        vm.addToCart()
        runCurrent()

        assertTrue("setQuantity:v1:3" in orders.calls)
    }

    @Test
    fun `a full cart line and an own product cannot be added`() = runTest {
        shops.product = ApiResult.Success(
            ProductDto("p1", shop = ShopRef(shopId = "s1"), variants = listOf(VariantDto("v1", "M", 5000))),
        )
        val own = ProductViewModel("p1", shops, orders)
        runCurrent()
        assertTrue(own.state.value.ownProduct)
        assertFalse(own.state.value.canAdd)

        shops.product = ApiResult.Success(ProductDto("p2", variants = listOf(VariantDto("v2", "M", 5000))))
        orders.cart = ApiResult.Success(CartDto(shops = listOf(CartShop(items = listOf(CartLine("v2", quantity = 99))))))
        val full = ProductViewModel("p2", shops, orders)
        runCurrent()
        full.addToCart()
        runCurrent()

        assertTrue(full.state.value.atMaximum)
        assertFalse(orders.calls.any { it.startsWith("setQuantity") })
    }

    @Test
    fun `a status conflict reloads the order`() = runTest {
        orders.order = ApiResult.Success(OrderDto("o1", status = OrderDto.CONFIRMED))
        val vm = OrderViewModel("o1", orders, asSeller = false)
        runCurrent()
        // La boutique a expédié entre-temps : l'annulation est refusée, la commande relue.
        orders.action = ApiResult.Failure(409, ApiErrorCodes.ORDER_STATUS_CONFLICT, "shipped meanwhile", null, null)
        orders.order = ApiResult.Success(OrderDto("o1", status = OrderDto.SHIPPED))

        vm.cancel()
        runCurrent()

        assertEquals(R.string.error_order_status_conflict, vm.state.value.error?.text)
        assertEquals(OrderDto.SHIPPED, vm.state.value.order.valueOrNull?.status)
        assertEquals(listOf("order:o1", "cancel:o1", "order:o1"), orders.calls)
    }

    @Test
    fun `cancel reasons are translated and an unreserved total is an estimate`() {
        assertEquals(R.string.order_cancel_reason_stock, cancelReasonLabel("stock_rejected"))
        assertEquals(R.string.order_cancel_reason_other, cancelReasonLabel("nouveau_motif"))
        assertTrue(OrderDto("o1", status = OrderDto.PENDING, total = null).totalIsEstimate)
        assertFalse(OrderDto("o1", status = OrderDto.CONFIRMED, total = 5000).totalIsEstimate)
        assertFalse(OrderDto("o1", status = OrderDto.CANCELLED, total = null).totalIsEstimate)
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
        vm.load()
        vm.load()
        runCurrent()
        assertFalse(vm.state.value.canCheckout)
        assertEquals("a load in progress is not repeated", 1, orders.calls.count { it == "cart" })

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
    fun `saving the shop details confirms it`() = runTest {
        val vm = MyShopViewModel(shops, FakeMediaReader())
        vm.load()
        runCurrent()

        vm.onEditName("Kofi Store")
        vm.saveDetails()

        assertEquals(MyShopEvent.Saved, vm.events.first())
        assertTrue("updateMyShop:Kofi Store" in shops.calls)
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

    @Test
    fun `emptying the cart replaces it with the server answer, once at a time`() = runTest {
        orders.cart = ApiResult.Success(CartDto(shops = listOf(CartShop(items = listOf(CartLine("v1", quantity = 2))))))
        val vm = CartViewModel(orders)
        vm.load()
        runCurrent()

        orders.cart = ApiResult.Success(CartDto())
        vm.clear()
        vm.clear()
        runCurrent()

        assertEquals("a second tap while emptying is ignored", 1, orders.calls.count { it == "clearCart" })
        val cart = (vm.state.value.cart as Loadable.Loaded).value
        assertTrue(cart.shops.isEmpty())
    }

    @Test
    fun `removing the logo asks the server only when there is one`() = runTest {
        val vm = MyShopViewModel(shops, FakeMediaReader())
        vm.load()
        runCurrent()
        vm.removeLogo()
        runCurrent()
        assertFalse("no logo: nothing sent", "deleteLogo" in shops.calls)

        shops.shop = ApiResult.Success(ShopDto("s1", "kofi.store", "Kofi Store", currency = "XOF", status = ShopDto.ACTIVE, logoUrl = "http://localhost:9000/l.webp"))
        vm.load()
        runCurrent()
        vm.removeLogo()
        runCurrent()
        assertTrue("deleteLogo" in shops.calls)
    }
}
