package com.poro.app.ui.shop

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.core.network.ApiResult
import com.poro.app.data.shop.ProductDto
import com.poro.app.data.shop.ShopDto
import com.poro.app.data.shop.ShopRepository
import com.poro.app.ui.auth.ErrorMessage
import com.poro.app.ui.auth.toErrorMessage
import com.poro.app.ui.common.Loadable
import com.poro.app.ui.common.toLoadable
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class ShopUiState(
    val shop: Loadable<ShopDto> = Loadable.Loading,
    val products: List<ProductDto> = emptyList(),
    val nextCursor: String? = null,
    val loading: Boolean = false,
    val started: Boolean = false,
    val error: ErrorMessage? = null,
) {
    val canLoadMore: Boolean get() = started && !loading && nextCursor != null
}

/** Vitrine d'une boutique active : en-tête et produits publiés, page par page. */
class ShopViewModel(private val handle: String, private val shops: ShopRepository) : ViewModel() {

    private val _state = MutableStateFlow(ShopUiState())
    val state: StateFlow<ShopUiState> = _state.asStateFlow()

    init {
        load()
    }

    fun load() {
        _state.value = ShopUiState()
        viewModelScope.launch { _state.update { it.copy(shop = shops.shop(handle).toLoadable()) } }
        loadMore()
    }

    fun loadMore() {
        val current = _state.value
        if (current.loading || (current.started && current.nextCursor == null)) return
        _state.update { it.copy(loading = true, started = true, error = null) }
        viewModelScope.launch {
            when (val result = shops.shopProducts(handle, current.nextCursor)) {
                is ApiResult.Success -> _state.update {
                    it.copy(products = it.products + result.data.items, nextCursor = result.data.nextCursor, loading = false)
                }
                else -> _state.update { it.copy(loading = false, error = result.toErrorMessage()) }
            }
        }
    }
}
