package com.poro.app.ui.profile

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.core.account.AccountRules
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.core.time.Clock
import com.poro.app.data.auth.AccountRepository
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

/** Preuve demandée par le serveur : le mot de passe d'un compte email, un code SMS frais sinon. */
sealed interface DeletionProof {
    data object Password : DeletionProof
    data class SmsCode(val phone: String) : DeletionProof
}

data class DeleteAccountUiState(
    val proof: DeletionProof? = null,
    val loadError: ErrorMessage? = null,
    val password: String = "",
    val code: String = "",
    val codeSent: Boolean = false,
    val sendingCode: Boolean = false,
    val resendInSeconds: Int = 0,
    /** Case « Je comprends » : la suppression ne part jamais d'un seul appui. */
    val acknowledged: Boolean = false,
    val deleting: Boolean = false,
    val retryInSeconds: Int = 0,
    val error: ErrorMessage? = null,
) {
    val canDelete: Boolean
        get() = acknowledged && !deleting && retryInSeconds == 0 && when (proof) {
            DeletionProof.Password -> password.isNotEmpty()
            is DeletionProof.SmsCode -> code.length == CODE_LENGTH
            null -> false
        }
    val canSendCode: Boolean get() = proof is DeletionProof.SmsCode && !sendingCode && resendInSeconds == 0

    companion object {
        const val CODE_LENGTH = 6
    }
}

sealed interface DeleteAccountEvent {
    /** Suppression programmée ; la session locale est déjà effacée (tous les appareils déconnectés). */
    data class Scheduled(val deletionScheduledAt: String?) : DeleteAccountEvent
}

class DeleteAccountViewModel(
    private val accounts: AccountRepository,
    private val auth: AuthRepository,
    savedState: SavedStateHandle,
    clock: Clock,
) : ViewModel() {

    private val _state = MutableStateFlow(DeleteAccountUiState())
    val state: StateFlow<DeleteAccountUiState> = _state.asStateFlow()

    private val _events = Channel<DeleteAccountEvent>(Channel.BUFFERED)
    val events: Flow<DeleteAccountEvent> = _events.receiveAsFlow()

    private val codeBackoff = RateLimitBackoff()
    private val deleteBackoff = RateLimitBackoff()
    private val resendCountdown = Countdown(viewModelScope, savedState, KEY_RESEND_AT, clock) { seconds ->
        _state.update { it.copy(resendInSeconds = seconds) }
    }
    private val retryCountdown = Countdown(viewModelScope, savedState, KEY_RETRY_AT, clock) { seconds ->
        _state.update { it.copy(retryInSeconds = seconds) }
    }

    init {
        resendCountdown.restore()
        retryCountdown.restore()
        if (resendCountdown.isSet) _state.update { it.copy(codeSent = true) }
        loadAccount()
    }

    fun loadAccount() {
        _state.update { it.copy(loadError = null) }
        viewModelScope.launch {
            when (val result = accounts.account()) {
                is ApiResult.Success -> {
                    val phone = result.data.phone
                    val proof = if (phone != null && result.data.email == null) DeletionProof.SmsCode(phone) else DeletionProof.Password
                    _state.update { it.copy(proof = proof) }
                }
                else -> _state.update { it.copy(loadError = result.toErrorMessage()) }
            }
        }
    }

    fun onPasswordChange(value: String) = _state.update { it.copy(password = value.take(AccountRules.PASSWORD_MAX), error = null) }

    fun onCodeChange(raw: String) =
        _state.update { it.copy(code = raw.filter { c -> c in '0'..'9' }.take(DeleteAccountUiState.CODE_LENGTH), error = null) }

    fun onAcknowledgedChange(value: Boolean) = _state.update { it.copy(acknowledged = value) }

    /** Code SMS de confirmation (même route que la connexion, mêmes limites d'envoi). */
    fun sendCode() {
        val current = _state.value
        val proof = current.proof as? DeletionProof.SmsCode ?: return
        if (!current.canSendCode) return
        _state.update { it.copy(sendingCode = true, error = null) }
        viewModelScope.launch {
            val result = auth.requestOtp(proof.phone)
            codeBackoff.delayAfter(result)?.let(resendCountdown::start)
            if (result is ApiResult.Success) resendCountdown.start(RESEND_COOLDOWN_SECONDS)
            _state.update {
                it.copy(sendingCode = false, codeSent = it.codeSent || result is ApiResult.Success, error = result.toErrorMessage())
            }
        }
    }

    fun delete() {
        val current = _state.value
        if (!current.canDelete) return
        _state.update { it.copy(deleting = true, error = null) }
        viewModelScope.launch {
            val result = when (current.proof) {
                is DeletionProof.SmsCode -> accounts.deleteAccount(password = null, otpCode = current.code)
                else -> accounts.deleteAccount(password = current.password, otpCode = null)
            }
            if (result is ApiResult.Success) {
                _state.update { it.copy(deleting = false) }
                _events.send(DeleteAccountEvent.Scheduled(result.data.deletionScheduledAt))
                return@launch
            }
            deleteBackoff.delayAfter(result)?.let(retryCountdown::start)
            val code = (result as? ApiResult.Failure)?.code
            _state.update {
                it.copy(
                    deleting = false,
                    error = result.toErrorMessage(),
                    password = if (code == ApiErrorCodes.INVALID_CREDENTIALS) "" else it.password,
                    code = if (code in CODE_REJECTED) "" else it.code,
                )
            }
        }
    }

    private companion object {
        const val KEY_RESEND_AT = "resend_at"
        const val KEY_RETRY_AT = "retry_at"
        const val RESEND_COOLDOWN_SECONDS = 60L
        val CODE_REJECTED = setOf(ApiErrorCodes.OTP_INVALID, ApiErrorCodes.OTP_EXPIRED, ApiErrorCodes.OTP_TOO_MANY_ATTEMPTS)
    }
}
