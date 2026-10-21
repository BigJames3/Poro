package com.poro.app.ui.auth.email

import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.tooling.preview.Preview
import com.poro.app.R
import com.poro.app.ui.auth.AuthForm
import com.poro.app.ui.auth.EmailField
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.auth.SubmitButton
import com.poro.app.ui.theme.PoroTheme

@Composable
fun ForgotPasswordScreen(
    state: ForgotPasswordUiState,
    onEmailChange: (String) -> Unit,
    onSubmit: () -> Unit,
    modifier: Modifier = Modifier,
) {
    AuthForm(
        title = stringResource(R.string.forgot_title),
        subtitle = stringResource(R.string.forgot_subtitle),
        modifier = modifier,
    ) {
        EmailField(
            value = state.email,
            onValueChange = onEmailChange,
            enabled = !state.sending,
            hint = emailHint(state.email),
            imeAction = ImeAction.Done,
            onDone = onSubmit,
        )
        state.error?.let { ErrorMessageText(it) }
        SubmitButton(
            text = stringResource(R.string.action_send_code),
            enabled = state.canSubmit,
            loading = state.sending,
            retryInSeconds = state.retryInSeconds,
            onClick = onSubmit,
        )
    }
}

@Preview(showBackground = true)
@Composable
private fun ForgotPasswordScreenPreview() {
    PoroTheme { ForgotPasswordScreen(state = ForgotPasswordUiState(email = "awa@poro.africa"), onEmailChange = {}, onSubmit = {}) }
}
