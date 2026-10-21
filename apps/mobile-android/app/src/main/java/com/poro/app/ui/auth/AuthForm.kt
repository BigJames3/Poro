package com.poro.app.ui.auth

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.ui.quantityString

/** Mise en page commune des écrans de connexion : titre annoncé comme tel, contenu défilant. */
@Composable
fun AuthForm(
    title: String,
    modifier: Modifier = Modifier,
    subtitle: String? = null,
    content: @Composable ColumnScope.() -> Unit,
) {
    Column(
        modifier = modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Text(title, style = MaterialTheme.typography.headlineMedium, modifier = Modifier.semantics { heading() })
        subtitle?.let { Text(it, style = MaterialTheme.typography.bodyLarge) }
        content()
    }
}

/**
 * Bouton principal d'un formulaire : indicateur pendant l'envoi, puis attente imposée par le
 * serveur (« Réessayer dans 42 secondes ») tant que [retryInSeconds] est positif.
 */
@Composable
fun SubmitButton(
    text: String,
    enabled: Boolean,
    loading: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
    retryInSeconds: Int = 0,
) {
    Button(onClick = onClick, enabled = enabled, modifier = modifier.fillMaxWidth().heightIn(min = 48.dp)) {
        when {
            loading -> {
                val description = stringResource(R.string.loading)
                CircularProgressIndicator(
                    modifier = Modifier.size(20.dp).semantics { contentDescription = description },
                    strokeWidth = 2.dp,
                )
            }
            retryInSeconds > 0 -> Text(quantityString(R.plurals.retry_in_seconds, retryInSeconds))
            else -> Text(text)
        }
    }
}

/** Lien secondaire centré (changer de méthode, mot de passe oublié…), cible de 48 dp. */
@Composable
fun ColumnScope.AuthLink(text: String, onClick: () -> Unit, enabled: Boolean = true) {
    TextButton(
        onClick = onClick,
        enabled = enabled,
        modifier = Modifier.align(Alignment.CenterHorizontally).heightIn(min = 48.dp),
    ) {
        Text(text)
    }
}
