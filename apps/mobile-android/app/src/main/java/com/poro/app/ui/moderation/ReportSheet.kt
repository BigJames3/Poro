package com.poro.app.ui.moderation

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.selection.selectableGroup
import androidx.compose.foundation.rememberScrollState
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
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.auth.SubmitButton

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ReportSheet(
    state: ReportUiState,
    onReason: (ReportReason) -> Unit,
    onCommentChange: (String) -> Unit,
    onSubmit: () -> Unit,
    onDismiss: () -> Unit,
) {
    val editable = !state.submitting
    ModalBottomSheet(onDismissRequest = onDismiss) {
        Column(
            Modifier
                .fillMaxWidth()
                .imePadding()
                .verticalScroll(rememberScrollState())
                .padding(bottom = 24.dp),
        ) {
            Text(
                stringResource(R.string.report_title),
                style = MaterialTheme.typography.titleMedium,
                modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp).semantics { heading() },
            )
            Column(Modifier.selectableGroup()) {
                ReportReason.entries.forEach { reason ->
                    val selected = state.reason == reason
                    Row(
                        Modifier
                            .fillMaxWidth()
                            .heightIn(min = 48.dp)
                            .selectable(
                                selected = selected,
                                enabled = editable,
                                role = Role.RadioButton,
                                onClick = { onReason(reason) },
                            )
                            .padding(horizontal = 16.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        RadioButton(selected = selected, onClick = null, enabled = editable)
                        Text(stringResource(reason.label), modifier = Modifier.padding(start = 8.dp))
                    }
                }
            }
            OutlinedTextField(
                value = state.comment,
                onValueChange = onCommentChange,
                enabled = editable,
                label = { Text(stringResource(R.string.report_comment_label)) },
                supportingText = { Text(stringResource(R.string.report_comment_counter, state.comment.length, ReportViewModel.COMMENT_MAX)) },
                modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp),
            )
            state.error?.let { ErrorMessageText(it, Modifier.padding(horizontal = 16.dp)) }
            SubmitButton(
                text = stringResource(R.string.report_submit),
                enabled = state.canSubmit,
                loading = state.submitting,
                retryInSeconds = state.retryInSeconds,
                onClick = onSubmit,
                modifier = Modifier.padding(horizontal = 16.dp),
            )
        }
    }
}
