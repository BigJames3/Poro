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

data class EmailRegisterUiState(
    val fullName: String = "",
    val email: String = "",
    val password: String = "",
    val registering: Boolean = false,
    val error: ErrorMessage? = null,
    val retryInSeconds: Int = 0,
) {
    val fullNameValid: Boolean get() = AccountRules.isValidFullName(fullName)
    val emailValid: Boolean get() = AccountRules.isValidEmail(email)
    val passwordValid: Boolean get() = AccountRules.isValidNewPassword(password)
    val canSubmit: Boolean
        get() = fullNameValid && emailValid && passwordValid && !registering && retryInSeconds == 0
}

sealed interface EmailRegisterEvent {
    /** Compte créé et session ouverte ; un code de vérification est parti par email. */
    data object Registered : EmailRegisterEvent
}

/** Nom et adresse survivent à la mort du processus ; le mot de passe reste en mémoire seulement. */
class EmailRegisterViewModel(
    private val auth: AuthRepository,
    private val savedState: SavedStateHandle,
    clock: Clock,
) : ViewModel() {

    private val _state = MutableStateFlow(
        EmailRegisterUiState(
            fullName = savedState.get<String>(KEY_FULL_NAME).orEmpty(),
            email = savedState.get<String>(KEY_EMAIL).orEmpty(),
        ),
    )
    val state: StateFlow<EmailRegisterUiState> = _state.asStateFlow()

    private val _events = Channel<EmailRegisterEvent>(Channel.BUFFERED)
    val events: Flow<EmailRegisterEvent> = _events.receiveAsFlow()

    private val backoff = RateLimitBackoff()
    private val retryCountdown = Countdown(viewModelScope, savedState, KEY_RETRY_AT, clock) { seconds ->
        _state.update { it.copy(retryInSeconds = seconds) }
    }

    init {
        retryCountdown.restore()
    }

    fun onFullNameChange(fullName: String) {
        val value = fullName.take(AccountRules.FULL_NAME_MAX)
        savedState[KEY_FULL_NAME] = value
        _state.update { it.copy(fullName = value, error = null) }
    }

    fun onEmailChange(email: String) {
        val value = email.take(AccountRules.EMAIL_MAX)
        savedState[KEY_EMAIL] = value
        _state.update { it.copy(email = value, error = null) }
    }

    fun onPasswordChange(password: String) {
        _state.update { it.copy(password = password.take(AccountRules.PASSWORD_MAX), error = null) }
    }

    fun submit() {
        val current = _state.value
        if (!current.canSubmit) return
        _state.update { it.copy(registering = true, error = null) }
        viewModelScope.launch {
            val result = auth.register(
                email = AccountRules.normalizeEmail(current.email),
                password = current.password,
                fullName = current.fullName.trim(),
            )
            backoff.delayAfter(result)?.let(retryCountdown::start)
            _state.update { it.copy(registering = false, error = result.toErrorMessage()) }
            if (result is ApiResult.Success) _events.send(EmailRegisterEvent.Registered)
        }
    }

    private companion object {
        const val KEY_FULL_NAME = "full_name"
        const val KEY_EMAIL = "email"
        const val KEY_RETRY_AT = "retry_at"
    }
}
