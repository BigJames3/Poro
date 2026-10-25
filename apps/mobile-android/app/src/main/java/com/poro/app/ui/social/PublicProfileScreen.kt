package com.poro.app.ui.social

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.MoreVert
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.core.format.Formats
import com.poro.app.data.social.UserStatsDto
import com.poro.app.data.video.VideoDto
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.common.Avatar
import com.poro.app.ui.common.BackTopBar
import com.poro.app.ui.common.displayNameOf
import com.poro.app.ui.theme.PoroSpacing
import com.poro.app.ui.theme.brandButtonColors

@Composable
fun PublicProfileScreen(
    state: PublicProfileUiState,
    onBack: () -> Unit,
    onToggleFollow: () -> Unit,
    onReport: () -> Unit,
    onOpenFollowers: () -> Unit,
    onOpenFollowing: () -> Unit,
    onOpenVideo: (VideoDto) -> Unit = {},
    onLoadMoreVideos: () -> Unit = {},
) {
    val profile = state.profile
    val name = displayNameOf(profile?.displayName, profile?.username)
    var menu by remember { mutableStateOf(false) }
    val actions: @Composable () -> Unit = {
        if (!state.loading && !state.isMe) {
            Box {
                IconButton(onClick = { menu = true }) { Icon(Icons.Filled.MoreVert, contentDescription = stringResource(R.string.action_more)) }
                DropdownMenu(expanded = menu, onDismissRequest = { menu = false }) {
                    DropdownMenuItem(
                        text = { Text(stringResource(R.string.report_user)) },
                        onClick = {
                            menu = false
                            onReport()
                        },
                    )
                }
            }
        }
    }
    Scaffold(topBar = { BackTopBar(name, onBack, actions) }) { padding ->
        if (state.loading) {
            Box(Modifier.fillMaxSize().padding(padding), contentAlignment = Alignment.Center) { CircularProgressIndicator() }
            return@Scaffold
        }
        Column(
            modifier = Modifier.fillMaxSize().padding(padding).verticalScroll(rememberScrollState()).padding(PoroSpacing.XL),
            verticalArrangement = Arrangement.spacedBy(PoroSpacing.M),
            horizontalAlignment = Alignment.CenterHorizontally,
        ) {
            Avatar(profile?.avatarUrl, profile?.displayName ?: profile?.username, size = 96.dp)
            Text(name, style = MaterialTheme.typography.headlineSmall)
            profile?.username?.let { Text("@$it", color = MaterialTheme.colorScheme.onSurfaceVariant) }
            if (profile?.isCreator == true) Text(stringResource(R.string.profile_creator_badge), color = MaterialTheme.colorScheme.primary)
            profile?.bio?.takeIf { it.isNotBlank() }?.let { Text(it, textAlign = TextAlign.Center) }
            state.stats?.let { FollowCounts(it, onOpenFollowers, onOpenFollowing) }
            if (!state.isMe) {
                val follow = Modifier.fillMaxWidth().heightIn(min = 48.dp)
                // L'action principale (s'abonner) est le bouton plein de marque ; se désabonner reste discret.
                if (state.following) {
                    OutlinedButton(onClick = onToggleFollow, enabled = !state.followBusy, modifier = follow) {
                        Text(stringResource(R.string.action_unfollow))
                    }
                } else {
                    Button(onClick = onToggleFollow, enabled = !state.followBusy, colors = brandButtonColors(), modifier = follow) {
                        Text(stringResource(R.string.action_follow))
                    }
                }
            }
            state.error?.let { ErrorMessageText(it) }
            PublicPostsSection(state, onOpenVideo, onLoadMoreVideos)
        }
    }
}

/**
 * « 1,2 k abonnés · 85 abonnements » : chaque compteur ouvre sa liste, avec une cible
 * tactile d'au moins 48 dp et un libellé lu par TalkBack comme un bouton.
 */
@Composable
fun FollowCounts(stats: UserStatsDto, onFollowers: () -> Unit, onFollowing: () -> Unit, modifier: Modifier = Modifier) {
    Row(modifier, horizontalArrangement = Arrangement.spacedBy(PoroSpacing.S)) {
        FollowCount(stringResource(R.string.profile_followers, Formats.count(stats.followers)), onFollowers)
        FollowCount(stringResource(R.string.profile_following, Formats.count(stats.following)), onFollowing)
    }
}

@Composable
private fun FollowCount(label: String, onClick: () -> Unit) {
    Box(
        Modifier
            .heightIn(min = 48.dp)
            .widthIn(min = 48.dp)
            .clip(MaterialTheme.shapes.small)
            .clickable(role = Role.Button, onClick = onClick)
            .semantics(mergeDescendants = true) {}
            .padding(horizontal = PoroSpacing.S),
        contentAlignment = Alignment.Center,
    ) {
        Text(label, style = MaterialTheme.typography.titleSmall)
    }
}
