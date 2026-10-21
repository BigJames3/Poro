package com.poro.app.ui.auth.email

import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.ui.auth.AuthForm
import com.poro.app.ui.auth.AuthLink
import com.poro.app.ui.auth.CodeField
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.auth.SubmitButton
import com.poro.app.ui.quantityString
import com.poro.app.ui.theme.PoroTheme

@Composable
fun EmailVerifyScreen(
    state: EmailVerifyUiState,
    onCodeChange: (String) -> Unit,
    onVerify: () -> Unit,
    onResend: () -> Unit,
    onLater: () -> Unit,
    modifier: Modifier = Modifier,
) {
    AuthForm(
        title = stringResource(R.string.email_verify_title),
        subtitle = stringResource(R.string.email_verify_subtitle),
        modifier = modifier,
    ) {
        CodeField(
            code = state.code,
            enabled = !state.verifying && !state.needsNewCode,
            onCodeChange = onCodeChange,
            onDone = onVerify,
            autofillType = null,
        )
        if (state.verifying) LinearProgressIndicator(modifier = Modifier.fillMaxWidth())
        state.error?.let { ErrorMessageText(it) }
        if (state.codeResent) {
            Text(
                stringResource(R.string.email_code_resent),
                color = MaterialTheme.colorScheme.secondary,
                modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite },
            )
        }
        SubmitButton(
            text = stringResource(R.string.action_verify),
            enabled = state.canVerify,
            loading = false,
            onClick = onVerify,
        )
        ResendButton(
            resendInSeconds = state.resendInSeconds,
            enabled = state.canResend,
            loading = state.resending,
            onClick = onResend,
        )
        AuthLink(stringResource(R.string.email_verify_later), onLater, enabled = !state.verifying)
    }
}

/** « Renvoyer le code », bloqué pendant l'attente avec le compte à rebours en toutes lettres. */
@Composable
internal fun ResendButton(resendInSeconds: Int, enabled: Boolean, loading: Boolean, onClick: () -> Unit) {
    OutlinedButton(onClick = onClick, enabled = enabled, modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp)) {
        Text(
            when {
                loading -> stringResource(R.string.loading)
                resendInSeconds > 0 -> quantityString(R.plurals.otp_resend_in_seconds, resendInSeconds)
                else -> stringResource(R.string.otp_resend)
            },
        )
    }
}

@Preview(showBackground = true)
@Composable
private fun EmailVerifyScreenPreview() {
    PoroTheme {
        EmailVerifyScreen(
            state = EmailVerifyUiState(code = "12", resendInSeconds = 42),
            onCodeChange = {},
            onVerify = {},
            onResend = {},
            onLater = {},
        )
    }
}
