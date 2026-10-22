package com.poro.app.ui.auth.email

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
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

data class EmailVerifyUiState(
    val code: String = "",
    val verifying: Boolean = false,
    val resending: Boolean = false,
    val error: ErrorMessage? = null,
    /** Le code actuel ne peut plus réussir (expiré, trop d'essais) : en demander un autre. */
    val needsNewCode: Boolean = false,
    val codeResent: Boolean = false,
    val resendInSeconds: Int = 0,
) {
    val canVerify: Boolean get() = code.length == CODE_LENGTH && !verifying && !needsNewCode
    val canResend: Boolean get() = resendInSeconds == 0 && !resending && !verifying

    companion object {
        const val CODE_LENGTH = 6
    }
}

sealed interface EmailVerifyEvent {
    data object Verified : EmailVerifyEvent
}

/**
 * Vérification de l'adresse (session ouverte, appels avec le Bearer). Juste après l'inscription,
 * [codeAlreadySent] : le renvoi n'est possible qu'après 60 s, comme côté serveur. Depuis le
 * profil, aucun code n'est parti : on en demande un à l'ouverture.
 */
class EmailVerifyViewModel(
    codeAlreadySent: Boolean,
    private val auth: AuthRepository,
    savedState: SavedStateHandle,
    clock: Clock,
) : ViewModel() {

    private val _state = MutableStateFlow(EmailVerifyUiState())
    val state: StateFlow<EmailVerifyUiState> = _state.asStateFlow()

    private val _events = Channel<EmailVerifyEvent>(Channel.BUFFERED)
    val events: Flow<EmailVerifyEvent> = _events.receiveAsFlow()

    private val backoff = RateLimitBackoff()
    private val resendCountdown = Countdown(viewModelScope, savedState, KEY_RESEND_AT, clock) { seconds ->
        _state.update { it.copy(resendInSeconds = seconds) }
    }

    init {
        when {
            resendCountdown.isSet -> resendCountdown.restore()
            codeAlreadySent -> resendCountdown.start(RESEND_COOLDOWN_SECONDS)
            else -> sendCode()
        }
    }

    fun onCodeChange(raw: String) {
        if (_state.value.verifying) return
        val code = raw.filter { it in '0'..'9' }.take(EmailVerifyUiState.CODE_LENGTH)
        _state.update { it.copy(code = code, error = if (code.isEmpty()) it.error else null, codeResent = false) }
        if (code.length == EmailVerifyUiState.CODE_LENGTH) verify()
    }

    fun verify() {
        val current = _state.value
        if (!current.canVerify) return
        _state.update { it.copy(verifying = true, error = null) }
        viewModelScope.launch {
            val result = auth.confirmEmailVerification(current.code)
            if (result is ApiResult.Success) {
                _state.update { it.copy(verifying = false) }
                _events.send(EmailVerifyEvent.Verified)
                return@launch
            }
            val code = (result as? ApiResult.Failure)?.code
            val needsNewCode = code == ApiErrorCodes.CODE_EXPIRED || code == ApiErrorCodes.CODE_TOO_MANY_ATTEMPTS
            val clear = needsNewCode || code == ApiErrorCodes.CODE_INVALID
            _state.update {
                it.copy(
                    verifying = false,
                    error = result.toErrorMessage(),
                    code = if (clear) "" else it.code,
                    needsNewCode = it.needsNewCode || needsNewCode,
                )
            }
        }
    }

    fun resend() {
        if (_state.value.canResend) sendCode()
    }

    private fun sendCode() {
        _state.update { it.copy(resending = true, error = null, codeResent = false) }
        viewModelScope.launch {
            val result = auth.requestEmailVerification()
            if ((result as? ApiResult.Failure)?.code == ApiErrorCodes.EMAIL_ALREADY_VERIFIED) {
                // Vérifiée entre-temps (lien ouvert sur un autre appareil) : rien à saisir.
                _state.update { it.copy(resending = false) }
                _events.send(EmailVerifyEvent.Verified)
                return@launch
            }
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

        /** Une adresse reçoit au plus un email par minute. */
        const val RESEND_COOLDOWN_SECONDS = 60L
    }
}
