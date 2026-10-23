package com.poro.app.ui.shop

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.R
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.data.order.OrderRepository
import com.poro.app.data.shop.ShopRepository
import com.poro.app.ui.auth.ErrorMessage
import com.poro.app.ui.auth.toErrorMessage
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class ShopHomeUiState(
    val cartCount: Int = 0,
    /** Null tant qu'on ne sait pas ; détermine « Ouvrir ma boutique » ou « Ma boutique ». */
    val hasShop: Boolean? = null,
    val handle: String = "",
    val finding: Boolean = false,
    val error: ErrorMessage? = null,
)

sealed interface ShopHomeEvent {
    data class OpenShop(val handle: String) : ShopHomeEvent
}

/**
 * Accueil de l'onglet Boutique : recherche dans le catalogue (onglet Produits de la recherche,
 * PORO-026), ou boutique trouvée directement par son identifiant « @… » partagé par le vendeur.
 */
class ShopHomeViewModel(
    private val shops: ShopRepository,
    private val orders: OrderRepository,
) : ViewModel() {

    private val _state = MutableStateFlow(ShopHomeUiState())
    val state: StateFlow<ShopHomeUiState> = _state.asStateFlow()

    private val _events = Channel<ShopHomeEvent>(Channel.BUFFERED)
    val events: Flow<ShopHomeEvent> = _events.receiveAsFlow()

    /** Relu à chaque retour sur l'onglet (le panier a pu changer). */
    fun refresh() {
        viewModelScope.launch {
            (orders.cart() as? ApiResult.Success)?.let { cart -> _state.update { it.copy(cartCount = cart.data.itemCount) } }
        }
        viewModelScope.launch {
            val shop = shops.myShop()
            val hasShop = when {
                shop is ApiResult.Success -> true
                shop is ApiResult.Failure && shop.httpStatus == 404 -> false
                else -> null
            }
            _state.update { it.copy(hasShop = hasShop ?: it.hasShop) }
        }
    }

    fun onHandleChange(value: String) = _state.update { it.copy(handle = value.trim().take(31), error = null) }

    fun find() {
        val handle = _state.value.handle.removePrefix("@").lowercase()
        if (handle.length < 3 || _state.value.finding) return
        _state.update { it.copy(finding = true, error = null) }
        viewModelScope.launch {
            when (val result = shops.shop(handle)) {
                is ApiResult.Success -> {
                    _state.update { it.copy(finding = false) }
                    _events.send(ShopHomeEvent.OpenShop(result.data.handle))
                }
                is ApiResult.Failure -> _state.update {
                    it.copy(
                        finding = false,
                        error = if (result.code == ApiErrorCodes.SHOP_NOT_FOUND || result.httpStatus == 404) {
                            ErrorMessage(R.string.shop_not_found)
                        } else {
                            result.toErrorMessage()
                        },
                    )
                }
                is ApiResult.NetworkError -> _state.update { it.copy(finding = false, error = result.toErrorMessage()) }
            }
        }
    }
}
