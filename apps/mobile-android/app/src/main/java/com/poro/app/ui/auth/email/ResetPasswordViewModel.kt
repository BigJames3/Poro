package com.poro.app.ui.auth.email

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.core.account.AccountRules
import com.poro.app.core.network.ApiErrorCodes
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

data class ResetPasswordUiState(
    /** Adresse à laquelle le code a été envoyé. */
    val email: String,
    val code: String = "",
    val newPassword: String = "",
    val submitting: Boolean = false,
    val resending: Boolean = false,
    val error: ErrorMessage? = null,
    val needsNewCode: Boolean = false,
    val codeResent: Boolean = false,
    val resendInSeconds: Int = 0,
) {
    val passwordValid: Boolean get() = AccountRules.isValidNewPassword(newPassword)
    val canSubmit: Boolean
        get() = code.length == CODE_LENGTH && passwordValid && !submitting && !needsNewCode
    val canResend: Boolean get() = resendInSeconds == 0 && !resending && !submitting

    companion object {
        const val CODE_LENGTH = 6
    }
}

sealed interface ResetPasswordEvent {
    /** Mot de passe changé : tous les appareils sont déconnectés, il faut se reconnecter. */
    data object PasswordChanged : ResetPasswordEvent
}

/** Pas de soumission automatique au 6e chiffre : le nouveau mot de passe est aussi requis. */
class ResetPasswordViewModel(
    email: String,
    private val auth: AuthRepository,
    savedState: SavedStateHandle,
    clock: Clock,
) : ViewModel() {

    private val _state = MutableStateFlow(ResetPasswordUiState(email = email))
    val state: StateFlow<ResetPasswordUiState> = _state.asStateFlow()

    private val _events = Channel<ResetPasswordEvent>(Channel.BUFFERED)
    val events: Flow<ResetPasswordEvent> = _events.receiveAsFlow()

    private val backoff = RateLimitBackoff()
    private val resendCountdown = Countdown(viewModelScope, savedState, KEY_RESEND_AT, clock) { seconds ->
        _state.update { it.copy(resendInSeconds = seconds) }
    }

    init {
        // Le code vient d'être demandé par l'écran précédent : un email par minute au plus.
        if (resendCountdown.isSet) resendCountdown.restore() else resendCountdown.start(RESEND_COOLDOWN_SECONDS)
    }

    fun onCodeChange(raw: String) {
        if (_state.value.submitting) return
        val code = raw.filter { it in '0'..'9' }.take(ResetPasswordUiState.CODE_LENGTH)
        _state.update { it.copy(code = code, error = null, codeResent = false) }
    }

    fun onPasswordChange(password: String) {
        _state.update { it.copy(newPassword = password.take(AccountRules.PASSWORD_MAX), error = null) }
    }

    fun submit() {
        val current = _state.value
        if (!current.canSubmit) return
        _state.update { it.copy(submitting = true, error = null) }
        viewModelScope.launch {
            val result = auth.resetPassword(current.email, current.code, current.newPassword)
            if (result is ApiResult.Success) {
                _state.update { it.copy(submitting = false) }
                _events.send(ResetPasswordEvent.PasswordChanged)
                return@launch
            }
            val code = (result as? ApiResult.Failure)?.code
            val needsNewCode = code == ApiErrorCodes.CODE_EXPIRED || code == ApiErrorCodes.CODE_TOO_MANY_ATTEMPTS
            val clear = needsNewCode || code == ApiErrorCodes.CODE_INVALID
            _state.update {
                it.copy(
                    submitting = false,
                    error = result.toErrorMessage(),
                    code = if (clear) "" else it.code,
                    needsNewCode = it.needsNewCode || needsNewCode,
                )
            }
        }
    }

    fun resend() {
        val current = _state.value
        if (!current.canResend) return
        _state.update { it.copy(resending = true, error = null, codeResent = false) }
        viewModelScope.launch {
            val result = auth.forgotPassword(current.email)
            backoff.delayAfter(result)?.let(resendCountdown::start)
            if (result is ApiResult.Success) {
                resendCountdown.start(RESEND_COOLDOWN_SECONDS)
                _state.update { it.copy(resending = false, code = "", needsNewCode = false, codeResent = true) }
            } else {
                _state.update { it.copy(resending = false, error = result.toErrorMessage()) }
            }
        }
    }

    private companion object {
        const val KEY_RESEND_AT = "resend_at"
        const val RESEND_COOLDOWN_SECONDS = 60L
    }
}
