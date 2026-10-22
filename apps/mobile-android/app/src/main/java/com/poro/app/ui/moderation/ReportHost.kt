package com.poro.app.ui.moderation

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.lifecycle.ViewModel
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.createSavedStateHandle
import androidx.lifecycle.viewModelScope
import com.poro.app.R
import com.poro.app.core.network.ApiResult
import com.poro.app.data.user.ProfileRepository
import com.poro.app.ui.AppNav
import com.poro.app.ui.CollectEvents
import com.poro.app.ui.poroViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

/** Feuille de signalement de [target], avec son ViewModel (un par cible) et ses messages. */
@Composable
fun ReportHost(nav: AppNav, target: ReportTarget, onDismiss: () -> Unit) {
    val container = nav.container
    val vm = poroViewModel(key = "report-${target.type.wire}-${target.id}") {
        ReportViewModel(target, container.moderationRepository, createSavedStateHandle())
    }
    val state by vm.state.collectAsStateWithLifecycle()
    CollectEvents(vm.events) { event ->
        when (event) {
            is ReportEvent.Sent -> nav.showMessage(if (event.created) R.string.report_sent else R.string.report_already_sent)
            ReportEvent.OwnContent -> nav.showMessage(R.string.report_own_content)
        }
        onDismiss()
    }
    ReportSheet(
        type = target.type,
        state = state,
        onSelectReason = vm::selectReason,
        onDetailChange = vm::onDetailChange,
        onSend = vm::send,
        onDismiss = onDismiss,
    )
}

/**
 * Identifiant du compte connecté, pour masquer « Signaler » sur ses propres contenus. Null tant
 * qu'il n'est pas connu : le serveur refuse de toute façon (`cannot_report_self`).
 */
class MyUserIdViewModel(profiles: ProfileRepository) : ViewModel() {
    private val _userId = MutableStateFlow<String?>(null)
    val userId: StateFlow<String?> = _userId.asStateFlow()

    init {
        viewModelScope.launch { _userId.value = (profiles.me() as? ApiResult.Success)?.data?.userId }
    }
}

@Composable
fun rememberMyUserId(nav: AppNav): String? {
    val vm = poroViewModel(key = "my-user-id") { MyUserIdViewModel(nav.container.profileRepository) }
    val userId by vm.userId.collectAsStateWithLifecycle()
    return userId
}
