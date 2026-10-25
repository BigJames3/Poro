package com.poro.app.ui.auth.phone

import androidx.annotation.StringRes
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.autofill.ContentType
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.contentType
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.core.phone.LaunchCountry
import com.poro.app.core.legal.LegalDocumentId
import com.poro.app.ui.auth.AuthLink
import com.poro.app.ui.legal.LegalMention
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.quantityString
import com.poro.app.ui.theme.PoroSpacing
import com.poro.app.ui.theme.PoroTheme
import com.poro.app.ui.theme.brandButtonColors

@Composable
fun PhoneEntryScreen(
    state: PhoneEntryUiState,
    onCountryChange: (LaunchCountry) -> Unit,
    onInputChange: (String) -> Unit,
    onSubmit: () -> Unit,
    onUseEmail: () -> Unit,
    modifier: Modifier = Modifier,
    onOpenLegal: (LegalDocumentId) -> Unit = {},
) {
    // L'indication n'apparaît qu'une fois assez de chiffres saisis, pas pendant la frappe.
    val showHint = !state.isValid && state.input.count { it in '0'..'9' } >= state.country.nationalLength
    Column(
        modifier = modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(PoroSpacing.XL),
        verticalArrangement = Arrangement.spacedBy(PoroSpacing.L),
    ) {
        Text(
            stringResource(R.string.phone_title),
            style = MaterialTheme.typography.headlineMedium,
            modifier = Modifier.semantics { heading() },
        )
        Text(stringResource(R.string.phone_subtitle), style = MaterialTheme.typography.bodyLarge)
        Row(horizontalArrangement = Arrangement.spacedBy(PoroSpacing.S), verticalAlignment = Alignment.Top) {
            CountryPicker(
                selected = state.country,
                enabled = !state.sending,
                onSelect = onCountryChange,
                modifier = Modifier.padding(top = PoroSpacing.S),
            )
            OutlinedTextField(
                value = state.input,
                onValueChange = onInputChange,
                label = { Text(stringResource(R.string.phone_label)) },
                singleLine = true,
                enabled = !state.sending,
                isError = showHint,
                supportingText = if (showHint) {
                    { Text(stringResource(R.string.phone_invalid_hint)) }
                } else {
                    null
                },
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Phone, imeAction = ImeAction.Done),
                keyboardActions = KeyboardActions(onDone = { onSubmit() }),
                modifier = Modifier.weight(1f).semantics { contentType = ContentType.PhoneNumberNational },
            )
        }
        state.error?.let { ErrorMessageText(it) }
        Button(
            onClick = onSubmit,
            enabled = state.canSubmit,
            colors = brandButtonColors(),
            modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp),
        ) {
            when {
                state.sending -> {
                    val loading = stringResource(R.string.loading)
                    CircularProgressIndicator(
                        modifier = Modifier.size(20.dp).semantics { contentDescription = loading },
                        strokeWidth = 2.dp,
                    )
                }
                state.retryInSeconds > 0 -> Text(quantityString(R.plurals.retry_in_seconds, state.retryInSeconds))
                else -> Text(stringResource(R.string.action_send_code))
            }
        }
        // Autre méthode, aussi le recours quand l'envoi de SMS est indisponible (sms_unavailable).
        AuthLink(stringResource(R.string.phone_use_email), onUseEmail, enabled = !state.sending)
        LegalMention(onOpenLegal)
    }
}

@Composable
private fun CountryPicker(
    selected: LaunchCountry,
    enabled: Boolean,
    onSelect: (LaunchCountry) -> Unit,
    modifier: Modifier = Modifier,
) {
    var expanded by rememberSaveable { mutableStateOf(false) }
    val description = stringResource(
        R.string.phone_country_picker_description,
        stringResource(selected.label()),
        selected.dialCode,
    )
    Box(modifier) {
        OutlinedButton(
            onClick = { expanded = true },
            enabled = enabled,
            modifier = Modifier.heightIn(min = 56.dp).semantics { contentDescription = description },
        ) {
            Text("${selected.flag} +${selected.dialCode} ▾")
        }
        DropdownMenu(expanded = expanded, onDismissRequest = { expanded = false }) {
            LaunchCountry.entries.forEach { country ->
                DropdownMenuItem(
                    text = { Text("${country.flag}  ${stringResource(country.label())} (+${country.dialCode})") },
                    onClick = {
                        expanded = false
                        onSelect(country)
                    },
                )
            }
        }
    }
}

@StringRes
internal fun LaunchCountry.label(): Int = when (this) {
    LaunchCountry.CI -> R.string.country_ci
    LaunchCountry.SN -> R.string.country_sn
    LaunchCountry.CM -> R.string.country_cm
    LaunchCountry.NG -> R.string.country_ng
}

@Preview(showBackground = true)
@Composable
private fun PhoneEntryScreenPreview() {
    PoroTheme {
        PhoneEntryScreen(
            state = PhoneEntryUiState(input = "07 00 00 00 00", isValid = true),
            onCountryChange = {},
            onInputChange = {},
            onSubmit = {},
            onUseEmail = {},
        )
    }
}
