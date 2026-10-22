package com.poro.app.data.shop

import com.poro.app.core.network.ApiEnvelope
import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PageDto
import com.poro.app.core.network.PresignedUploader
import com.poro.app.core.network.UploadResult
import com.poro.app.core.network.apiCall
import com.poro.app.data.user.ImageUploadRequest
import com.poro.app.data.user.PresignedForm
import com.poro.app.data.user.UploadKey
import com.poro.app.data.user.uploadRejected
import kotlinx.serialization.json.JsonArray
import kotlinx.serialization.json.JsonNull
import kotlinx.serialization.json.JsonPrimitive
import kotlinx.serialization.json.buildJsonObject
import retrofit2.Response

interface ShopRepository {
    suspend fun shop(handle: String): ApiResult<ShopDto>

    suspend fun shopProducts(handle: String, cursor: String?): ApiResult<PageDto<ProductDto>>

    suspend fun product(productId: String): ApiResult<ProductDto>

    /** Sa boutique ; `shop_not_found` (404) si le compte n'en a pas encore. */
    suspend fun myShop(): ApiResult<ShopDto>

    suspend fun openShop(name: String, handle: String, countryCode: String, description: String?): ApiResult<ShopDto>

    suspend fun updateMyShop(name: String, description: String): ApiResult<ShopDto>

    suspend fun setShopOpen(open: Boolean): ApiResult<ShopDto>

    suspend fun uploadLogo(bytes: ByteArray, contentType: String): ApiResult<ShopDto>

    suspend fun myProducts(cursor: String?): ApiResult<PageDto<ProductDto>>

    suspend fun myProduct(productId: String): ApiResult<ProductDto>

    suspend fun createProduct(title: String, description: String?, variants: List<VariantInput>): ApiResult<ProductDto>

    /** Titre, description et liste complète des déclinaisons (les absentes sont supprimées). */
    suspend fun updateProduct(productId: String, title: String, description: String?, variants: List<VariantInput>): ApiResult<ProductDto>

    /** Publier (`active`, une photo au moins), dépublier (`draft`) ou archiver. */
    suspend fun setProductStatus(productId: String, status: String): ApiResult<ProductDto>

    suspend fun deleteProduct(productId: String): ApiResult<Unit>

    suspend fun setStock(productId: String, variantId: String, stockOnHand: Int): ApiResult<ProductDto>

    suspend fun uploadProductImage(productId: String, bytes: ByteArray, contentType: String): ApiResult<ProductDto>

    suspend fun deleteProductImage(productId: String, imageId: String): ApiResult<ProductDto>
}

class DefaultShopRepository(
    private val api: ShopApi,
    private val uploader: PresignedUploader,
) : ShopRepository {
    override suspend fun shop(handle: String) = apiCall { api.shop(handle.removePrefix("@").lowercase()) }

    override suspend fun shopProducts(handle: String, cursor: String?) = apiCall { api.shopProducts(handle, cursor, PAGE_SIZE) }

    override suspend fun product(productId: String) = apiCall { api.product(productId) }

    override suspend fun myShop() = apiCall { api.myShop() }

    override suspend fun openShop(name: String, handle: String, countryCode: String, description: String?) =
        apiCall { api.openShop(OpenShop(name.trim(), handle.trim().removePrefix("@").lowercase(), countryCode, description?.trim()?.ifEmpty { null })) }

    override suspend fun updateMyShop(name: String, description: String) = apiCall {
        api.updateMyShop(
            buildJsonObject {
                put("name", JsonPrimitive(name.trim()))
                put("description", description.trim().ifEmpty { null }?.let(::JsonPrimitive) ?: JsonNull)
            },
        )
    }

    override suspend fun setShopOpen(open: Boolean) = apiCall { if (open) api.reopenShop() else api.closeShop() }

    override suspend fun uploadLogo(bytes: ByteArray, contentType: String): ApiResult<ShopDto> =
        presignedImage({ api.logoUploadUrl(ImageUploadRequest(contentType)) }, bytes, contentType) { key ->
            api.confirmLogo(UploadKey(key))
        }

    override suspend fun myProducts(cursor: String?) = apiCall { api.myProducts(cursor, PAGE_SIZE) }

    override suspend fun myProduct(productId: String) = apiCall { api.myProduct(productId) }

    override suspend fun createProduct(title: String, description: String?, variants: List<VariantInput>) =
        apiCall { api.createProduct(NewProduct(title.trim(), description?.trim()?.ifEmpty { null }, variants)) }

    override suspend fun updateProduct(productId: String, title: String, description: String?, variants: List<VariantInput>) =
        apiCall {
            api.updateProduct(
                productId,
                buildJsonObject {
                    put("title", JsonPrimitive(title.trim()))
                    put("description", description?.trim()?.ifEmpty { null }?.let(::JsonPrimitive) ?: JsonNull)
                    put(
                        "variants",
                        JsonArray(
                            variants.map { v ->
                                buildJsonObject {
                                    v.variantId?.let { put("variant_id", JsonPrimitive(it)) }
                                    put("title", JsonPrimitive(v.title.trim()))
                                    put("price", JsonPrimitive(v.price))
                                    // Le stock initial ne vaut que pour une nouvelle déclinaison.
                                    if (v.variantId == null) v.stock?.let { put("stock", JsonPrimitive(it)) }
                                }
                            },
                        ),
                    )
                },
            )
        }

    override suspend fun setProductStatus(productId: String, status: String) =
        apiCall { api.updateProduct(productId, buildJsonObject { put("status", JsonPrimitive(status)) }) }

    override suspend fun deleteProduct(productId: String) = apiCall(ifEmpty = {}) { api.deleteProduct(productId) }

    override suspend fun setStock(productId: String, variantId: String, stockOnHand: Int) =
        apiCall { api.setStock(productId, variantId, StockOnHand(stockOnHand)) }

    override suspend fun uploadProductImage(productId: String, bytes: ByteArray, contentType: String): ApiResult<ProductDto> =
        presignedImage({ api.productImageUploadUrl(productId, ImageUploadRequest(contentType)) }, bytes, contentType) { key ->
            api.confirmProductImage(productId, UploadKey(key))
        }

    override suspend fun deleteProductImage(productId: String, imageId: String) = apiCall { api.deleteProductImage(productId, imageId) }

    /** Formulaire présigné, envoi au stockage, puis confirmation (image recadrée, sans métadonnées). */
    private suspend fun <T> presignedImage(
        form: suspend () -> Response<ApiEnvelope<PresignedForm>>,
        bytes: ByteArray,
        contentType: String,
        confirm: suspend (String) -> Response<ApiEnvelope<T>>,
    ): ApiResult<T> {
        val presigned = when (val result = apiCall { form() }) {
            is ApiResult.Success -> result.data
            is ApiResult.Failure -> return result
            is ApiResult.NetworkError -> return result
        }
        return when (val upload = uploader.postForm(presigned.uploadUrl, presigned.fields, bytes, contentType)) {
            is UploadResult.Success -> apiCall { confirm(presigned.uploadKey) }
            is UploadResult.Rejected -> uploadRejected(upload.httpStatus)
            is UploadResult.NetworkError -> ApiResult.NetworkError(upload.cause)
        }
    }

    private companion object {
        const val PAGE_SIZE = 20
    }
}
