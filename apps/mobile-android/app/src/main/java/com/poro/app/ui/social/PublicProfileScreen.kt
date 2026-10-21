package com.poro.app.ui.social

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.core.format.Formats
import com.poro.app.data.social.UserStatsDto
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.common.Avatar
import com.poro.app.ui.common.BackTopBar
import com.poro.app.ui.common.displayNameOf

@Composable
fun PublicProfileScreen(state: PublicProfileUiState, onBack: () -> Unit, onToggleFollow: () -> Unit) {
    val profile = state.profile
    val name = displayNameOf(profile?.displayName, profile?.username)
    Scaffold(topBar = { BackTopBar(name, onBack) }) { padding ->
        if (state.loading) {
            Box(Modifier.fillMaxSize().padding(padding), contentAlignment = Alignment.Center) { CircularProgressIndicator() }
            return@Scaffold
        }
        Column(
            modifier = Modifier.fillMaxSize().padding(padding).verticalScroll(rememberScrollState()).padding(24.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Avatar(profile?.avatarUrl, profile?.displayName ?: profile?.username, size = 96.dp)
            Text(name, style = MaterialTheme.typography.headlineSmall)
            profile?.username?.let { Text("@$it", color = MaterialTheme.colorScheme.onSurfaceVariant) }
            if (profile?.isCreator == true) Text(stringResource(R.string.profile_creator_badge), color = MaterialTheme.colorScheme.primary)
            profile?.bio?.takeIf { it.isNotBlank() }?.let { Text(it, textAlign = TextAlign.Center) }
            state.stats?.let { FollowCounts(it) }
            if (!state.isMe) {
                val follow = Modifier.fillMaxWidth().heightIn(min = 48.dp)
                if (state.following) {
                    OutlinedButton(onClick = onToggleFollow, enabled = !state.followBusy, modifier = follow) {
                        Text(stringResource(R.string.action_unfollow))
                    }
                } else {
                    Button(onClick = onToggleFollow, enabled = !state.followBusy, modifier = follow) {
                        Text(stringResource(R.string.action_follow))
                    }
                }
            }
            state.error?.let { ErrorMessageText(it) }
        }
    }
}

/** « 1,2 k abonnés · 85 abonnements », lu d'une traite par TalkBack. */
@Composable
fun FollowCounts(stats: UserStatsDto, modifier: Modifier = Modifier) {
    val followers = stringResource(R.string.profile_followers, Formats.count(stats.followers))
    val following = stringResource(R.string.profile_following, Formats.count(stats.following))
    Row(
        modifier.clearAndSetSemantics { contentDescription = "$followers, $following" },
        horizontalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Text(followers, style = MaterialTheme.typography.titleSmall)
        Text(following, style = MaterialTheme.typography.titleSmall)
    }
}
