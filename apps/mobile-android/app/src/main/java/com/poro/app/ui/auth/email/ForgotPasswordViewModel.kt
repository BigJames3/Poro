package com.poro.app.ui.auth.email

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.core.account.AccountRules
import com.poro.app.core.network.ApiResult
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

data class ForgotPasswordUiState(
    val email: String = "",
    val sending: Boolean = false,
    val error: ErrorMessage? = null,
    val retryInSeconds: Int = 0,
) {
    val canSubmit: Boolean get() = AccountRules.isValidEmail(email) && !sending && retryInSeconds == 0
}

sealed interface ForgotPasswordEvent {
    /** Code demandé pour [email] (normalisée). Le serveur ne dit pas si l'adresse a un compte. */
    data class CodeSent(val email: String) : ForgotPasswordEvent
}

class ForgotPasswordViewModel(
    initialEmail: String,
    private val auth: AuthRepository,
    private val savedState: SavedStateHandle,
    clock: Clock,
) : ViewModel() {

    private val _state = MutableStateFlow(ForgotPasswordUiState(email = savedState.get<String>(KEY_EMAIL) ?: initialEmail))
    val state: StateFlow<ForgotPasswordUiState> = _state.asStateFlow()

    private val _events = Channel<ForgotPasswordEvent>(Channel.BUFFERED)
    val events: Flow<ForgotPasswordEvent> = _events.receiveAsFlow()

    private val backoff = RateLimitBackoff()
    private val retryCountdown = Countdown(viewModelScope, savedState, KEY_RETRY_AT, clock) { seconds ->
        _state.update { it.copy(retryInSeconds = seconds) }
    }

    init {
        retryCountdown.restore()
    }

    fun onEmailChange(email: String) {
        val value = email.take(AccountRules.EMAIL_MAX)
        savedState[KEY_EMAIL] = value
        _state.update { it.copy(email = value, error = null) }
    }

    fun submit() {
        val current = _state.value
        if (!current.canSubmit) return
        val email = AccountRules.normalizeEmail(current.email)
        _state.update { it.copy(sending = true, error = null) }
        viewModelScope.launch {
            val result = auth.forgotPassword(email)
            backoff.delayAfter(result)?.let(retryCountdown::start)
            _state.update { it.copy(sending = false, error = result.toErrorMessage()) }
            if (result is ApiResult.Success) _events.send(ForgotPasswordEvent.CodeSent(email))
        }
    }

    private companion object {
        const val KEY_EMAIL = "email_input"
        const val KEY_RETRY_AT = "retry_at"
    }
}
