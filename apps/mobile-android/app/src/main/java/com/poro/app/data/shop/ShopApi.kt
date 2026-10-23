package com.poro.app.data.shop

import com.poro.app.core.network.ApiEnvelope
import com.poro.app.core.network.PageDto
import com.poro.app.data.user.ImageUploadRequest
import com.poro.app.data.user.PresignedForm
import com.poro.app.data.user.UploadKey
import kotlinx.serialization.SerialName
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.JsonObject
import retrofit2.Response
import retrofit2.http.Body
import retrofit2.http.DELETE
import retrofit2.http.GET
import retrofit2.http.PATCH
import retrofit2.http.POST
import retrofit2.http.PUT
import retrofit2.http.Path
import retrofit2.http.Query

/**
 * Boutiques et produits (docs/api/shop.openapi.yaml). Montants en unités mineures de la devise
 * de la boutique : XOF et XAF n'en ont pas, NGN est en kobo.
 */
interface ShopApi {
    // Catalogue public.
    @GET("api/v1/shops/{handle}")
    suspend fun shop(@Path("handle") handle: String): Response<ApiEnvelope<ShopDto>>

    @GET("api/v1/shops/{handle}/products")
    suspend fun shopProducts(
        @Path("handle") handle: String,
        @Query("cursor") cursor: String?,
        @Query("limit") limit: Int,
    ): Response<ApiEnvelope<PageDto<ProductDto>>>

    @GET("api/v1/products/{productId}")
    suspend fun product(@Path("productId") productId: String): Response<ApiEnvelope<ProductDto>>

    // Sa propre boutique.
    @POST("api/v1/shops")
    suspend fun openShop(@Body body: OpenShop): Response<ApiEnvelope<ShopDto>>

    @GET("api/v1/shops/me")
    suspend fun myShop(): Response<ApiEnvelope<ShopDto>>

    /** `description: null` l'efface : JsonObject, PoroJson omettant les null. */
    @PATCH("api/v1/shops/me")
    suspend fun updateMyShop(@Body body: JsonObject): Response<ApiEnvelope<ShopDto>>

    @POST("api/v1/shops/me/close")
    suspend fun closeShop(): Response<ApiEnvelope<ShopDto>>

    @POST("api/v1/shops/me/reopen")
    suspend fun reopenShop(): Response<ApiEnvelope<ShopDto>>

    @POST("api/v1/shops/me/logo/upload-url")
    suspend fun logoUploadUrl(@Body body: ImageUploadRequest): Response<ApiEnvelope<PresignedForm>>

    @PUT("api/v1/shops/me/logo")
    suspend fun confirmLogo(@Body body: UploadKey): Response<ApiEnvelope<ShopDto>>

    @DELETE("api/v1/shops/me/logo")
    suspend fun deleteLogo(): Response<ApiEnvelope<ShopDto>>

    @GET("api/v1/shops/me/products")
    suspend fun myProducts(@Query("cursor") cursor: String?, @Query("limit") limit: Int): Response<ApiEnvelope<PageDto<ProductDto>>>

    @POST("api/v1/shops/me/products")
    suspend fun createProduct(@Body body: NewProduct): Response<ApiEnvelope<ProductDto>>

    @GET("api/v1/shops/me/products/{productId}")
    suspend fun myProduct(@Path("productId") productId: String): Response<ApiEnvelope<ProductDto>>

    /** Champs, statut ou liste complète des déclinaisons (voir [ProductPatch]). */
    @PATCH("api/v1/shops/me/products/{productId}")
    suspend fun updateProduct(@Path("productId") productId: String, @Body body: JsonObject): Response<ApiEnvelope<ProductDto>>

    /** 204 sans corps ; refusé tant que des commandes retiennent son stock. */
    @DELETE("api/v1/shops/me/products/{productId}")
    suspend fun deleteProduct(@Path("productId") productId: String): Response<ApiEnvelope<Unit>>

    @PUT("api/v1/shops/me/products/{productId}/variants/{variantId}/stock")
    suspend fun setStock(
        @Path("productId") productId: String,
        @Path("variantId") variantId: String,
        @Body body: StockOnHand,
    ): Response<ApiEnvelope<ProductDto>>

    @POST("api/v1/shops/me/products/{productId}/images/upload-url")
    suspend fun productImageUploadUrl(@Path("productId") productId: String, @Body body: ImageUploadRequest): Response<ApiEnvelope<PresignedForm>>

    @PUT("api/v1/shops/me/products/{productId}/images")
    suspend fun confirmProductImage(@Path("productId") productId: String, @Body body: UploadKey): Response<ApiEnvelope<ProductDto>>

    @DELETE("api/v1/shops/me/products/{productId}/images/{imageId}")
    suspend fun deleteProductImage(@Path("productId") productId: String, @Path("imageId") imageId: String): Response<ApiEnvelope<ProductDto>>
}

/** Boutique ; [ownerId] et [status] seulement pour sa propre boutique. */
@Serializable
data class ShopDto(
    @SerialName("shop_id") val shopId: String,
    val handle: String = "",
    val name: String = "",
    val description: String? = null,
    @SerialName("logo_url") val logoUrl: String? = null,
    @SerialName("country_code") val countryCode: String = "",
    val currency: String = "",
    /** `active`, `suspended` ou `closed`. */
    val status: String? = null,
) {
    companion object {
        const val ACTIVE = "active"
        const val SUSPENDED = "suspended"
        const val CLOSED = "closed"
    }
}

@Serializable
data class ShopRef(
    @SerialName("shop_id") val shopId: String = "",
    val handle: String = "",
    val name: String = "",
)

/** Produit ; [status] et le stock des déclinaisons seulement pour ses propres produits. */
@Serializable
data class ProductDto(
    @SerialName("product_id") val productId: String,
    val shop: ShopRef = ShopRef(),
    val title: String = "",
    val description: String? = null,
    val currency: String = "",
    val images: List<ProductImage> = emptyList(),
    val variants: List<VariantDto> = emptyList(),
    /** `draft`, `active` ou `archived`. */
    val status: String? = null,
) {
    companion object {
        const val DRAFT = "draft"
        const val ACTIVE = "active"
        const val ARCHIVED = "archived"
    }
}

@Serializable
data class ProductImage(@SerialName("image_id") val imageId: String, val url: String)

@Serializable
data class VariantDto(
    @SerialName("variant_id") val variantId: String,
    val title: String = "",
    val price: Long = 0,
    /** Indication seulement : la réservation au moment de la commande décide. */
    @SerialName("in_stock") val inStock: Boolean = true,
    @SerialName("stock_on_hand") val stockOnHand: Int? = null,
    @SerialName("stock_reserved") val stockReserved: Int? = null,
    @SerialName("stock_available") val stockAvailable: Int? = null,
)

@Serializable
data class OpenShop(
    val name: String,
    val handle: String,
    @SerialName("country_code") val countryCode: String,
    val description: String? = null,
)

@Serializable
data class NewProduct(val title: String, val description: String? = null, val variants: List<VariantInput>)

/** Déclinaison envoyée : [variantId] null pour une nouvelle, [stock] initial d'une nouvelle seulement. */
@Serializable
data class VariantInput(
    @SerialName("variant_id") val variantId: String? = null,
    val title: String,
    val price: Long,
    val stock: Int? = null,
)

@Serializable
data class StockOnHand(@SerialName("stock_on_hand") val stockOnHand: Int)
