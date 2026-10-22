package com.poro.app.ui.auth.otp

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

data class OtpUiState(
    /** Numéro E.164 auquel le code a été envoyé. */
    val phone: String,
    val code: String = "",
    val verifying: Boolean = false,
    val resending: Boolean = false,
    val error: ErrorMessage? = null,
    /** Le code actuel ne peut plus réussir (expiré, trop d'essais) : en demander un autre. */
    val needsNewCode: Boolean = false,
    /** Un nouveau code vient d'être envoyé (confirmation affichée). */
    val codeResent: Boolean = false,
    val resendInSeconds: Int = 0,
    val expiresInSeconds: Int = 0,
) {
    val canVerify: Boolean get() = code.length == CODE_LENGTH && !verifying && !needsNewCode
    val canResend: Boolean get() = resendInSeconds == 0 && !resending && !verifying

    companion object {
        const val CODE_LENGTH = 6
    }
}

sealed interface OtpEvent {
    /** Session enregistrée. [deletionCancelled] : la suppression programmée du compte est annulée. */
    data class SignedIn(val deletionCancelled: Boolean) : OtpEvent
}

/**
 * Saisie du code reçu par SMS. Les échéances (renvoi possible, expiration du code) sont des
 * [Countdown] : elles survivent à la mort du processus.
 */
class OtpVerifyViewModel(
    phone: String,
    expiresInSeconds: Int,
    private val auth: AuthRepository,
    savedState: SavedStateHandle,
    clock: Clock,
) : ViewModel() {

    private val _state = MutableStateFlow(OtpUiState(phone = phone))
    val state: StateFlow<OtpUiState> = _state.asStateFlow()

    private val _events = Channel<OtpEvent>(Channel.BUFFERED)
    val events: Flow<OtpEvent> = _events.receiveAsFlow()

    private val backoff = RateLimitBackoff()
    private val resendCountdown = Countdown(viewModelScope, savedState, KEY_RESEND_AT, clock) { seconds ->
        _state.update { it.copy(resendInSeconds = seconds) }
    }
    private val expiryCountdown = Countdown(viewModelScope, savedState, KEY_EXPIRES_AT, clock) { seconds ->
        _state.update { it.copy(expiresInSeconds = seconds) }
    }

    init {
        if (resendCountdown.isSet) {
            resendCountdown.restore()
            expiryCountdown.restore()
        } else {
            // Première ouverture : le code vient d'être envoyé par l'écran précédent.
            codeSent(expiresInSeconds)
        }
    }

    fun onCodeChange(raw: String) {
        if (_state.value.verifying) return
        val code = raw.filter { it in '0'..'9' }.take(OtpUiState.CODE_LENGTH)
        _state.update { it.copy(code = code, error = if (code.isEmpty()) it.error else null, codeResent = false) }
        // Saisie complète (frappe, collage ou remplissage automatique du SMS) : vérification immédiate.
        if (code.length == OtpUiState.CODE_LENGTH) verify()
    }

    fun verify() {
        val current = _state.value
        if (!current.canVerify) return
        _state.update { it.copy(verifying = true, error = null) }
        viewModelScope.launch {
            when (val result = auth.verifyOtp(current.phone, current.code)) {
                is ApiResult.Success -> {
                    _state.update { it.copy(verifying = false) }
                    _events.send(OtpEvent.SignedIn(result.data.deletionCancelled))
                }
                is ApiResult.Failure -> {
                    val needsNewCode = result.code == ApiErrorCodes.OTP_EXPIRED ||
                        result.code == ApiErrorCodes.OTP_TOO_MANY_ATTEMPTS
                    // Code faux ou perdu : on vide le champ pour une nouvelle saisie.
                    val clear = needsNewCode || result.code == ApiErrorCodes.OTP_INVALID
                    _state.update {
                        it.copy(
                            verifying = false,
                            error = result.toErrorMessage(),
                            code = if (clear) "" else it.code,
                            needsNewCode = it.needsNewCode || needsNewCode,
                        )
                    }
                }
                // Hors ligne : le code reste saisi, le bouton « Valider » permet de réessayer.
                is ApiResult.NetworkError -> _state.update { it.copy(verifying = false, error = result.toErrorMessage()) }
            }
        }
    }

    fun resend() {
        val current = _state.value
        if (!current.canResend) return
        _state.update { it.copy(resending = true, error = null, codeResent = false) }
        viewModelScope.launch {
            val result = auth.requestOtp(current.phone)
            backoff.delayAfter(result)?.let(resendCountdown::start)
            if (result is ApiResult.Success) {
                codeSent(result.data.expiresIn)
                _state.update { it.copy(resending = false, code = "", needsNewCode = false, codeResent = true) }
            } else {
                _state.update { it.copy(resending = false, error = result.toErrorMessage()) }
            }
        }
    }

    private fun codeSent(expiresInSeconds: Int) {
        resendCountdown.start(RESEND_COOLDOWN_SECONDS)
        expiryCountdown.start(expiresInSeconds.coerceAtLeast(0).toLong())
    }

    private companion object {
        const val KEY_RESEND_AT = "resend_at"
        const val KEY_EXPIRES_AT = "expires_at"

        /** Le serveur accepte un code par numéro toutes les 60 s. */
        const val RESEND_COOLDOWN_SECONDS = 60L
    }
}
