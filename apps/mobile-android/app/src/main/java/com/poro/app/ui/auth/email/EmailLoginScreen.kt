package com.poro.app.ui.auth.email

import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.autofill.ContentType
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview
import com.poro.app.R
import com.poro.app.core.account.AccountRules
import com.poro.app.ui.auth.AuthForm
import com.poro.app.ui.auth.AuthLink
import com.poro.app.ui.auth.EmailField
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.auth.PasswordField
import com.poro.app.ui.auth.SubmitButton
import com.poro.app.ui.theme.PoroTheme

@Composable
fun EmailLoginScreen(
    state: EmailLoginUiState,
    onEmailChange: (String) -> Unit,
    onPasswordChange: (String) -> Unit,
    onSubmit: () -> Unit,
    onForgotPassword: () -> Unit,
    onCreateAccount: () -> Unit,
    onUsePhone: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val editable = !state.signingIn
    AuthForm(title = stringResource(R.string.email_login_title), modifier = modifier) {
        EmailField(
            value = state.email,
            onValueChange = onEmailChange,
            enabled = editable,
            hint = emailHint(state.email),
        )
        PasswordField(
            value = state.password,
            onValueChange = onPasswordChange,
            label = R.string.password_label,
            enabled = editable,
            autofillType = ContentType.Password,
            onDone = onSubmit,
        )
        state.error?.let { ErrorMessageText(it) }
        SubmitButton(
            text = stringResource(R.string.action_sign_in),
            enabled = state.canSubmit,
            loading = state.signingIn,
            retryInSeconds = state.retryInSeconds,
            onClick = onSubmit,
        )
        AuthLink(stringResource(R.string.email_forgot_password), onForgotPassword, enabled = editable)
        AuthLink(stringResource(R.string.email_create_account), onCreateAccount, enabled = editable)
        AuthLink(stringResource(R.string.email_use_phone), onUsePhone, enabled = editable)
    }
}

/** Indication affichée seulement quand l'adresse semble complète (un point après l'arobase). */
@Composable
internal fun emailHint(email: String): String? {
    val looksComplete = email.substringAfter('@', "").contains('.')
    return if (looksComplete && !AccountRules.isValidEmail(email)) stringResource(R.string.email_invalid_hint) else null
}

@Preview(showBackground = true)
@Composable
private fun EmailLoginScreenPreview() {
    PoroTheme {
        EmailLoginScreen(
            state = EmailLoginUiState(email = "awa@poro.africa"),
            onEmailChange = {},
            onPasswordChange = {},
            onSubmit = {},
            onForgotPassword = {},
            onCreateAccount = {},
            onUsePhone = {},
        )
    }
}
