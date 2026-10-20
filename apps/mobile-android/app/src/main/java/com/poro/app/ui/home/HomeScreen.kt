package com.poro.app.ui.home

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.poro.app.ui.theme.PoroTheme

/**
 * Écran provisoire des fondations : confirme que l'application joint la gateway.
 * Il sera remplacé par le fil de vidéos ; la connexion (étape 3) s'insère avant lui.
 */
@Composable
fun HomeScreen(
    status: ApiStatus,
    signedIn: Boolean,
    onRetry: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier.fillMaxSize().padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp, Alignment.CenterVertically),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text("Poro", style = MaterialTheme.typography.displayMedium, color = MaterialTheme.colorScheme.primary)
        when (status) {
            ApiStatus.Checking -> CircularProgressIndicator()
            ApiStatus.Reachable -> Text("API joignable ✓", style = MaterialTheme.typography.titleMedium)
            is ApiStatus.Unreachable -> {
                Text(
                    "API injoignable : ${status.reason}\nLa pile locale est-elle lancée (docker compose up) ?",
                    textAlign = TextAlign.Center,
                    color = MaterialTheme.colorScheme.error,
                )
                Button(onClick = onRetry) { Text("Réessayer") }
            }
        }
        Text(
            if (signedIn) "Session active" else "Non connecté",
            style = MaterialTheme.typography.bodyMedium,
        )
    }
}

@Preview(showBackground = true)
@Composable
private fun HomeScreenPreview() {
    PoroTheme { HomeScreen(status = ApiStatus.Reachable, signedIn = false, onRetry = {}) }
}
