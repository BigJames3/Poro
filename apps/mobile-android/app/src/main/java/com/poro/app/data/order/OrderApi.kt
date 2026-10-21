package com.poro.app.data.order

import com.poro.app.core.network.ApiEnvelope
import com.poro.app.core.network.PageDto
import kotlinx.serialization.EncodeDefault
import kotlinx.serialization.ExperimentalSerializationApi
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.POST
import retrofit2.http.PUT
import retrofit2.http.Path
import retrofit2.http.Query

/**
 * Panier, commande et suivi (docs/api/order.openapi.yaml). La commande se passe en deux temps :
 * un récapitulatif (preview), puis une confirmation explicite ; une commande par boutique.
 */
interface OrderApi {
    @GET("api/v1/cart")
    suspend fun cart(): Response<ApiEnvelope<CartDto>>

    @PUT("api/v1/cart/items/{variantId}")
    suspend fun setQuantity(@Path("variantId") variantId: String, @Body body: Quantity): Response<ApiEnvelope<CartDto>>

    @DELETE("api/v1/cart/items/{variantId}")
    suspend fun removeItem(@Path("variantId") variantId: String): Response<ApiEnvelope<CartDto>>

    @GET("api/v1/checkout/prefill")
    suspend fun prefill(): Response<ApiEnvelope<Prefill>>

    @POST("api/v1/checkout/preview")
    suspend fun preview(@Body body: PreviewRequest): Response<ApiEnvelope<PreviewDto>>

    @POST("api/v1/checkout/confirm")
    suspend fun confirm(@Body body: ConfirmRequest): Response<ApiEnvelope<Placed>>

    @GET("api/v1/addresses")
    suspend fun addresses(): Response<ApiEnvelope<List<AddressDto>>>

    /** 204 sans corps. */
    @DELETE("api/v1/addresses/{addressId}")
    suspend fun deleteAddress(@Path("addressId") addressId: String): Response<ApiEnvelope<Unit>>

    @GET("api/v1/orders")
    suspend fun orders(@Query("cursor") cursor: String?, @Query("limit") limit: Int): Response<ApiEnvelope<PageDto<OrderDto>>>

    @GET("api/v1/orders/{orderId}")
    suspend fun order(@Path("orderId") orderId: String): Response<ApiEnvelope<OrderDto>>

    @POST("api/v1/orders/{orderId}/cancel")
    suspend fun cancel(@Path("orderId") orderId: String): Response<ApiEnvelope<OrderDto>>

    @POST("api/v1/orders/{orderId}/confirm-receipt")
    suspend fun confirmReceipt(@Path("orderId") orderId: String): Response<ApiEnvelope<OrderDto>>

    @GET("api/v1/seller/orders")
    suspend fun sellerOrders(@Query("cursor") cursor: String?, @Query("limit") limit: Int): Response<ApiEnvelope<PageDto<OrderDto>>>

    @GET("api/v1/seller/orders/{orderId}")
    suspend fun sellerOrder(@Path("orderId") orderId: String): Response<ApiEnvelope<OrderDto>>

    @POST("api/v1/seller/orders/{orderId}/ship")
    suspend fun ship(@Path("orderId") orderId: String, @Body body: Tracking): Response<ApiEnvelope<OrderDto>>

    @POST("api/v1/seller/orders/{orderId}/cancel")
    suspend fun sellerCancel(@Path("orderId") orderId: String): Response<ApiEnvelope<OrderDto>>
}

@Serializable
data class Quantity(val quantity: Int)

@Serializable
data class CartDto(
    val shops: List<CartShop> = emptyList(),
    /** Lignes dont le produit n'est plus en vente : à retirer avant de commander. */
    val unavailable: List<CartLine> = emptyList(),
    @SerialName("item_count") val itemCount: Int = 0,
)

@Serializable
data class CartShop(val shop: CartShopRef = CartShopRef(), val items: List<CartLine> = emptyList(), val subtotal: Long = 0)

@Serializable
data class CartShopRef(
    @SerialName("shop_id") val shopId: String = "",
    val name: String = "",
    val handle: String = "",
    val currency: String = "",
)

@Serializable
data class CartLine(
    @SerialName("variant_id") val variantId: String,
    @SerialName("product_id") val productId: String? = null,
    val title: String? = null,
    @SerialName("variant_title") val variantTitle: String? = null,
    @SerialName("image_url") val imageUrl: String? = null,
    val quantity: Int = 1,
    @SerialName("unit_price") val unitPrice: Long? = null,
    @SerialName("line_total") val lineTotal: Long? = null,
    @SerialName("in_stock") val inStock: Boolean = true,
    val available: Boolean = true,
)

@Serializable
data class Location(
    val latitude: Double,
    val longitude: Double,
    @SerialName("accuracy_m") val accuracyM: Int? = null,
)

/** Contact et lieu de livraison : nom, téléphone et ville requis, plus une adresse ou une position. */
@Serializable
data class Delivery(
    @SerialName("full_name") val fullName: String = "",
    val phone: String = "",
    val city: String = "",
    val address: String? = null,
    val landmark: String? = null,
    val location: Location? = null,
)

@Serializable
data class Prefill(
    /** `address_book`, `account` ou `empty`. */
    val source: String = "empty",
    @SerialName("address_id") val addressId: String? = null,
    val delivery: PrefillDelivery = PrefillDelivery(),
)

/** Champs du préremplissage, chacun null si le service qui le fournit n'a pas répondu. */
@Serializable
data class PrefillDelivery(
    @SerialName("full_name") val fullName: String? = null,
    val phone: String? = null,
    val city: String? = null,
    val address: String? = null,
    val landmark: String? = null,
)

@OptIn(ExperimentalSerializationApi::class)
@Serializable
data class PreviewRequest(
    val delivery: Delivery,
    // PoroJson n'encode pas les valeurs par défaut : le serveur exige pourtant ces deux champs.
    @EncodeDefault @SerialName("payment_method") val paymentMethod: String = PAYMENT_CASH_ON_DELIVERY,
    @EncodeDefault @SerialName("save_address") val saveAddress: Boolean = false,
    @SerialName("address_label") val addressLabel: String? = null,
)

const val PAYMENT_CASH_ON_DELIVERY = "cash_on_delivery"

@Serializable
data class PreviewDto(
    @SerialName("preview_id") val previewId: String,
    @SerialName("expires_at") val expiresAt: String? = null,
    val shops: List<PreviewShop> = emptyList(),
    val totals: List<Money> = emptyList(),
    @SerialName("item_count") val itemCount: Int = 0,
    val delivery: Delivery = Delivery(),
    @SerialName("payment_method_label") val paymentMethodLabel: String = "",
    @SerialName("delivery_fee_notice") val deliveryFeeNotice: String = "",
)

@Serializable
data class PreviewShop(
    val shop: CartShopRef = CartShopRef(),
    val currency: String = "",
    val items: List<PreviewItem> = emptyList(),
    val subtotal: Long = 0,
)

@Serializable
data class PreviewItem(
    @SerialName("variant_id") val variantId: String = "",
    val title: String = "",
    @SerialName("variant_title") val variantTitle: String = "",
    @SerialName("image_url") val imageUrl: String? = null,
    val quantity: Int = 1,
    @SerialName("unit_price") val unitPrice: Long = 0,
    @SerialName("line_total") val lineTotal: Long = 0,
)

@Serializable
data class Money(val currency: String, val amount: Long)

@OptIn(ExperimentalSerializationApi::class)
@Serializable
data class ConfirmRequest(@SerialName("preview_id") val previewId: String, @EncodeDefault val confirmed: Boolean = true)

@Serializable
data class Placed(
    @SerialName("checkout_id") val checkoutId: String = "",
    val orders: List<OrderDto> = emptyList(),
    @SerialName("address_saved") val addressSaved: Boolean = false,
)

@Serializable
data class AddressDto(
    @SerialName("address_id") val addressId: String,
    val label: String? = null,
    @SerialName("is_default") val isDefault: Boolean = false,
    @SerialName("full_name") val fullName: String = "",
    val phone: String = "",
    val city: String = "",
    val address: String? = null,
    val landmark: String? = null,
) {
    fun toDelivery() = Delivery(fullName, phone, city, address, landmark)
}

@Serializable
data class OrderDto(
    @SerialName("order_id") val orderId: String,
    /** pending, awaiting_payment, confirmed, paid, shipped, completed ou cancelled. */
    val status: String = "",
    val shop: ShopName = ShopName(),
    val currency: String = "",
    @SerialName("payment_method_label") val paymentMethodLabel: String = "",
    val items: List<OrderItem> = emptyList(),
    @SerialName("subtotal_seen") val subtotalSeen: Long = 0,
    /** Null tant que le stock n'est pas réservé (prix définitifs). */
    val total: Long? = null,
    /** Effacée 90 jours après la fin de la commande. */
    val delivery: Delivery? = null,
    val tracking: String? = null,
    @SerialName("cancel_reason") val cancelReason: String? = null,
    val paid: Boolean = false,
    @SerialName("placed_at") val placedAt: String? = null,
    @SerialName("shipped_at") val shippedAt: String? = null,
    @SerialName("completed_at") val completedAt: String? = null,
    @SerialName("cancelled_at") val cancelledAt: String? = null,
    @SerialName("created_at") val createdAt: String? = null,
) {
    /** Annulable par l'acheteur ou le vendeur tant que la commande n'est pas expédiée. */
    val cancellable: Boolean get() = status in setOf(PENDING, AWAITING_PAYMENT, CONFIRMED, PAID)

    companion object {
        const val PENDING = "pending"
        const val AWAITING_PAYMENT = "awaiting_payment"
        const val CONFIRMED = "confirmed"
        const val PAID = "paid"
        const val SHIPPED = "shipped"
        const val COMPLETED = "completed"
        const val CANCELLED = "cancelled"
    }
}

@Serializable
data class ShopName(@SerialName("shop_id") val shopId: String = "", val name: String = "")

@Serializable
data class OrderItem(
    val title: String = "",
    val quantity: Int = 1,
    @SerialName("unit_price") val unitPrice: Long = 0,
    @SerialName("unit_price_seen") val unitPriceSeen: Long = 0,
)

@Serializable
data class Tracking(val tracking: String? = null)
