package com.poro.app.ui.auth.email

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.autofill.ContentType
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.tooling.preview.Preview
import com.poro.app.R
import com.poro.app.ui.auth.AuthForm
import com.poro.app.ui.auth.CodeField
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.auth.PasswordField
import com.poro.app.ui.auth.SubmitButton
import com.poro.app.ui.theme.PoroTheme

@Composable
fun ResetPasswordScreen(
    state: ResetPasswordUiState,
    onCodeChange: (String) -> Unit,
    onPasswordChange: (String) -> Unit,
    onSubmit: () -> Unit,
    onResend: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val passwordHint =
        if (state.newPassword.isNotEmpty() && !state.passwordValid) stringResource(R.string.password_rules_hint) else null
    AuthForm(
        title = stringResource(R.string.reset_title),
        subtitle = stringResource(R.string.reset_subtitle, state.email),
        modifier = modifier,
    ) {
        CodeField(
            code = state.code,
            enabled = !state.submitting && !state.needsNewCode,
            onCodeChange = onCodeChange,
            onDone = {},
            autofillType = null,
        )
        PasswordField(
            value = state.newPassword,
            onValueChange = onPasswordChange,
            label = R.string.new_password_label,
            enabled = !state.submitting,
            autofillType = ContentType.NewPassword,
            onDone = onSubmit,
            hint = passwordHint,
        )
        state.error?.let { ErrorMessageText(it) }
        if (state.codeResent) {
            Text(
                stringResource(R.string.email_code_resent),
                color = MaterialTheme.colorScheme.secondary,
                modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite },
            )
        }
        SubmitButton(
            text = stringResource(R.string.action_change_password),
            enabled = state.canSubmit,
            loading = state.submitting,
            onClick = onSubmit,
        )
        ResendButton(
            resendInSeconds = state.resendInSeconds,
            enabled = state.canResend,
            loading = state.resending,
            onClick = onResend,
        )
    }
}

@Preview(showBackground = true)
@Composable
private fun ResetPasswordScreenPreview() {
    PoroTheme {
        ResetPasswordScreen(
            state = ResetPasswordUiState(email = "awa@poro.africa", code = "1234", resendInSeconds = 30),
            onCodeChange = {},
            onPasswordChange = {},
            onSubmit = {},
            onResend = {},
        )
    }
}
