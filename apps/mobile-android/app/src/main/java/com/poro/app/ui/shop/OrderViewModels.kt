package com.poro.app.ui.shop

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.data.order.OrderDto
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

data class OrdersUiState(
    val orders: List<OrderDto> = emptyList(),
    val nextCursor: String? = null,
    val loading: Boolean = false,
    val started: Boolean = false,
    val error: ErrorMessage? = null,
) {
    val isEmpty: Boolean get() = started && !loading && orders.isEmpty() && error == null
    val canLoadMore: Boolean get() = started && !loading && nextCursor != null
}

/** Commandes de l'acheteur, ou celles reçues par sa boutique ([asSeller]). */
class OrdersViewModel(private val orders: OrderRepository, private val asSeller: Boolean) : ViewModel() {

    private val _state = MutableStateFlow(OrdersUiState())
    val state: StateFlow<OrdersUiState> = _state.asStateFlow()

    init {
        loadMore()
    }

    fun refresh() {
        _state.value = OrdersUiState()
        loadMore()
    }

    fun loadMore() {
        val current = _state.value
        if (current.loading || (current.started && current.nextCursor == null)) return
        _state.update { it.copy(loading = true, started = true, error = null) }
        viewModelScope.launch {
            val result = if (asSeller) orders.sellerOrders(current.nextCursor) else orders.orders(current.nextCursor)
            _state.update {
                when (result) {
                    is ApiResult.Success -> it.copy(orders = it.orders + result.data.items, nextCursor = result.data.nextCursor, loading = false)
                    else -> it.copy(loading = false, error = result.toErrorMessage())
                }
            }
        }
    }
}

data class OrderUiState(
    val order: Loadable<OrderDto> = Loadable.Loading,
    val working: Boolean = false,
    val tracking: String = "",
    val error: ErrorMessage? = null,
)

/**
 * Une commande. L'acheteur peut l'annuler avant l'expédition et confirmer la réception ; le
 * vendeur peut l'expédier (suivi facultatif) ou l'annuler.
 */
class OrderViewModel(
    private val orderId: String,
    private val orders: OrderRepository,
    private val asSeller: Boolean,
) : ViewModel() {

    private val _state = MutableStateFlow(OrderUiState())
    val state: StateFlow<OrderUiState> = _state.asStateFlow()

    init {
        load()
    }

    fun load() {
        _state.update { it.copy(order = Loadable.Loading) }
        viewModelScope.launch {
            val result = if (asSeller) orders.sellerOrder(orderId) else orders.order(orderId)
            _state.update { it.copy(order = result.toLoadable()) }
        }
    }

    /** Relit la commande sans masquer l'écran ni le message d'erreur. */
    private suspend fun reload() {
        val result = if (asSeller) orders.sellerOrder(orderId) else orders.order(orderId)
        if (result is ApiResult.Success) _state.update { it.copy(order = Loadable.Loaded(result.data)) }
    }

    fun onTrackingChange(value: String) = _state.update { it.copy(tracking = value.take(TRACKING_MAX)) }

    fun cancel() = act { if (asSeller) orders.sellerCancel(orderId) else orders.cancel(orderId) }

    fun confirmReceipt() = act { orders.confirmReceipt(orderId) }

    fun ship() = act { orders.ship(orderId, _state.value.tracking) }

    private fun act(call: suspend () -> ApiResult<OrderDto>) {
        if (_state.value.working) return
        _state.update { it.copy(working = true, error = null) }
        viewModelScope.launch {
            val result = call()
            _state.update {
                it.copy(
                    working = false,
                    order = if (result is ApiResult.Success) Loadable.Loaded(result.data) else it.order,
                    error = result.toErrorMessage(),
                )
            }
            // Statut changé entre-temps (l'autre partie a agi) : afficher le statut réel.
            if (result is ApiResult.Failure && result.code == ApiErrorCodes.ORDER_STATUS_CONFLICT) reload()
        }
    }

    private companion object {
        const val TRACKING_MAX = 200
    }
}
