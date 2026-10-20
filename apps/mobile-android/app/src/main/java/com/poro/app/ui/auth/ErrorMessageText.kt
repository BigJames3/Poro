package com.poro.app.ui.auth

import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import com.poro.app.R

/** Texte d'erreur annoncé par TalkBack dès qu'il apparaît. */
@Composable
fun ErrorMessageText(error: ErrorMessage, modifier: Modifier = Modifier) {
    val text = stringResource(error.text)
    val full = error.requestId?.let { text + "\n" + stringResource(R.string.error_reference, it) } ?: text
    Text(
        text = full,
        color = MaterialTheme.colorScheme.error,
        style = MaterialTheme.typography.bodyMedium,
        modifier = modifier.semantics { liveRegion = LiveRegionMode.Polite },
    )
}
