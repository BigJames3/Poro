package com.poro.app.ui.notification

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.core.network.ApiResult
import com.poro.app.data.notification.NotificationRepository
import com.poro.app.data.notification.PreferencesDto
import com.poro.app.ui.auth.ErrorMessage
import com.poro.app.ui.auth.toErrorMessage
import com.poro.app.ui.common.Loadable
import com.poro.app.ui.common.toLoadable
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class PreferencesUiState(
    val preferences: Loadable<PreferencesDto> = Loadable.Loading,
    val saving: Boolean = false,
    val error: ErrorMessage? = null,
)

/** Chaque interrupteur est enregistré aussitôt ; un refus remet la valeur précédente. */
class NotificationPreferencesViewModel(private val notifications: NotificationRepository) : ViewModel() {

    private val _state = MutableStateFlow(PreferencesUiState())
    val state: StateFlow<PreferencesUiState> = _state.asStateFlow()

    init {
        load()
    }

    fun load() {
        _state.update { it.copy(preferences = Loadable.Loading) }
        viewModelScope.launch {
            val result = notifications.preferences()
            _state.update { it.copy(preferences = result.toLoadable()) }
        }
    }

    fun update(change: (PreferencesDto) -> PreferencesDto) {
        val before = _state.value.preferences.valueOrNull ?: return
        val after = change(before)
        _state.update { it.copy(preferences = Loadable.Loaded(after), saving = true, error = null) }
        viewModelScope.launch {
            val result = notifications.updatePreferences(after)
            _state.update {
                when (result) {
                    is ApiResult.Success -> it.copy(preferences = Loadable.Loaded(result.data), saving = false)
                    else -> it.copy(preferences = Loadable.Loaded(before), saving = false, error = result.toErrorMessage())
                }
            }
        }
    }
}
