package com.poro.app.ui.shop

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.R
import com.poro.app.core.format.Formats
import com.poro.app.core.media.LocalMediaReader
import com.poro.app.core.network.ApiResult
import com.poro.app.core.text.MultilineText
import com.poro.app.data.shop.ProductDto
import com.poro.app.data.shop.ProductImage
import com.poro.app.data.shop.ShopRepository
import com.poro.app.data.shop.VariantInput
import com.poro.app.ui.auth.ErrorMessage
import com.poro.app.ui.auth.toErrorMessage
import java.io.IOException
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** Déclinaison en cours de saisie ; [variantId] null pour une nouvelle. Prix et stock en texte. */
data class VariantDraft(
    val variantId: String? = null,
    val title: String = "",
    val price: String = "",
    val stock: String = "",
    /** Stock en main enregistré, pour ne l'envoyer que s'il a changé. */
    val savedStock: Int? = null,
    val reserved: Int = 0,
) {
    fun priceIn(currency: String): Long? = Formats.parseMoney(price, currency)?.takeIf { it >= 1 }
    val stockValue: Int? get() = stock.trim().toIntOrNull()?.takeIf { it in 0..1_000_000 }
    fun isValid(currency: String): Boolean = title.trim().length in 1..60 && priceIn(currency) != null && stockValue != null
}

data class ProductEditorUiState(
    val loading: Boolean = false,
    val productId: String? = null,
    val status: String? = null,
    val title: String = "",
    val description: String = "",
    val variants: List<VariantDraft> = listOf(VariantDraft()),
    val images: List<ProductImage> = emptyList(),
    val working: Boolean = false,
    val error: ErrorMessage? = null,
) {
    val descriptionFits: Boolean get() = MultilineText.fits(description, DESCRIPTION_MAX, DESCRIPTION_MAX_LINES)

    fun canSave(currency: String): Boolean =
        !working && title.trim().length in 1..120 && descriptionFits &&
            variants.isNotEmpty() && variants.size <= 50 && variants.all { it.isValid(currency) }

    companion object {
        /** Bornes de `services/shop/src/products/products.service.ts`. */
        const val DESCRIPTION_MAX = 2000
        const val DESCRIPTION_MAX_LINES = 50
    }
}

sealed interface ProductEditorEvent {
    data object Deleted : ProductEditorEvent
}

/**
 * Création et modification d'un produit (brouillon jusqu'à publication). Le stock d'une
 * déclinaison existante passe par sa route dédiée ; celui d'une nouvelle, par sa création.
 */
class ProductEditorViewModel(
    productId: String?,
    private val currency: String,
    private val shops: ShopRepository,
    private val media: LocalMediaReader,
) : ViewModel() {

    private val _state = MutableStateFlow(ProductEditorUiState(productId = productId, loading = productId != null))
    val state: StateFlow<ProductEditorUiState> = _state.asStateFlow()

    private val _events = Channel<ProductEditorEvent>(Channel.BUFFERED)
    val events: Flow<ProductEditorEvent> = _events.receiveAsFlow()

    init {
        productId?.let { id -> viewModelScope.launch { applyResult(shops.myProduct(id)) } }
    }

    fun onTitleChange(value: String) = _state.update { it.copy(title = value.take(120), error = null) }

    fun onDescriptionChange(value: String) = _state.update { it.copy(description = value.take(2 * ProductEditorUiState.DESCRIPTION_MAX), error = null) }

    fun onVariantChange(index: Int, variant: VariantDraft) =
        _state.update { it.copy(variants = it.variants.toMutableList().also { list -> list[index] = variant }, error = null) }

    fun addVariant() = _state.update { if (it.variants.size < 50) it.copy(variants = it.variants + VariantDraft()) else it }

    /** Une déclinaison avec des commandes en cours sera refusée par le serveur (`variant_has_reservations`). */
    fun removeVariant(index: Int) = _state.update {
        if (it.variants.size > 1) it.copy(variants = it.variants.filterIndexed { i, _ -> i != index }) else it
    }

    fun save() {
        val current = _state.value
        if (!current.canSave(currency)) return
        val inputs = current.variants.map { VariantInput(it.variantId, it.title, it.priceIn(currency)!!, it.stockValue) }
        working {
            val id = current.productId
            val saved = if (id == null) {
                shops.createProduct(current.title, current.description, inputs)
            } else {
                shops.updateProduct(id, current.title, current.description, inputs)
            }
            if (saved !is ApiResult.Success || id == null) return@working saved
            // Stock modifié d'une déclinaison existante : une requête par déclinaison changée.
            var last: ApiResult<ProductDto> = saved
            for (draft in current.variants) {
                val variantId = draft.variantId ?: continue
                val stock = draft.stockValue ?: continue
                if (stock != draft.savedStock) {
                    last = shops.setStock(id, variantId, stock)
                    if (last !is ApiResult.Success) break
                }
            }
            last
        }
    }

    /** Publier exige une photo au moins (`image_required`). */
    fun setStatus(status: String) {
        val id = _state.value.productId ?: return
        working { shops.setProductStatus(id, status) }
    }

    fun onImagePicked(uri: String) {
        val id = _state.value.productId ?: return
        viewModelScope.launch {
            val local = media.describe(uri)
            if (local == null || local.mimeType !in MyShopViewModel.IMAGE_TYPES || local.sizeBytes > MyShopViewModel.IMAGE_MAX_BYTES) {
                _state.update { it.copy(error = ErrorMessage(R.string.error_image_invalid)) }
                return@launch
            }
            val bytes = try {
                media.read(uri, 0, local.sizeBytes.toInt())
            } catch (e: IOException) {
                _state.update { it.copy(error = ErrorMessage(R.string.error_image_invalid)) }
                return@launch
            }
            working { shops.uploadProductImage(id, bytes, local.mimeType) }
        }
    }

    fun removeImage(imageId: String) {
        val id = _state.value.productId ?: return
        working { shops.deleteProductImage(id, imageId) }
    }

    fun delete() {
        val id = _state.value.productId ?: return
        if (_state.value.working) return
        _state.update { it.copy(working = true, error = null) }
        viewModelScope.launch {
            when (val result = shops.deleteProduct(id)) {
                is ApiResult.Success -> _events.send(ProductEditorEvent.Deleted)
                else -> _state.update { it.copy(working = false, error = result.toErrorMessage()) }
            }
        }
    }

    private fun working(call: suspend () -> ApiResult<ProductDto>) {
        if (_state.value.working) return
        _state.update { it.copy(working = true, error = null) }
        viewModelScope.launch {
            applyResult(call())
            _state.update { it.copy(working = false) }
        }
    }

    private fun applyResult(result: ApiResult<ProductDto>) {
        when (result) {
            is ApiResult.Success -> {
                val product = result.data
                _state.update {
                    it.copy(
                        loading = false,
                        productId = product.productId,
                        status = product.status,
                        title = product.title,
                        description = product.description.orEmpty(),
                        images = product.images,
                        variants = product.variants.map { v ->
                            VariantDraft(
                                variantId = v.variantId,
                                title = v.title,
                                price = Formats.moneyInput(v.price, currency),
                                stock = (v.stockOnHand ?: 0).toString(),
                                savedStock = v.stockOnHand,
                                reserved = v.stockReserved ?: 0,
                            )
                        }.ifEmpty { listOf(VariantDraft()) },
                        error = null,
                    )
                }
            }
            else -> _state.update { it.copy(loading = false, error = result.toErrorMessage()) }
        }
    }
}
