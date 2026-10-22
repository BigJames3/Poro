package com.poro.app.testing

import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PageDto
import com.poro.app.data.order.AddressDto
import com.poro.app.data.order.CartDto
import com.poro.app.data.order.OrderDto
import com.poro.app.data.order.OrderRepository
import com.poro.app.data.order.Placed
import com.poro.app.data.order.Prefill
import com.poro.app.data.order.PreviewDto
import com.poro.app.data.order.PreviewRequest

/** Panier et commandes en mémoire : chaque appel est noté dans [calls]. */
class FakeOrderRepository : OrderRepository {
    val calls = mutableListOf<String>()
    var cart: ApiResult<CartDto> = ApiResult.Success(CartDto())
    var prefill: ApiResult<Prefill> = ApiResult.Success(Prefill())
    var addresses: ApiResult<List<AddressDto>> = ApiResult.Success(emptyList())
    var preview: ApiResult<PreviewDto> = ApiResult.Success(PreviewDto("pv1"))
    var confirm: ApiResult<Placed> = ApiResult.Success(Placed("c1", listOf(OrderDto("o1"))))
    var order: ApiResult<OrderDto> = ApiResult.Success(OrderDto("o1", status = OrderDto.CONFIRMED))
    var orders: ApiResult<PageDto<OrderDto>> = ApiResult.Success(PageDto(emptyList(), null))
    var lastPreview: PreviewRequest? = null

    /** Résultat des actions sur une commande (annuler, recevoir, expédier) ; sinon [order]. */
    var action: ApiResult<OrderDto>? = null

    override suspend fun cart() = cart.also { calls += "cart" }

    override suspend fun setQuantity(variantId: String, quantity: Int) = cart.also { calls += "setQuantity:$variantId:$quantity" }

    override suspend fun removeItem(variantId: String) = cart.also { calls += "removeItem:$variantId" }

    override suspend fun prefill() = prefill.also { calls += "prefill" }

    override suspend fun addresses() = addresses.also { calls += "addresses" }

    override suspend fun deleteAddress(addressId: String): ApiResult<Unit> = ApiResult.Success(Unit).also { calls += "deleteAddress:$addressId" }

    override suspend fun preview(request: PreviewRequest): ApiResult<PreviewDto> {
        calls += "preview"
        lastPreview = request
        return preview
    }

    override suspend fun confirm(previewId: String) = confirm.also { calls += "confirm:$previewId" }

    override suspend fun orders(cursor: String?) = orders.also { calls += "orders:$cursor" }

    override suspend fun order(orderId: String) = order.also { calls += "order:$orderId" }

    override suspend fun cancel(orderId: String) = (action ?: order).also { calls += "cancel:$orderId" }

    override suspend fun confirmReceipt(orderId: String) = (action ?: order).also { calls += "confirmReceipt:$orderId" }

    override suspend fun sellerOrders(cursor: String?) = orders.also { calls += "sellerOrders:$cursor" }

    override suspend fun sellerOrder(orderId: String) = order.also { calls += "sellerOrder:$orderId" }

    override suspend fun ship(orderId: String, tracking: String?) = (action ?: order).also { calls += "ship:$orderId:$tracking" }

    override suspend fun sellerCancel(orderId: String) = (action ?: order).also { calls += "sellerCancel:$orderId" }
}
