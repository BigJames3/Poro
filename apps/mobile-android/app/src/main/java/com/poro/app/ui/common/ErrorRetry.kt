package com.poro.app.ui.common

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.heightIn
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.ui.auth.ErrorMessage
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.theme.PoroSpacing

/** Message d'erreur d'une liste et son bouton « Réessayer » : une erreur n'est jamais une impasse. */
@Composable
fun ErrorRetry(error: ErrorMessage, onRetry: () -> Unit, modifier: Modifier = Modifier) {
    Column(
        modifier,
        verticalArrangement = Arrangement.spacedBy(PoroSpacing.S),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        ErrorMessageText(error)
        OutlinedButton(onClick = onRetry, modifier = Modifier.heightIn(min = 48.dp)) {
            Text(stringResource(R.string.action_retry))
        }
    }
}
