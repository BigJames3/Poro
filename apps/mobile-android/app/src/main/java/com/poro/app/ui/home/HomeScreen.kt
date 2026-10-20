package com.poro.app.ui.home

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.ui.theme.PoroTheme

/**
 * Écran provisoire des fondations : confirme que l'application joint la gateway.
 * Il sera remplacé par le fil de vidéos.
 */
@Composable
fun HomeScreen(
    status: ApiStatus,
    signingOut: Boolean,
    onRetry: () -> Unit,
    onLogout: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier.fillMaxSize().padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp, Alignment.CenterVertically),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(stringResource(R.string.app_name), style = MaterialTheme.typography.displayMedium, color = MaterialTheme.colorScheme.primary)
        when (status) {
            ApiStatus.Checking -> CircularProgressIndicator()
            ApiStatus.Reachable -> Text(stringResource(R.string.home_api_reachable), style = MaterialTheme.typography.titleMedium)
            is ApiStatus.Unreachable -> {
                Text(
                    stringResource(R.string.home_api_unreachable, status.reason),
                    textAlign = TextAlign.Center,
                    color = MaterialTheme.colorScheme.error,
                )
                Button(onClick = onRetry, modifier = Modifier.heightIn(min = 48.dp)) { Text(stringResource(R.string.action_retry)) }
            }
        }
        Text(stringResource(R.string.home_signed_in), style = MaterialTheme.typography.bodyMedium)
        OutlinedButton(onClick = onLogout, enabled = !signingOut, modifier = Modifier.heightIn(min = 48.dp)) {
            Text(stringResource(R.string.action_logout))
        }
    }
}

@Preview(showBackground = true)
@Composable
private fun HomeScreenPreview() {
    PoroTheme { HomeScreen(status = ApiStatus.Reachable, signingOut = false, onRetry = {}, onLogout = {}) }
}
