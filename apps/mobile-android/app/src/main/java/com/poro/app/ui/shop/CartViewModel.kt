package com.poro.app.ui.shop

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.core.network.ApiResult
import com.poro.app.data.order.CartDto
import com.poro.app.data.order.OrderRepository
import com.poro.app.ui.auth.ErrorMessage
import com.poro.app.ui.auth.toErrorMessage
import com.poro.app.ui.common.Loadable
import com.poro.app.ui.common.toLoadable
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class CartUiState(
    val cart: Loadable<CartDto> = Loadable.Loading,
    /** Lignes en cours de modification (boutons désactivés). */
    val busy: Set<String> = emptySet(),
    val error: ErrorMessage? = null,
) {
    /** Commander exige au moins un article disponible et aucun article indisponible restant. */
    val canCheckout: Boolean
        get() = cart.valueOrNull?.let { it.shops.isNotEmpty() && it.unavailable.isEmpty() } == true && busy.isEmpty()
}

/** Panier par boutique ; chaque modification renvoie le panier à jour (prix du catalogue). */
class CartViewModel(private val orders: OrderRepository) : ViewModel() {

    private val _state = MutableStateFlow(CartUiState())
    val state: StateFlow<CartUiState> = _state.asStateFlow()

    init {
        load()
    }

    fun load() {
        viewModelScope.launch { _state.update { it.copy(cart = orders.cart().toLoadable()) } }
    }

    fun setQuantity(variantId: String, quantity: Int) {
        if (quantity < 1) return remove(variantId)
        change(variantId) { orders.setQuantity(variantId, quantity.coerceAtMost(99)) }
    }

    fun remove(variantId: String) = change(variantId) { orders.removeItem(variantId) }

    private fun change(variantId: String, call: suspend () -> ApiResult<CartDto>) {
        if (variantId in _state.value.busy) return
        _state.update { it.copy(busy = it.busy + variantId, error = null) }
        viewModelScope.launch {
            val result = call()
            _state.update {
                it.copy(
                    busy = it.busy - variantId,
                    cart = if (result is ApiResult.Success) Loadable.Loaded(result.data) else it.cart,
                    error = result.toErrorMessage(),
                )
            }
        }
    }
}
