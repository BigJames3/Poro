package com.poro.app.ui.shop

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.core.network.ApiResult
import com.poro.app.data.order.OrderRepository
import com.poro.app.data.shop.ProductDto
import com.poro.app.data.shop.ShopRepository
import com.poro.app.ui.auth.ErrorMessage
import com.poro.app.ui.auth.toErrorMessage
import com.poro.app.ui.common.Loadable
import com.poro.app.ui.common.toLoadable
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class ProductUiState(
    val product: Loadable<ProductDto> = Loadable.Loading,
    val variantId: String? = null,
    val quantity: Int = 1,
    val adding: Boolean = false,
    val error: ErrorMessage? = null,
) {
    val canAdd: Boolean get() = variantId != null && !adding
}

sealed interface ProductEvent {
    data object AddedToCart : ProductEvent
}

/** Fiche produit : choix de la déclinaison et de la quantité, ajout au panier. */
class ProductViewModel(
    private val productId: String,
    private val shops: ShopRepository,
    private val orders: OrderRepository,
) : ViewModel() {

    private val _state = MutableStateFlow(ProductUiState())
    val state: StateFlow<ProductUiState> = _state.asStateFlow()

    private val _events = Channel<ProductEvent>(Channel.BUFFERED)
    val events: Flow<ProductEvent> = _events.receiveAsFlow()

    init {
        load()
    }

    fun load() {
        _state.update { it.copy(product = Loadable.Loading) }
        viewModelScope.launch {
            val product = shops.product(productId).toLoadable()
            // Première déclinaison en stock choisie d'office.
            val first = product.valueOrNull?.variants?.let { variants -> variants.firstOrNull { it.inStock } ?: variants.firstOrNull() }
            _state.update { it.copy(product = product, variantId = it.variantId ?: first?.variantId) }
        }
    }

    fun selectVariant(variantId: String) = _state.update { it.copy(variantId = variantId, error = null) }

    fun setQuantity(quantity: Int) = _state.update { it.copy(quantity = quantity.coerceIn(1, MAX_QUANTITY), error = null) }

    fun addToCart() {
        val current = _state.value
        val variantId = current.variantId ?: return
        if (!current.canAdd) return
        _state.update { it.copy(adding = true, error = null) }
        viewModelScope.launch {
            val result = orders.setQuantity(variantId, current.quantity)
            _state.update { it.copy(adding = false, error = result.toErrorMessage()) }
            if (result is ApiResult.Success) _events.send(ProductEvent.AddedToCart)
        }
    }

    private companion object {
        const val MAX_QUANTITY = 99
    }
}
