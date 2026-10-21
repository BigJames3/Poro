package com.poro.app.ui.profile

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.common.Avatar
import com.poro.app.ui.common.CreatorBadge
import com.poro.app.ui.common.LoadableContent
import com.poro.app.ui.common.displayNameOf
import com.poro.app.ui.theme.PoroSize
import com.poro.app.ui.theme.PoroSpace

/**
 * Profil de l'utilisateur connecté. [extra] accueille ce qui dépend d'autres services
 * (compteurs d'abonnés, vidéos publiées) sans alourdir ce ViewModel.
 */
@Composable
fun ProfileScreen(
    state: ProfileUiState,
    onRetry: () -> Unit,
    onEdit: () -> Unit,
    onSettings: () -> Unit,
    onBecomeCreator: () -> Unit,
    onVerifyEmail: () -> Unit,
    modifier: Modifier = Modifier,
    extra: @Composable (MyAccount) -> Unit = {},
) {
    LoadableContent(state.content, onRetry, modifier) { me ->
        val profile = me.profile
        Column(
            modifier = Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(PoroSpace.lg),
            verticalArrangement = Arrangement.spacedBy(PoroSpace.xs),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Avatar(profile.avatarUrl, profile.displayName ?: profile.username, size = PoroSize.avatarL)
            Text(
                displayNameOf(profile.displayName, profile.username),
                style = MaterialTheme.typography.headlineSmall,
                modifier = Modifier.semantics { heading() },
            )
            if (profile.username != null) {
                Text("@${profile.username}", style = MaterialTheme.typography.bodyLarge, color = MaterialTheme.colorScheme.onSurfaceVariant)
            } else {
                Text(stringResource(R.string.profile_no_username), textAlign = TextAlign.Center)
            }
            if (profile.isCreator) CreatorBadge()
            profile.bio?.takeIf { it.isNotBlank() }?.let { Text(it, textAlign = TextAlign.Center) }
            if (me.needsEmailVerification) {
                Card(colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.secondaryContainer)) {
                    Row(
                        modifier = Modifier.padding(start = 16.dp, end = 8.dp, top = 8.dp, bottom = 8.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text(stringResource(R.string.profile_verify_email), modifier = Modifier.weight(1f))
                        TextButton(onClick = onVerifyEmail) { Text(stringResource(R.string.profile_verify_email_action)) }
                    }
                }
            }
            extra(me)
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp), modifier = Modifier.fillMaxWidth()) {
                OutlinedButton(onClick = onEdit, modifier = Modifier.weight(1f).heightIn(min = 48.dp)) {
                    Text(stringResource(R.string.profile_edit))
                }
                OutlinedButton(onClick = onSettings, modifier = Modifier.weight(1f).heightIn(min = 48.dp)) {
                    Text(stringResource(R.string.profile_settings))
                }
            }
            if (!profile.isCreator) {
                Button(
                    onClick = onBecomeCreator,
                    enabled = profile.username != null && !state.becomingCreator,
                    modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp),
                ) {
                    Text(stringResource(R.string.profile_become_creator))
                }
                if (profile.username == null) {
                    Text(stringResource(R.string.profile_creator_hint), style = MaterialTheme.typography.bodySmall)
                }
            }
            state.error?.let { ErrorMessageText(it) }
        }
    }
}
