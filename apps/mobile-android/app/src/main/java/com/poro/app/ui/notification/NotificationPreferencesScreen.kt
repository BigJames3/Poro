package com.poro.app.ui.notification

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.toggleable
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import com.poro.app.R
import com.poro.app.data.notification.PreferencesDto
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.common.BackTopBar
import com.poro.app.ui.common.LoadableContent
import com.poro.app.ui.theme.PoroSpacing

@Composable
fun NotificationPreferencesScreen(
    state: PreferencesUiState,
    onBack: () -> Unit,
    onRetry: () -> Unit,
    onChange: ((PreferencesDto) -> PreferencesDto) -> Unit,
) {
    Scaffold(topBar = { BackTopBar(stringResource(R.string.settings_notifications), onBack) }) { padding ->
        LoadableContent(state.preferences, onRetry, Modifier.padding(padding)) { prefs ->
            Column(Modifier.fillMaxSize().verticalScroll(rememberScrollState())) {
                // Aucun appareil n'est enregistré auprès de FCM (pas de projet Firebase) : ces deux
                // réglages ne commandent que le push, ils seraient sans effet. Valeurs serveur conservées.
                Text(
                    stringResource(R.string.prefs_push_unavailable),
                    style = MaterialTheme.typography.bodyMedium,
                    modifier = Modifier.padding(PoroSpacing.L),
                )
                Toggle(R.string.prefs_push, R.string.prefs_push_hint, prefs.pushEnabled, enabled = PUSH_AVAILABLE) { v ->
                    onChange { it.copy(pushEnabled = v) }
                }
                Toggle(R.string.prefs_likes, null, prefs.likes) { v -> onChange { it.copy(likes = v) } }
                Toggle(R.string.prefs_comments, null, prefs.comments) { v -> onChange { it.copy(comments = v) } }
                Toggle(R.string.prefs_follows, null, prefs.follows) { v -> onChange { it.copy(follows = v) } }
                Toggle(R.string.prefs_video_ready, null, prefs.videoReady) { v -> onChange { it.copy(videoReady = v) } }
                Toggle(R.string.prefs_orders, R.string.prefs_orders_hint, prefs.orders, enabled = PUSH_AVAILABLE) { v ->
                    onChange { it.copy(orders = v) }
                }
                state.error?.let { ErrorMessageText(it) }
            }
        }
    }
}

@Composable
private fun Toggle(label: Int, hint: Int?, checked: Boolean, enabled: Boolean = true, onChange: (Boolean) -> Unit) {
    ListItem(
        headlineContent = { Text(stringResource(label)) },
        supportingContent = hint?.let { { Text(stringResource(it)) } },
        trailingContent = { Switch(checked = checked, onCheckedChange = null, enabled = enabled) },
        modifier = Modifier.toggleable(value = checked, enabled = enabled, role = Role.Switch, onValueChange = onChange),
    )
}

/** Vrai quand l'app enregistrera son jeton FCM (PUT /api/v1/notifications/devices). */
private const val PUSH_AVAILABLE = false
