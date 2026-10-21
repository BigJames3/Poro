package com.poro.app.ui.shop

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.R
import com.poro.app.core.media.LocalMediaReader
import com.poro.app.core.network.ApiResult
import com.poro.app.core.phone.LaunchCountry
import com.poro.app.data.shop.ProductDto
import com.poro.app.data.shop.ShopDto
import com.poro.app.data.shop.ShopRepository
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

/** Formulaire d'ouverture : l'identifiant et le pays (donc la devise) ne changent plus ensuite. */
data class OpenShopForm(
    val name: String = "",
    val handle: String = "",
    val country: LaunchCountry = LaunchCountry.DEFAULT,
    val description: String = "",
) {
    val isValid: Boolean
        get() = name.trim().length in 1..60 && HANDLE.matches(handle.trim().removePrefix("@")) && description.length <= 500

    private companion object {
        val HANDLE = Regex("^[A-Za-z0-9._]{3,30}$")
    }
}

data class MyShopUiState(
    val loading: Boolean = true,
    /** Null : pas encore de boutique (formulaire d'ouverture). */
    val shop: ShopDto? = null,
    val openForm: OpenShopForm = OpenShopForm(),
    val editName: String = "",
    val editDescription: String = "",
    val products: List<ProductDto> = emptyList(),
    val productsCursor: String? = null,
    val working: Boolean = false,
    val error: ErrorMessage? = null,
)

sealed interface MyShopEvent {
    /** Nom et description enregistrés. */
    data object Saved : MyShopEvent
}

/** Sa boutique : ouverture, nom et description, fermeture, logo, et liste de ses produits. */
class MyShopViewModel(
    private val shops: ShopRepository,
    private val media: LocalMediaReader,
) : ViewModel() {

    private val _state = MutableStateFlow(MyShopUiState())
    val state: StateFlow<MyShopUiState> = _state.asStateFlow()

    private val _events = Channel<MyShopEvent>(Channel.BUFFERED)
    val events: Flow<MyShopEvent> = _events.receiveAsFlow()

    fun load() {
        viewModelScope.launch {
            when (val result = shops.myShop()) {
                is ApiResult.Success -> {
                    applyShop(result.data)
                    loadProducts(reset = true)
                }
                is ApiResult.Failure -> _state.update {
                    // 404 : le compte n'a pas encore de boutique.
                    if (result.httpStatus == 404) it.copy(loading = false, shop = null) else it.copy(loading = false, error = result.toErrorMessage())
                }
                is ApiResult.NetworkError -> _state.update { it.copy(loading = false, error = result.toErrorMessage()) }
            }
        }
    }

    fun loadProducts(reset: Boolean = false) {
        val cursor = if (reset) null else _state.value.productsCursor ?: return
        viewModelScope.launch {
            when (val result = shops.myProducts(cursor)) {
                is ApiResult.Success -> _state.update {
                    it.copy(products = (if (reset) emptyList() else it.products) + result.data.items, productsCursor = result.data.nextCursor)
                }
                else -> _state.update { it.copy(error = result.toErrorMessage()) }
            }
        }
    }

    fun onOpenFormChange(form: OpenShopForm) = _state.update { it.copy(openForm = form, error = null) }

    fun openShop() {
        val form = _state.value.openForm
        if (!form.isValid) return
        act { shops.openShop(form.name, form.handle, form.country.name, form.description) }
    }

    fun onEditName(value: String) = _state.update { it.copy(editName = value.take(60), error = null) }

    fun onEditDescription(value: String) = _state.update { it.copy(editDescription = value.take(500), error = null) }

    fun saveDetails() {
        val current = _state.value
        if (current.editName.isBlank()) return
        act(onSuccess = { _events.send(MyShopEvent.Saved) }) { shops.updateMyShop(current.editName, current.editDescription) }
    }

    /** Fermée, la boutique disparaît du catalogue ; rouverte, elle y revient. */
    fun setOpen(open: Boolean) = act { shops.setShopOpen(open) }

    fun onLogoPicked(uri: String) {
        viewModelScope.launch {
            val local = media.describe(uri)
            if (local == null || local.mimeType !in IMAGE_TYPES || local.sizeBytes > IMAGE_MAX_BYTES) {
                _state.update { it.copy(error = ErrorMessage(R.string.error_image_invalid)) }
                return@launch
            }
            val bytes = try {
                media.read(uri, 0, local.sizeBytes.toInt())
            } catch (e: IOException) {
                _state.update { it.copy(error = ErrorMessage(R.string.error_image_invalid)) }
                return@launch
            }
            act { shops.uploadLogo(bytes, local.mimeType) }
        }
    }

    private fun act(onSuccess: suspend () -> Unit = {}, call: suspend () -> ApiResult<ShopDto>) {
        if (_state.value.working) return
        _state.update { it.copy(working = true, error = null) }
        viewModelScope.launch {
            val result = call()
            if (result is ApiResult.Success) applyShop(result.data)
            _state.update { it.copy(working = false, error = result.toErrorMessage()) }
            if (result is ApiResult.Success) onSuccess()
        }
    }

    private fun applyShop(shop: ShopDto) = _state.update {
        it.copy(loading = false, shop = shop, editName = shop.name, editDescription = shop.description.orEmpty())
    }

    internal companion object {
        const val IMAGE_MAX_BYTES = 5L * 1024 * 1024
        val IMAGE_TYPES = setOf("image/jpeg", "image/png", "image/webp")
    }
}
