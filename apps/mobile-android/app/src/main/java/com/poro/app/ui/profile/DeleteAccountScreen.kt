package com.poro.app.ui.profile

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.toggleable
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Checkbox
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.autofill.ContentType
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.core.phone.PhoneNumber
import com.poro.app.ui.auth.CodeField
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.auth.PasswordField
import com.poro.app.ui.common.BackTopBar
import com.poro.app.ui.quantityString
import com.poro.app.ui.theme.PoroSpacing

@Composable
fun DeleteAccountScreen(
    state: DeleteAccountUiState,
    onBack: () -> Unit,
    onRetryLoad: () -> Unit,
    onPasswordChange: (String) -> Unit,
    onCodeChange: (String) -> Unit,
    onSendCode: () -> Unit,
    onAcknowledgedChange: (Boolean) -> Unit,
    onDelete: () -> Unit,
) {
    Scaffold(topBar = { BackTopBar(stringResource(R.string.delete_title), onBack) }) { padding ->
        Column(
            modifier = Modifier.fillMaxSize().padding(padding).verticalScroll(rememberScrollState()).padding(PoroSpacing.XL),
            verticalArrangement = Arrangement.spacedBy(PoroSpacing.L),
        ) {
            Text(stringResource(R.string.delete_explanation), style = MaterialTheme.typography.bodyLarge)
            state.loadError?.let {
                ErrorMessageText(it)
                OutlinedButton(onClick = onRetryLoad) { Text(stringResource(R.string.action_retry)) }
            }
            when (val proof = state.proof) {
                null -> if (state.loadError == null) Box(Modifier.fillMaxWidth(), contentAlignment = Alignment.Center) { CircularProgressIndicator() }
                DeletionProof.Password -> {
                    Text(stringResource(R.string.delete_password_prompt))
                    PasswordField(
                        value = state.password,
                        onValueChange = onPasswordChange,
                        label = R.string.password_label,
                        enabled = !state.deleting,
                        autofillType = ContentType.Password,
                        onDone = onDelete,
                    )
                }
                is DeletionProof.SmsCode -> {
                    val phone = PhoneNumber.fromE164(proof.phone)?.masked() ?: proof.phone
                    Text(stringResource(R.string.delete_code_prompt, phone))
                    OutlinedButton(
                        onClick = onSendCode,
                        enabled = state.canSendCode,
                        modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp),
                    ) {
                        Text(
                            when {
                                state.resendInSeconds > 0 -> quantityString(R.plurals.otp_resend_in_seconds, state.resendInSeconds)
                                state.codeSent -> stringResource(R.string.otp_resend)
                                else -> stringResource(R.string.delete_send_code)
                            },
                        )
                    }
                    if (state.codeSent) {
                        CodeField(code = state.code, enabled = !state.deleting, onCodeChange = onCodeChange, onDone = {})
                    }
                }
            }
            Row(
                verticalAlignment = Alignment.CenterVertically,
                modifier = Modifier
                    .fillMaxWidth()
                    .heightIn(min = 48.dp)
                    .toggleable(value = state.acknowledged, role = Role.Checkbox, onValueChange = onAcknowledgedChange),
            ) {
                Checkbox(checked = state.acknowledged, onCheckedChange = null)
                Text(stringResource(R.string.delete_acknowledge), modifier = Modifier.padding(start = PoroSpacing.S))
            }
            state.error?.let { ErrorMessageText(it) }
            Button(
                onClick = onDelete,
                enabled = state.canDelete,
                colors = ButtonDefaults.buttonColors(containerColor = MaterialTheme.colorScheme.error),
                modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp),
            ) {
                Text(
                    if (state.retryInSeconds > 0) {
                        quantityString(R.plurals.retry_in_seconds, state.retryInSeconds)
                    } else {
                        stringResource(R.string.action_delete_account)
                    },
                )
            }
        }
    }
}
