package com.poro.app.ui.shop

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.R
import com.poro.app.core.network.ApiResult
import com.poro.app.data.order.AddressDto
import com.poro.app.data.order.AddressInput
import com.poro.app.data.order.OrderRepository
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

data class AddressesUiState(
    val list: Loadable<List<AddressDto>> = Loadable.Loading,
    val form: DeliveryForm = DeliveryForm(),
    val label: String = "",
    val editingId: String? = null,
    val showForm: Boolean = false,
    val working: Boolean = false,
    val error: ErrorMessage? = null,
    /** Le carnet a changé depuis l'ouverture : l'écran précédent (checkout) doit le relire. */
    val changed: Boolean = false,
) {
    val canSave: Boolean get() = form.isValid && label.length <= 40 && !working
}

sealed interface AddressesEvent {
    data object Saved : AddressesEvent
    data object Deleted : AddressesEvent
}

class AddressesViewModel(private val orders: OrderRepository) : ViewModel() {

    private val _state = MutableStateFlow(AddressesUiState())
    val state: StateFlow<AddressesUiState> = _state.asStateFlow()

    private val _events = Channel<AddressesEvent>(Channel.BUFFERED)
    val events: Flow<AddressesEvent> = _events.receiveAsFlow()

    init {
        load()
    }

    fun load() {
        viewModelScope.launch { _state.update { it.copy(list = orders.addresses().toLoadable(), error = null) } }
    }

    fun startCreate() = _state.update {
        it.copy(showForm = true, editingId = null, form = DeliveryForm(), label = "", error = null)
    }

    fun startEdit(address: AddressDto) = _state.update {
        it.copy(
            showForm = true,
            editingId = address.addressId,
            form = DeliveryForm(address.fullName, address.phone, address.city, address.address.orEmpty(), address.landmark.orEmpty()),
            label = address.label.orEmpty(),
            error = null,
        )
    }

    fun cancelForm() = _state.update { it.copy(showForm = false, editingId = null, error = null) }

    fun onFormChange(form: DeliveryForm) = _state.update { it.copy(form = form, error = null) }

    fun onLabelChange(value: String) = _state.update { it.copy(label = value.take(40), error = null) }

    fun save() {
        val current = _state.value
        if (!current.canSave) return
        _state.update { it.copy(working = true, error = null) }
        viewModelScope.launch {
            val input = AddressInput(
                fullName = current.form.fullName.trim(),
                phone = current.form.phone.trim(),
                city = current.form.city.trim(),
                address = current.form.address.trim().ifEmpty { null },
                landmark = current.form.landmark.trim().ifEmpty { null },
                label = current.label.trim().ifEmpty { null },
            )
            when (val result = orders.saveAddress(input, current.editingId)) {
                is ApiResult.Success -> {
                    _state.update { it.copy(working = false, showForm = false, editingId = null, changed = true) }
                    load()
                    _events.send(AddressesEvent.Saved)
                }
                else -> _state.update { it.copy(working = false, error = result.toErrorMessage()) }
            }
        }
    }

    fun setDefault(addressId: String) {
        if (_state.value.working) return
        _state.update { it.copy(working = true, error = null) }
        viewModelScope.launch {
            val result = orders.setDefaultAddress(addressId)
            if (result is ApiResult.Success) load()
            _state.update { it.copy(working = false, error = result.toErrorMessage(), changed = it.changed || result is ApiResult.Success) }
        }
    }

    fun delete(addressId: String) {
        if (_state.value.working) return
        _state.update { it.copy(working = true, error = null) }
        viewModelScope.launch {
            when (val result = orders.deleteAddress(addressId)) {
                is ApiResult.Success -> {
                    _state.update { it.copy(working = false, changed = true) }
                    load()
                    _events.send(AddressesEvent.Deleted)
                }
                else -> _state.update { it.copy(working = false, error = result.toErrorMessage()) }
            }
        }
    }
}
