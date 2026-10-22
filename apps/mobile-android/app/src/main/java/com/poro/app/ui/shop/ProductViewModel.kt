package com.poro.app.ui.shop

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.core.network.ApiResult
import com.poro.app.data.order.CartDto
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
    /** Quantité déjà au panier, par déclinaison. */
    val inCart: Map<String, Int> = emptyMap(),
    /** Produit de sa propre boutique : on ne peut pas l'acheter (`own_shop`). */
    val ownProduct: Boolean = false,
    val adding: Boolean = false,
    val error: ErrorMessage? = null,
) {
    /** Déjà au panier pour la déclinaison choisie. */
    val selectedInCart: Int get() = variantId?.let { inCart[it] } ?: 0

    /** Le panier accepte 99 articles d'une même déclinaison au plus. */
    val atMaximum: Boolean get() = selectedInCart >= MAX_QUANTITY

    val canAdd: Boolean get() = variantId != null && !adding && !ownProduct && !atMaximum

    companion object {
        const val MAX_QUANTITY = 99
    }
}

sealed interface ProductEvent {
    /** [quantityInCart] : quantité de la déclinaison au panier après l'ajout. */
    data class AddedToCart(val quantityInCart: Int) : ProductEvent
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
            product.valueOrNull?.let { loaded ->
                // Sans réponse (réseau), le serveur refusera de toute façon l'achat à son propriétaire.
                val myShopId = (shops.myShop() as? ApiResult.Success)?.data?.shopId
                _state.update { it.copy(ownProduct = myShopId != null && myShopId == loaded.shop.shopId) }
            }
        }
        viewModelScope.launch { refreshCart() }
    }

    fun selectVariant(variantId: String) = _state.update { it.copy(variantId = variantId, quantity = 1, error = null) }

    fun setQuantity(quantity: Int) = _state.update {
        it.copy(quantity = quantity.coerceIn(1, (ProductUiState.MAX_QUANTITY - it.selectedInCart).coerceAtLeast(1)), error = null)
    }

    /**
     * Ajoute la quantité choisie à celle déjà au panier : l'API fixe la quantité d'une ligne
     * (`PUT`), on relit donc le panier juste avant pour ne pas écraser un ajout précédent.
     */
    fun addToCart() {
        val current = _state.value
        val variantId = current.variantId ?: return
        if (!current.canAdd) return
        _state.update { it.copy(adding = true, error = null) }
        viewModelScope.launch {
            val existing = refreshCart()?.get(variantId) ?: current.selectedInCart
            val target = (existing + current.quantity).coerceAtMost(ProductUiState.MAX_QUANTITY)
            when (val result = orders.setQuantity(variantId, target)) {
                is ApiResult.Success -> {
                    val quantities = result.data.quantities()
                    _state.update { it.copy(adding = false, quantity = 1, inCart = quantities) }
                    _events.send(ProductEvent.AddedToCart(quantities[variantId] ?: target))
                }
                else -> _state.update { it.copy(adding = false, error = result.toErrorMessage()) }
            }
        }
    }

    /** Relit le panier ; null si la lecture échoue (la dernière valeur connue est gardée). */
    private suspend fun refreshCart(): Map<String, Int>? {
        val cart = (orders.cart() as? ApiResult.Success)?.data ?: return null
        val quantities = cart.quantities()
        _state.update { it.copy(inCart = quantities) }
        return quantities
    }
}

private fun CartDto.quantities(): Map<String, Int> =
    (shops.flatMap { it.items } + unavailable).associate { it.variantId to it.quantity }
