package com.poro.app.ui.notification

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.selection.toggleable
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.ListItem
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
                Toggle(R.string.prefs_push, R.string.prefs_push_hint, prefs.pushEnabled) { v -> onChange { it.copy(pushEnabled = v) } }
                Toggle(R.string.prefs_likes, null, prefs.likes) { v -> onChange { it.copy(likes = v) } }
                Toggle(R.string.prefs_comments, null, prefs.comments) { v -> onChange { it.copy(comments = v) } }
                Toggle(R.string.prefs_follows, null, prefs.follows) { v -> onChange { it.copy(follows = v) } }
                Toggle(R.string.prefs_video_ready, null, prefs.videoReady) { v -> onChange { it.copy(videoReady = v) } }
                Toggle(R.string.prefs_orders, R.string.prefs_orders_hint, prefs.orders) { v -> onChange { it.copy(orders = v) } }
                state.error?.let { ErrorMessageText(it) }
            }
        }
    }
}

@Composable
private fun Toggle(label: Int, hint: Int?, checked: Boolean, onChange: (Boolean) -> Unit) {
    ListItem(
        headlineContent = { Text(stringResource(label)) },
        supportingContent = hint?.let { { Text(stringResource(it)) } },
        trailingContent = { Switch(checked = checked, onCheckedChange = null) },
        modifier = Modifier.toggleable(value = checked, role = Role.Switch, onValueChange = onChange),
    )
}
