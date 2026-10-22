package com.poro.app.ui.auth.otp

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.core.phone.PhoneNumber
import com.poro.app.ui.auth.CodeField
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.quantityString
import com.poro.app.ui.theme.PoroTheme
import com.poro.app.ui.theme.brandButtonColors

@Composable
fun OtpVerifyScreen(
    state: OtpUiState,
    onCodeChange: (String) -> Unit,
    onVerify: () -> Unit,
    onResend: () -> Unit,
    onChangeNumber: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val phone = remember(state.phone) { PhoneNumber.fromE164(state.phone)?.masked() ?: state.phone }
    Column(
        modifier = modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Text(
            stringResource(R.string.otp_title),
            style = MaterialTheme.typography.headlineMedium,
            modifier = Modifier.semantics { heading() },
        )
        Text(stringResource(R.string.otp_subtitle, phone), style = MaterialTheme.typography.bodyLarge)
        CodeField(
            code = state.code,
            enabled = !state.verifying && !state.needsNewCode,
            onCodeChange = onCodeChange,
            onDone = onVerify,
        )
        when {
            state.verifying -> LinearProgressIndicator(modifier = Modifier.fillMaxWidth())
            state.needsNewCode -> Unit // le message d'erreur dit déjà de demander un nouveau code
            state.expiresInSeconds == 0 -> Text(
                stringResource(R.string.otp_expired_hint),
                color = MaterialTheme.colorScheme.error,
                style = MaterialTheme.typography.bodyMedium,
            )
            else -> Text(expiryText(state.expiresInSeconds), style = MaterialTheme.typography.bodyMedium)
        }
        state.error?.let { ErrorMessageText(it) }
        if (state.codeResent) {
            Text(
                stringResource(R.string.otp_code_resent),
                color = MaterialTheme.colorScheme.secondary,
                modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite },
            )
        }
        Button(
            onClick = onVerify,
            enabled = state.canVerify,
            colors = brandButtonColors(),
            modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp),
        ) {
            Text(stringResource(R.string.action_verify))
        }
        OutlinedButton(
            onClick = onResend,
            enabled = state.canResend,
            modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp),
        ) {
            when {
                state.resending -> {
                    val loading = stringResource(R.string.loading)
                    CircularProgressIndicator(
                        modifier = Modifier.size(20.dp).semantics { contentDescription = loading },
                        strokeWidth = 2.dp,
                    )
                }
                state.resendInSeconds > 0 -> Text(quantityString(R.plurals.otp_resend_in_seconds, state.resendInSeconds))
                else -> Text(stringResource(R.string.otp_resend))
            }
        }
        TextButton(
            onClick = onChangeNumber,
            enabled = !state.verifying,
            modifier = Modifier.align(Alignment.CenterHorizontally).heightIn(min = 48.dp),
        ) {
            Text(stringResource(R.string.otp_change_number))
        }
    }
}

/** « Ce code expire dans 5 minutes », puis en secondes pendant la dernière minute. */
@Composable
private fun expiryText(seconds: Int): String =
    if (seconds >= SECONDS_PER_MINUTE) {
        quantityString(R.plurals.otp_expires_in_minutes, (seconds + SECONDS_PER_MINUTE - 1) / SECONDS_PER_MINUTE)
    } else {
        quantityString(R.plurals.otp_expires_in_seconds, seconds)
    }

private const val SECONDS_PER_MINUTE = 60

@Preview(showBackground = true)
@Composable
private fun OtpVerifyScreenPreview() {
    PoroTheme {
        OtpVerifyScreen(
            state = OtpUiState(phone = "+2250700000000", code = "123", resendInSeconds = 42, expiresInSeconds = 280),
            onCodeChange = {},
            onVerify = {},
            onResend = {},
            onChangeNumber = {},
        )
    }
}
