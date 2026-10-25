package com.poro.app.ui.moderation

import androidx.annotation.StringRes
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.selection.selectableGroup
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.data.moderation.ReportReason
import com.poro.app.data.moderation.ReportTargetType
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.auth.SubmitButton
import com.poro.app.ui.quantityString
import com.poro.app.ui.theme.PoroSpacing

@StringRes
fun ReportReason.label(): Int = when (this) {
    ReportReason.Spam -> R.string.report_reason_spam
    ReportReason.Nudity -> R.string.report_reason_nudity
    ReportReason.Violence -> R.string.report_reason_violence
    ReportReason.Harassment -> R.string.report_reason_harassment
    ReportReason.Hate -> R.string.report_reason_hate
    ReportReason.Fraud -> R.string.report_reason_fraud
    ReportReason.Other -> R.string.report_reason_other
}

@StringRes
private fun ReportTargetType.title(): Int = when (this) {
    ReportTargetType.Video -> R.string.report_title_video
    ReportTargetType.Comment -> R.string.report_title_comment
    ReportTargetType.User -> R.string.report_title_user
}

/** Choix du motif (obligatoire) et détail facultatif, puis envoi. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ReportSheet(
    type: ReportTargetType,
    state: ReportUiState,
    onSelectReason: (ReportReason) -> Unit,
    onDetailChange: (String) -> Unit,
    onSend: () -> Unit,
    onDismiss: () -> Unit,
) {
    ModalBottomSheet(onDismissRequest = onDismiss) {
        Column(
            Modifier.verticalScroll(rememberScrollState()).padding(horizontal = PoroSpacing.L).padding(bottom = PoroSpacing.XL),
            verticalArrangement = Arrangement.spacedBy(PoroSpacing.S),
        ) {
            Text(stringResource(type.title()), style = MaterialTheme.typography.titleMedium, modifier = Modifier.semantics { heading() })
            Text(stringResource(R.string.report_intro), style = MaterialTheme.typography.bodyMedium)
            Column(Modifier.selectableGroup()) {
                ReportReason.entries.forEach { reason ->
                    Row(
                        Modifier.fillMaxWidth().heightIn(min = 48.dp)
                            .selectable(selected = state.reason == reason, role = Role.RadioButton, enabled = !state.sending) { onSelectReason(reason) },
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        // L'appui est géré par la ligne entière, pas seulement par le bouton.
                        RadioButton(selected = state.reason == reason, onClick = null)
                        Text(stringResource(reason.label()), modifier = Modifier.padding(start = PoroSpacing.M))
                    }
                }
            }
            OutlinedTextField(
                value = state.detail,
                onValueChange = onDetailChange,
                label = { Text(stringResource(R.string.report_detail_label)) },
                supportingText = { Text(stringResource(R.string.report_detail_count, state.detail.length, ReportUiState.DETAIL_MAX)) },
                enabled = !state.sending,
                minLines = 2,
                modifier = Modifier.fillMaxWidth(),
            )
            state.retryInMinutes?.let { minutes ->
                Text(
                    quantityString(R.plurals.report_retry_in_minutes, minutes.toInt()),
                    color = MaterialTheme.colorScheme.error,
                    modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite },
                )
            }
            state.error?.let { ErrorMessageText(it) }
            SubmitButton(stringResource(R.string.report_send), enabled = state.canSend, loading = state.sending, onClick = onSend)
        }
    }
}
