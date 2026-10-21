package com.poro.app.ui.profile

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.core.format.Formats
import com.poro.app.data.auth.SessionDto
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.common.BackTopBar
import com.poro.app.ui.common.Loadable

@Composable
fun SettingsScreen(
    state: SettingsUiState,
    onBack: () -> Unit,
    onRetrySessions: () -> Unit,
    onRevoke: (String) -> Unit,
    onRevokeOthers: () -> Unit,
    onDeleteAccount: () -> Unit,
    onLogout: () -> Unit,
) {
    Scaffold(topBar = { BackTopBar(stringResource(R.string.settings_title), onBack) }) { padding ->
        LazyColumn(modifier = Modifier.fillMaxSize().padding(padding)) {
            item { SectionTitle(stringResource(R.string.settings_devices)) }
            when (val sessions = state.sessions) {
                Loadable.Loading -> item { CircularProgressIndicator(Modifier.padding(16.dp)) }
                is Loadable.Failed -> item {
                    Column(Modifier.padding(16.dp)) {
                        ErrorMessageText(sessions.error)
                        TextButton(onClick = onRetrySessions) { Text(stringResource(R.string.action_retry)) }
                    }
                }
                is Loadable.Loaded -> {
                    items(sessions.value, key = { it.id }) { session ->
                        SessionRow(session, revoking = session.id in state.revoking, onRevoke = { onRevoke(session.id) })
                    }
                    if (sessions.value.any { !it.current }) {
                        item {
                            OutlinedButton(
                                onClick = onRevokeOthers,
                                enabled = !state.revokingOthers,
                                modifier = Modifier.fillMaxWidth().padding(horizontal = 16.dp).heightIn(min = 48.dp),
                            ) {
                                Text(stringResource(R.string.settings_sign_out_others))
                            }
                        }
                    }
                }
            }
            state.error?.let { item { ErrorMessageText(it, Modifier.padding(16.dp)) } }
            item { HorizontalDivider(Modifier.padding(vertical = 8.dp)) }
            item { SectionTitle(stringResource(R.string.settings_account)) }
            item { SettingsAction(stringResource(R.string.action_logout), onLogout, enabled = !state.signingOut) }
            item {
                SettingsAction(
                    stringResource(R.string.settings_delete_account),
                    onDeleteAccount,
                    color = MaterialTheme.colorScheme.error,
                )
            }
        }
    }
}

@Composable
private fun SectionTitle(text: String) {
    Text(
        text,
        style = MaterialTheme.typography.titleMedium,
        modifier = Modifier.padding(horizontal = 16.dp, vertical = 12.dp).semantics { heading() },
    )
}

@Composable
private fun SessionRow(session: SessionDto, revoking: Boolean, onRevoke: () -> Unit) {
    val now = System.currentTimeMillis()
    ListItem(
        headlineContent = { Text(session.device ?: stringResource(R.string.settings_unknown_device)) },
        supportingContent = {
            Text(
                if (session.current) {
                    stringResource(R.string.settings_this_device)
                } else {
                    stringResource(R.string.settings_last_active, Formats.relative(session.lastActiveAt, now))
                },
            )
        },
        trailingContent = {
            if (!session.current) {
                TextButton(onClick = onRevoke, enabled = !revoking) { Text(stringResource(R.string.settings_sign_out_device)) }
            }
        },
    )
}

@Composable
private fun SettingsAction(
    text: String,
    onClick: () -> Unit,
    enabled: Boolean = true,
    color: Color = MaterialTheme.colorScheme.primary,
) {
    Column(Modifier.fillMaxWidth().padding(horizontal = 8.dp), verticalArrangement = Arrangement.Center) {
        TextButton(onClick = onClick, enabled = enabled, modifier = Modifier.heightIn(min = 48.dp)) {
            Text(text, color = color)
        }
    }
}
