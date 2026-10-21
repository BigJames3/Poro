package com.poro.app.ui.auth.phone

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.core.network.ApiResult
import com.poro.app.core.phone.LaunchCountry
import com.poro.app.core.phone.PhoneNumber
import com.poro.app.core.time.Clock
import com.poro.app.data.auth.AuthRepository
import com.poro.app.ui.auth.Countdown
import com.poro.app.ui.auth.ErrorMessage
import com.poro.app.ui.auth.RateLimitBackoff
import com.poro.app.ui.auth.toErrorMessage
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class PhoneEntryUiState(
    val country: LaunchCountry = LaunchCountry.DEFAULT,
    val input: String = "",
    val isValid: Boolean = false,
    val sending: Boolean = false,
    val error: ErrorMessage? = null,
    /** Attente imposée après un 429 : le bouton reste bloqué jusqu'à 0. */
    val retryInSeconds: Int = 0,
) {
    val canSubmit: Boolean get() = isValid && !sending && retryInSeconds == 0
}

sealed interface PhoneEntryEvent {
    /** Code envoyé : ouvrir la saisie du code pour [phone] (E.164). */
    data class CodeSent(val phone: String, val expiresInSeconds: Int) : PhoneEntryEvent
}

class PhoneEntryViewModel(
    private val auth: AuthRepository,
    private val savedState: SavedStateHandle,
    clock: Clock,
) : ViewModel() {

    private val _state = MutableStateFlow(
        run {
            val country = savedState.get<String>(KEY_COUNTRY)
                ?.let { name -> LaunchCountry.entries.firstOrNull { it.name == name } }
                ?: LaunchCountry.DEFAULT
            val input = savedState.get<String>(KEY_INPUT).orEmpty()
            PhoneEntryUiState(country = country, input = input, isValid = isValid(input, country))
        },
    )
    val state: StateFlow<PhoneEntryUiState> = _state.asStateFlow()

    private val _events = Channel<PhoneEntryEvent>(Channel.BUFFERED)
    val events: Flow<PhoneEntryEvent> = _events.receiveAsFlow()

    private val backoff = RateLimitBackoff()
    private val retryCountdown = Countdown(viewModelScope, savedState, KEY_RETRY_AT, clock) { seconds ->
        _state.update { it.copy(retryInSeconds = seconds) }
    }

    init {
        retryCountdown.restore()
    }

    fun onCountryChange(country: LaunchCountry) {
        savedState[KEY_COUNTRY] = country.name
        _state.update { it.copy(country = country, isValid = isValid(it.input, country), error = null) }
    }

    fun onInputChange(raw: String) {
        val input = raw.filter { it in '0'..'9' || it in ALLOWED_SYMBOLS }.take(MAX_INPUT)
        savedState[KEY_INPUT] = input
        _state.update { it.copy(input = input, isValid = isValid(input, it.country), error = null) }
    }

    fun submit() {
        val current = _state.value
        if (!current.canSubmit) return
        val phone = PhoneNumber.parse(current.input, current.country) ?: return
        // Un numéro collé au format international peut désigner un autre pays de lancement.
        savedState[KEY_COUNTRY] = phone.country.name
        _state.update { it.copy(sending = true, error = null, country = phone.country) }
        viewModelScope.launch {
            val result = auth.requestOtp(phone.e164)
            _state.update { it.copy(sending = false, error = result.toErrorMessage()) }
            backoff.delayAfter(result)?.let(retryCountdown::start)
            if (result is ApiResult.Success) _events.send(PhoneEntryEvent.CodeSent(phone.e164, result.data.expiresIn))
        }
    }

    private fun isValid(input: String, country: LaunchCountry) = PhoneNumber.parse(input, country) != null

    private companion object {
        const val KEY_COUNTRY = "country"
        const val KEY_INPUT = "input"
        const val KEY_RETRY_AT = "retry_at"
        const val MAX_INPUT = 20
        const val ALLOWED_SYMBOLS = "+ -.()"
    }
}
