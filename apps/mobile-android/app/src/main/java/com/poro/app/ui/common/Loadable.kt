package com.poro.app.ui.common

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.core.network.ApiResult
import com.poro.app.ui.auth.ErrorMessage
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.auth.toErrorMessage

/** Contenu chargé depuis l'API : en cours, disponible, ou en échec avec un message à afficher. */
sealed interface Loadable<out T> {
    data object Loading : Loadable<Nothing>
    data class Loaded<T>(val value: T) : Loadable<T>
    data class Failed(val error: ErrorMessage) : Loadable<Nothing>

    val valueOrNull: T?
        get() = when (this) {
            is Loaded -> value
            else -> null
        }
}

/** Convertit un résultat d'API ; un échec garde le message traduit du code d'erreur. */
fun <T> ApiResult<T>.toLoadable(): Loadable<T> = when (this) {
    is ApiResult.Success -> Loadable.Loaded(data)
    else -> Loadable.Failed(toErrorMessage()!!)
}

/** Indicateur centré, ou message d'erreur et « Réessayer », sinon [content]. */
@Composable
fun <T> LoadableContent(
    state: Loadable<T>,
    onRetry: () -> Unit,
    modifier: Modifier = Modifier,
    content: @Composable (T) -> Unit,
) {
    when (state) {
        Loadable.Loading -> Box(modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            val loading = stringResource(R.string.loading)
            CircularProgressIndicator(modifier = Modifier.semantics { contentDescription = loading })
        }
        is Loadable.Failed -> Column(
            modifier = modifier.fillMaxSize().padding(24.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp, Alignment.CenterVertically),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            ErrorMessageText(state.error)
            OutlinedButton(onClick = onRetry) { Text(stringResource(R.string.action_retry)) }
        }
        is Loadable.Loaded -> content(state.value)
    }
}
