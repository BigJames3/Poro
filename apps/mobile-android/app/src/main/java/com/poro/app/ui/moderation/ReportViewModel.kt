package com.poro.app.ui.moderation

import androidx.annotation.StringRes
import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.R
import com.poro.app.core.network.ApiResult
import com.poro.app.core.time.Clock
import com.poro.app.data.moderation.ModerationRepository
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

/** Les 7 motifs du contrat ; les valeurs envoyées restent l'enum anglais. */
enum class ReportReason(val wire: String, @StringRes val label: Int) {
    Spam("spam", R.string.report_reason_spam),
    Nudity("nudity", R.string.report_reason_nudity),
    Violence("violence", R.string.report_reason_violence),
    Harassment("harassment", R.string.report_reason_harassment),
    Hate("hate", R.string.report_reason_hate),
    Fraud("fraud", R.string.report_reason_fraud),
    Other("other", R.string.report_reason_other),
}

data class ReportUiState(
    val reason: ReportReason? = null,
    val comment: String = "",
    val submitting: Boolean = false,
    val retryInSeconds: Int = 0,
    val error: ErrorMessage? = null,
) {
    val canSubmit: Boolean get() = reason != null && !submitting && retryInSeconds == 0
}

sealed interface ReportEvent {
    data object Submitted : ReportEvent
    data object AlreadyReported : ReportEvent
}

class ReportViewModel(
    private val targetType: String,
    private val targetId: String,
    private val reports: ModerationRepository,
    private val savedState: SavedStateHandle,
    clock: Clock,
) : ViewModel() {

    private val _state = MutableStateFlow(
        ReportUiState(
            reason = savedState.get<String>(KEY_REASON)?.let { wire -> ReportReason.entries.find { it.wire == wire } },
            comment = savedState.get<String>(KEY_COMMENT).orEmpty(),
        ),
    )
    val state: StateFlow<ReportUiState> = _state.asStateFlow()

    private val _events = Channel<ReportEvent>(Channel.BUFFERED)
    val events: Flow<ReportEvent> = _events.receiveAsFlow()

    private val backoff = RateLimitBackoff()
    private val retryCountdown = Countdown(viewModelScope, savedState, KEY_RETRY_AT, clock) { seconds ->
        _state.update { it.copy(retryInSeconds = seconds) }
    }

    init {
        retryCountdown.restore()
    }

    fun onReason(reason: ReportReason) {
        savedState[KEY_REASON] = reason.wire
        _state.update { it.copy(reason = reason, error = null) }
    }

    fun onCommentChange(value: String) {
        val comment = value.take(COMMENT_MAX)
        savedState[KEY_COMMENT] = comment
        _state.update { it.copy(comment = comment, error = null) }
    }

    fun submit() {
        val current = _state.value
        val reason = current.reason ?: return
        if (!current.canSubmit) return
        _state.update { it.copy(submitting = true, error = null) }
        viewModelScope.launch {
            val result = reports.report(targetType, targetId, reason.wire, current.comment)
            backoff.delayAfter(result)?.let(retryCountdown::start)
            when (result) {
                is ApiResult.Success -> {
                    _state.update { it.copy(submitting = false) }
                    _events.send(if (result.data.created) ReportEvent.Submitted else ReportEvent.AlreadyReported)
                }
                else -> _state.update { it.copy(submitting = false, error = result.toErrorMessage()) }
            }
        }
    }

    companion object {
        const val COMMENT_MAX = 500
        private const val KEY_REASON = "reason"
        private const val KEY_COMMENT = "comment"
        private const val KEY_RETRY_AT = "retry_at"
    }
}
