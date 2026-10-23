package com.poro.app.data.order

import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PageDto
import com.poro.app.core.network.apiCall

interface OrderRepository {
    suspend fun cart(): ApiResult<CartDto>

    /** Quantité totale d'une déclinaison dans le panier (1 à 99). */
    suspend fun setQuantity(variantId: String, quantity: Int): ApiResult<CartDto>

    suspend fun removeItem(variantId: String): ApiResult<CartDto>

    suspend fun clearCart(): ApiResult<CartDto>

    suspend fun prefill(): ApiResult<Prefill>

    suspend fun addresses(): ApiResult<List<AddressDto>>

    suspend fun saveAddress(input: AddressInput, addressId: String?): ApiResult<AddressDto>

    suspend fun setDefaultAddress(addressId: String): ApiResult<AddressDto>

    suspend fun deleteAddress(addressId: String): ApiResult<Unit>

    /** Récapitulatif valable 15 minutes ; rien n'est commandé. */
    suspend fun preview(request: PreviewRequest): ApiResult<PreviewDto>

    /** Passe les commandes du récapitulatif (une par boutique) ; idempotent. */
    suspend fun confirm(previewId: String): ApiResult<Placed>

    suspend fun orders(cursor: String?): ApiResult<PageDto<OrderDto>>

    suspend fun order(orderId: String): ApiResult<OrderDto>

    suspend fun cancel(orderId: String): ApiResult<OrderDto>

    suspend fun confirmReceipt(orderId: String): ApiResult<OrderDto>

    suspend fun sellerOrders(cursor: String?): ApiResult<PageDto<OrderDto>>

    suspend fun sellerOrder(orderId: String): ApiResult<OrderDto>

    suspend fun ship(orderId: String, tracking: String?): ApiResult<OrderDto>

    suspend fun sellerCancel(orderId: String): ApiResult<OrderDto>
}

class DefaultOrderRepository(private val api: OrderApi) : OrderRepository {
    override suspend fun cart() = apiCall { api.cart() }

    override suspend fun setQuantity(variantId: String, quantity: Int) = apiCall { api.setQuantity(variantId, Quantity(quantity)) }

    override suspend fun removeItem(variantId: String) = apiCall { api.removeItem(variantId) }

    override suspend fun clearCart() = apiCall { api.clearCart() }

    override suspend fun prefill() = apiCall { api.prefill() }

    override suspend fun addresses() = apiCall { api.addresses() }

    override suspend fun saveAddress(input: AddressInput, addressId: String?) =
        apiCall { if (addressId == null) api.createAddress(input) else api.updateAddress(addressId, input) }

    override suspend fun setDefaultAddress(addressId: String) = apiCall { api.setDefaultAddress(addressId) }

    override suspend fun deleteAddress(addressId: String) = apiCall(ifEmpty = {}) { api.deleteAddress(addressId) }

    override suspend fun preview(request: PreviewRequest) = apiCall { api.preview(request) }

    override suspend fun confirm(previewId: String) = apiCall { api.confirm(ConfirmRequest(previewId, confirmed = true)) }

    override suspend fun orders(cursor: String?) = apiCall { api.orders(cursor, PAGE_SIZE) }

    override suspend fun order(orderId: String) = apiCall { api.order(orderId) }

    override suspend fun cancel(orderId: String) = apiCall { api.cancel(orderId) }

    override suspend fun confirmReceipt(orderId: String) = apiCall { api.confirmReceipt(orderId) }

    override suspend fun sellerOrders(cursor: String?) = apiCall { api.sellerOrders(cursor, PAGE_SIZE) }

    override suspend fun sellerOrder(orderId: String) = apiCall { api.sellerOrder(orderId) }

    override suspend fun ship(orderId: String, tracking: String?) =
        apiCall { api.ship(orderId, Tracking(tracking?.trim()?.ifEmpty { null })) }

    override suspend fun sellerCancel(orderId: String) = apiCall { api.sellerCancel(orderId) }

    private companion object {
        const val PAGE_SIZE = 20
    }
}
