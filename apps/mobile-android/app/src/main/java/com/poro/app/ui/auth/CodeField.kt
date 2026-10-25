package com.poro.app.ui.auth

import androidx.compose.foundation.border
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.sizeIn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.BasicTextField
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.autofill.ContentType
import androidx.compose.ui.focus.FocusRequester
import androidx.compose.ui.focus.focusRequester
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.contentType
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.ui.theme.PoroSpacing

/**
 * Code à 6 chiffres : un seul champ invisible dessiné en 6 cases. Le collage et le remplissage
 * automatique d'un code reçu par SMS arrivent en une fois, comme une saisie au clavier.
 */
@Composable
fun CodeField(
    code: String,
    enabled: Boolean,
    onCodeChange: (String) -> Unit,
    onDone: () -> Unit,
    modifier: Modifier = Modifier,
    /** Type proposé au remplissage automatique : le code d'un SMS, ou rien pour un code reçu par email. */
    autofillType: ContentType? = ContentType.SmsOtpCode,
    length: Int = 6,
) {
    val focusRequester = remember { FocusRequester() }
    // À l'ouverture et après une vérification refusée : le clavier revient sans toucher le champ.
    LaunchedEffect(enabled) { if (enabled) focusRequester.requestFocus() }
    val description = stringResource(R.string.code_field_description)
    BasicTextField(
        value = code,
        onValueChange = onCodeChange,
        enabled = enabled,
        singleLine = true,
        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.NumberPassword, imeAction = ImeAction.Done),
        keyboardActions = KeyboardActions(onDone = { onDone() }),
        modifier = modifier
            .focusRequester(focusRequester)
            .semantics {
                autofillType?.let { contentType = it }
                contentDescription = description
            },
        decorationBox = {
            Row(horizontalArrangement = Arrangement.spacedBy(PoroSpacing.S)) {
                repeat(length) { index ->
                    val active = enabled && index == code.length
                    Box(
                        contentAlignment = Alignment.Center,
                        modifier = Modifier
                            // Taille minimale : la case grandit avec la police au lieu de couper le chiffre.
                            .sizeIn(minWidth = 44.dp, minHeight = 56.dp)
                            .border(
                                width = if (active) 2.dp else 1.dp,
                                color = if (active) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.outline,
                                shape = RoundedCornerShape(8.dp),
                            ),
                    ) {
                        Text(code.getOrNull(index)?.toString().orEmpty(), style = MaterialTheme.typography.headlineSmall)
                    }
                }
            }
        },
    )
}
