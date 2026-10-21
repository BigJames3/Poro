package com.poro.app.ui.auth.email

import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.autofill.ContentType
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentType
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.tooling.preview.Preview
import com.poro.app.R
import com.poro.app.ui.auth.AuthForm
import com.poro.app.ui.auth.AuthLink
import com.poro.app.ui.auth.EmailField
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.auth.PasswordField
import com.poro.app.ui.auth.SubmitButton
import com.poro.app.ui.theme.PoroTheme

@Composable
fun EmailRegisterScreen(
    state: EmailRegisterUiState,
    onFullNameChange: (String) -> Unit,
    onEmailChange: (String) -> Unit,
    onPasswordChange: (String) -> Unit,
    onSubmit: () -> Unit,
    onHaveAccount: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val editable = !state.registering
    // Les indications n'apparaissent qu'une fois le champ commencé, pas sur un formulaire vide.
    val nameHint = if (state.fullName.isNotEmpty() && !state.fullNameValid) stringResource(R.string.full_name_hint) else null
    val passwordHint = if (state.password.isNotEmpty() && !state.passwordValid) stringResource(R.string.password_rules_hint) else null
    AuthForm(title = stringResource(R.string.email_register_title), modifier = modifier) {
        OutlinedTextField(
            value = state.fullName,
            onValueChange = onFullNameChange,
            label = { Text(stringResource(R.string.full_name_label)) },
            singleLine = true,
            enabled = editable,
            isError = nameHint != null,
            supportingText = nameHint?.let { { Text(it) } },
            keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Words, imeAction = ImeAction.Next),
            modifier = Modifier.fillMaxWidth().semantics { contentType = ContentType.PersonFullName },
        )
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
            autofillType = ContentType.NewPassword,
            onDone = onSubmit,
            hint = passwordHint,
        )
        state.error?.let { ErrorMessageText(it) }
        SubmitButton(
            text = stringResource(R.string.action_register),
            enabled = state.canSubmit,
            loading = state.registering,
            retryInSeconds = state.retryInSeconds,
            onClick = onSubmit,
        )
        AuthLink(stringResource(R.string.email_have_account), onHaveAccount, enabled = editable)
    }
}

@Preview(showBackground = true)
@Composable
private fun EmailRegisterScreenPreview() {
    PoroTheme {
        EmailRegisterScreen(
            state = EmailRegisterUiState(fullName = "Awa Koné", email = "awa@poro.africa", password = "1234"),
            onFullNameChange = {},
            onEmailChange = {},
            onPasswordChange = {},
            onSubmit = {},
            onHaveAccount = {},
        )
    }
}
