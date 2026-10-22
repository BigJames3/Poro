package com.poro.app.ui.moderation

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.data.moderation.ModerationRepository
import com.poro.app.data.moderation.ReportReason
import com.poro.app.data.moderation.ReportTargetType
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

/** Ce qui est signalé : une vidéo, un commentaire ou un compte. */
data class ReportTarget(val type: ReportTargetType, val id: String)

data class ReportUiState(
    val reason: ReportReason? = null,
    val detail: String = "",
    val sending: Boolean = false,
    val error: ErrorMessage? = null,
    /** Limite de 10 signalements par heure atteinte : minutes avant de réessayer. */
    val retryInMinutes: Long? = null,
) {
    val canSend: Boolean get() = reason != null && !sending && detail.length <= DETAIL_MAX

    companion object {
        const val DETAIL_MAX = 500
    }
}

sealed interface ReportEvent {
    /** [created] false : ce compte avait déjà signalé la cible (réponse 200). */
    data class Sent(val created: Boolean) : ReportEvent

    /** Son propre contenu (`cannot_report_self`) : rien à envoyer. */
    data object OwnContent : ReportEvent
}

/**
 * Signalement d'une cible (docs/api/moderation.openapi.yaml). Le motif et le détail survivent à la
 * mort du processus ; le serveur est idempotent, un double envoi ne crée pas deux signalements.
 */
class ReportViewModel(
    private val target: ReportTarget,
    private val moderation: ModerationRepository,
    private val savedState: SavedStateHandle,
) : ViewModel() {

    private val _state = MutableStateFlow(
        ReportUiState(
            reason = savedState.get<String>(KEY_REASON)?.let { name -> ReportReason.entries.firstOrNull { it.name == name } },
            detail = savedState[KEY_DETAIL] ?: "",
        ),
    )
    val state: StateFlow<ReportUiState> = _state.asStateFlow()

    private val _events = Channel<ReportEvent>(Channel.BUFFERED)
    val events: Flow<ReportEvent> = _events.receiveAsFlow()

    fun selectReason(reason: ReportReason) {
        savedState[KEY_REASON] = reason.name
        _state.update { it.copy(reason = reason, error = null) }
    }

    fun onDetailChange(value: String) {
        val detail = value.take(ReportUiState.DETAIL_MAX)
        savedState[KEY_DETAIL] = detail
        _state.update { it.copy(detail = detail, error = null) }
    }

    fun send() {
        val current = _state.value
        val reason = current.reason ?: return
        if (!current.canSend) return
        _state.update { it.copy(sending = true, error = null, retryInMinutes = null) }
        viewModelScope.launch {
            when (val result = moderation.report(target.type, target.id, reason, current.detail)) {
                is ApiResult.Success -> {
                    _state.update { it.copy(sending = false) }
                    _events.send(ReportEvent.Sent(result.data.created))
                }
                is ApiResult.Failure -> when {
                    result.code == ApiErrorCodes.CANNOT_REPORT_SELF -> {
                        _state.update { it.copy(sending = false) }
                        _events.send(ReportEvent.OwnContent)
                    }
                    result.httpStatus == 429 -> _state.update {
                        val minutes = result.retryAfterSeconds?.let { s -> ((s + 59) / 60).coerceAtLeast(1) }
                        it.copy(sending = false, retryInMinutes = minutes, error = if (minutes == null) result.toErrorMessage() else null)
                    }
                    else -> _state.update { it.copy(sending = false, error = result.toErrorMessage()) }
                }
                is ApiResult.NetworkError -> _state.update { it.copy(sending = false, error = result.toErrorMessage()) }
            }
        }
    }

    private companion object {
        const val KEY_REASON = "report_reason"
        const val KEY_DETAIL = "report_detail"
    }
}
