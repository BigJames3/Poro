package com.poro.app.testing

import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PageDto
import com.poro.app.data.shop.ProductDto
import com.poro.app.data.shop.ShopDto
import com.poro.app.data.shop.ShopRepository
import com.poro.app.data.shop.VariantInput

/** Boutique en mémoire : chaque appel est noté dans [calls] et renvoie le résultat configuré. */
class FakeShopRepository : ShopRepository {
    val calls = mutableListOf<String>()
    var shop: ApiResult<ShopDto> = ApiResult.Success(ShopDto("s1", "kofi.store", "Kofi Store", currency = "XOF", status = ShopDto.ACTIVE))
    var product: ApiResult<ProductDto> = ApiResult.Success(ProductDto("p1"))
    var products: ApiResult<PageDto<ProductDto>> = ApiResult.Success(PageDto(emptyList(), null))
    var savedVariants: List<VariantInput>? = null

    override suspend fun shop(handle: String) = shop.also { calls += "shop:$handle" }

    override suspend fun shopProducts(handle: String, cursor: String?) = products.also { calls += "shopProducts:$handle:$cursor" }

    override suspend fun product(productId: String) = product.also { calls += "product:$productId" }

    override suspend fun myShop() = shop.also { calls += "myShop" }

    override suspend fun openShop(name: String, handle: String, countryCode: String, description: String?) =
        shop.also { calls += "openShop:$name:$handle:$countryCode" }

    override suspend fun updateMyShop(name: String, description: String) = shop.also { calls += "updateMyShop:$name" }

    override suspend fun setShopOpen(open: Boolean) = shop.also { calls += "setShopOpen:$open" }

    override suspend fun uploadLogo(bytes: ByteArray, contentType: String) = shop.also { calls += "uploadLogo:$contentType:${bytes.size}" }

    override suspend fun deleteLogo() = shop.also { calls += "deleteLogo" }

    override suspend fun myProducts(cursor: String?) = products.also { calls += "myProducts:$cursor" }

    override suspend fun myProduct(productId: String) = product.also { calls += "myProduct:$productId" }

    override suspend fun createProduct(title: String, description: String?, variants: List<VariantInput>): ApiResult<ProductDto> {
        calls += "createProduct:$title"
        savedVariants = variants
        return product
    }

    override suspend fun updateProduct(productId: String, title: String, description: String?, variants: List<VariantInput>): ApiResult<ProductDto> {
        calls += "updateProduct:$productId:$title"
        savedVariants = variants
        return product
    }

    override suspend fun setProductStatus(productId: String, status: String) = product.also { calls += "setProductStatus:$productId:$status" }

    override suspend fun deleteProduct(productId: String): ApiResult<Unit> = ApiResult.Success(Unit).also { calls += "deleteProduct:$productId" }

    override suspend fun setStock(productId: String, variantId: String, stockOnHand: Int) =
        product.also { calls += "setStock:$productId:$variantId:$stockOnHand" }

    override suspend fun uploadProductImage(productId: String, bytes: ByteArray, contentType: String) =
        product.also { calls += "uploadProductImage:$productId:$contentType" }

    override suspend fun deleteProductImage(productId: String, imageId: String) = product.also { calls += "deleteProductImage:$productId:$imageId" }
}
