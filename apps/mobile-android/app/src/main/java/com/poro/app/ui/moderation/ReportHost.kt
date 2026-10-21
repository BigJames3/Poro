package com.poro.app.ui.moderation

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.createSavedStateHandle
import com.poro.app.R
import com.poro.app.ui.AppNav
import com.poro.app.ui.CollectEvents
import com.poro.app.ui.poroViewModel

@Composable
fun ReportHost(nav: AppNav, targetType: String, targetId: String, onDismiss: () -> Unit) {
    val vm = poroViewModel(key = "report-$targetType-$targetId") {
        ReportViewModel(targetType, targetId, nav.container.moderationRepository, createSavedStateHandle(), nav.container.clock)
    }
    val state by vm.state.collectAsStateWithLifecycle()
    CollectEvents(vm.events) { event ->
        when (event) {
            ReportEvent.Submitted -> nav.showMessage(R.string.report_sent)
            ReportEvent.AlreadyReported -> nav.showMessage(R.string.report_already)
        }
        onDismiss()
    }
    ReportSheet(
        state = state,
        onReason = vm::onReason,
        onCommentChange = vm::onCommentChange,
        onSubmit = vm::submit,
        onDismiss = onDismiss,
    )
}
