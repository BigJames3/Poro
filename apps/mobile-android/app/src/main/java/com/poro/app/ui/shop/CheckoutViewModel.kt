package com.poro.app.ui.shop

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.R
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.core.text.MultilineText
import com.poro.app.data.order.AddressDto
import com.poro.app.data.order.Delivery
import com.poro.app.data.order.OrderDto
import com.poro.app.data.order.OrderRepository
import com.poro.app.data.order.PreviewDto
import com.poro.app.data.order.PreviewRequest
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

data class DeliveryForm(
    val fullName: String = "",
    val phone: String = "",
    val city: String = "",
    val address: String = "",
    val landmark: String = "",
) {
    /** Adresse trop courte (3 caractères au moins côté serveur), signalée dès qu'elle est saisie. */
    val addressTooShort: Boolean get() = address.isNotBlank() && address.trim().length < MIN_TEXT

    val addressFits: Boolean get() = MultilineText.fits(address, MAX_TEXT, MAX_LINES)
    val landmarkFits: Boolean get() = MultilineText.fits(landmark, MAX_TEXT, MAX_LINES)

    /** Point de repère facultatif, mais de 3 caractères au moins s'il est rempli. */
    val landmarkTooShort: Boolean get() = landmark.isNotBlank() && landmark.trim().length < MIN_TEXT

    /**
     * Nom, téléphone international (+…) et ville requis, plus une adresse (pas de position GPS
     * ici) ; mêmes bornes que le service order (`services/order/src/delivery/delivery.ts`).
     */
    val isValid: Boolean
        get() = fullName.trim().length in 1..80 && PHONE.matches(phone.trim()) && city.trim().length in 1..80 &&
            address.trim().length >= MIN_TEXT && addressFits && (landmark.isBlank() || landmark.trim().length >= MIN_TEXT) && landmarkFits

    fun toDelivery() = Delivery(
        fullName = fullName.trim(),
        phone = phone.trim(),
        city = city.trim(),
        address = address.trim().ifEmpty { null },
        landmark = landmark.trim().ifEmpty { null },
    )

    private companion object {
        val PHONE = Regex("^\\+[1-9]\\d{7,14}$")
        const val MIN_TEXT = 3
        const val MAX_TEXT = 300
        const val MAX_LINES = 3
    }
}

data class CheckoutUiState(
    val loading: Boolean = true,
    val form: DeliveryForm = DeliveryForm(),
    val addresses: List<AddressDto> = emptyList(),
    val saveAddress: Boolean = false,
    /** Récapitulatif à relire ; null pendant la saisie de la livraison. */
    val preview: PreviewDto? = null,
    val working: Boolean = false,
    val error: ErrorMessage? = null,
)

sealed interface CheckoutEvent {
    data class Placed(val orders: List<OrderDto>) : CheckoutEvent
}

/**
 * Commande en deux étapes : livraison (préremplie, carnet d'adresses), puis récapitulatif à
 * confirmer explicitement. Paiement à la livraison : le paiement en ligne (Wave) n'est pas encore
 * proposé par le backend.
 */
class CheckoutViewModel(
    private val orders: OrderRepository,
    private val savedState: SavedStateHandle,
) : ViewModel() {

    private val _state = MutableStateFlow(CheckoutUiState())
    val state: StateFlow<CheckoutUiState> = _state.asStateFlow()

    private val _events = Channel<CheckoutEvent>(Channel.BUFFERED)
    val events: Flow<CheckoutEvent> = _events.receiveAsFlow()

    init {
        viewModelScope.launch {
            val addresses = (orders.addresses() as? ApiResult.Success)?.data.orEmpty()
            val restored = savedState.get<String>(KEY_NAME)?.let {
                DeliveryForm(it, savedState[KEY_PHONE] ?: "", savedState[KEY_CITY] ?: "", savedState[KEY_ADDRESS] ?: "", savedState[KEY_LANDMARK] ?: "")
            }
            val form = restored ?: (orders.prefill() as? ApiResult.Success)?.data?.delivery?.let {
                DeliveryForm(it.fullName.orEmpty(), it.phone.orEmpty(), it.city.orEmpty(), it.address.orEmpty(), it.landmark.orEmpty())
            } ?: DeliveryForm()
            _state.update { it.copy(loading = false, form = form, addresses = addresses) }
        }
    }

    fun onFormChange(form: DeliveryForm) {
        savedState[KEY_NAME] = form.fullName
        savedState[KEY_PHONE] = form.phone
        savedState[KEY_CITY] = form.city
        savedState[KEY_ADDRESS] = form.address
        savedState[KEY_LANDMARK] = form.landmark
        _state.update { it.copy(form = form, error = null) }
    }

    fun useAddress(address: AddressDto) =
        onFormChange(DeliveryForm(address.fullName, address.phone, address.city, address.address.orEmpty(), address.landmark.orEmpty()))

    fun onSaveAddressChange(value: Boolean) = _state.update { it.copy(saveAddress = value) }

    /** Étape 1 → 2 : le serveur valide le panier et la livraison et renvoie le récapitulatif. */
    fun review() {
        val current = _state.value
        if (!current.form.isValid || current.working) return
        _state.update { it.copy(working = true, error = null) }
        viewModelScope.launch {
            val result = orders.preview(PreviewRequest(current.form.toDelivery(), saveAddress = current.saveAddress))
            _state.update {
                when (result) {
                    is ApiResult.Success -> it.copy(working = false, preview = result.data)
                    else -> it.copy(working = false, error = result.toErrorMessage())
                }
            }
        }
    }

    /** Retour à la livraison pour la modifier. */
    fun edit() = _state.update { it.copy(preview = null, error = null) }

    fun confirm() {
        val preview = _state.value.preview ?: return
        if (_state.value.working) return
        _state.update { it.copy(working = true, error = null) }
        viewModelScope.launch {
            when (val result = orders.confirm(preview.previewId)) {
                is ApiResult.Success -> {
                    _state.update { it.copy(working = false) }
                    _events.send(CheckoutEvent.Placed(result.data.orders))
                }
                is ApiResult.Failure -> {
                    // Panier ou prix changés, récapitulatif expiré : il faut le relire.
                    val stale = result.code == ApiErrorCodes.PREVIEW_OUTDATED || result.code == ApiErrorCodes.PREVIEW_EXPIRED ||
                        result.code == ApiErrorCodes.PREVIEW_NOT_FOUND
                    _state.update {
                        it.copy(
                            working = false,
                            preview = if (stale) null else it.preview,
                            error = if (stale) ErrorMessage(R.string.checkout_preview_outdated) else result.toErrorMessage(),
                        )
                    }
                }
                is ApiResult.NetworkError -> _state.update { it.copy(working = false, error = result.toErrorMessage()) }
            }
        }
    }

    private companion object {
        const val KEY_NAME = "full_name"
        const val KEY_PHONE = "phone"
        const val KEY_CITY = "city"
        const val KEY_ADDRESS = "address"
        const val KEY_LANDMARK = "landmark"
    }
}
