package com.poro.app.ui.feed

import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.pager.VerticalPager
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.role
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import coil3.compose.AsyncImage
import com.poro.app.R
import com.poro.app.data.feed.FeedItem
import com.poro.app.data.feed.FeedKind
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.common.Avatar
import com.poro.app.ui.common.EmptyState
import com.poro.app.ui.common.PoroIcons
import com.poro.app.ui.common.displayNameOf
import com.poro.app.ui.player.VideoPlayer
import com.poro.app.ui.player.VideoSurface
import com.poro.app.ui.theme.OverlaySystemBars
import com.poro.app.ui.theme.PoroOrange
import com.poro.app.ui.theme.PoroOverlayInk
import com.poro.app.ui.theme.PoroOverlayMuted
import com.poro.app.ui.theme.PoroScrimBottomEnd
import com.poro.app.ui.theme.PoroScrimTop
import com.poro.app.ui.theme.PoroSize
import com.poro.app.ui.theme.PoroSpace

/**
 * Fil plein écran : une vidéo par page, défilement vertical. [actions] dessine la colonne
 * d'actions d'une vidéo (j'aime, commentaires, partage), [topEnd] le coin haut droit.
 */
@Composable
fun FeedScreen(
    state: FeedUiState,
    player: VideoPlayer,
    onSelect: (FeedKind) -> Unit,
    onItemShown: (Int) -> Unit,
    onRetry: () -> Unit,
    onOpenAuthor: (FeedItem) -> Unit,
    modifier: Modifier = Modifier,
    onFindCreators: () -> Unit = {},
    actions: @Composable (FeedItem) -> Unit = {},
    topEnd: @Composable () -> Unit = {},
) {
    OverlaySystemBars()
    val list = state.current
    Box(modifier.fillMaxSize().background(Color.Black)) {
        when {
            list.items.isNotEmpty() -> key(state.kind) {
                VideoPager(list.items, player, onItemShown, onOpenAuthor, actions)
            }
            list.error != null -> Column(
                Modifier.fillMaxSize().padding(24.dp),
                verticalArrangement = Arrangement.spacedBy(16.dp, Alignment.CenterVertically),
                horizontalAlignment = Alignment.CenterHorizontally,
            ) {
                ErrorMessageText(list.error)
                OutlinedButton(onClick = onRetry) { Text(stringResource(R.string.action_retry)) }
            }
            list.isEmpty -> EmptyState(
                title = stringResource(if (state.kind == FeedKind.Following) R.string.feed_following_empty else R.string.feed_empty),
                actionLabel = if (state.kind == FeedKind.Following) stringResource(R.string.feed_following_search) else null,
                onAction = if (state.kind == FeedKind.Following) onFindCreators else null,
                color = PoroOverlayInk,
                modifier = Modifier.align(Alignment.Center),
            )
            else -> {
                val loading = stringResource(R.string.loading)
                CircularProgressIndicator(Modifier.align(Alignment.Center).semantics { contentDescription = loading })
            }
        }
        Box(
            Modifier
                .fillMaxWidth()
                .height(88.dp)
                .background(Brush.verticalGradient(listOf(PoroScrimTop, Color.Transparent))),
        )
        Row(
            modifier = Modifier.fillMaxWidth().padding(top = PoroSpace.xs, start = PoroSpace.xs, end = PoroSpace.xs),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Row(Modifier.weight(1f), horizontalArrangement = Arrangement.Center) {
                FeedKind.entries.forEach { kind -> FeedTab(kind, selected = kind == state.kind, onClick = { onSelect(kind) }) }
            }
            topEnd()
        }
    }
}

@Composable
private fun FeedTab(kind: FeedKind, selected: Boolean, onClick: () -> Unit) {
    val label = stringResource(
        when (kind) {
            FeedKind.ForYou -> R.string.feed_for_you
            FeedKind.Following -> R.string.feed_following
            FeedKind.Trending -> R.string.feed_trending
        },
    )
    TextButton(
        onClick = onClick,
        modifier = Modifier.heightIn(min = PoroSize.touch).semantics {
            this.selected = selected
            this.role = Role.Tab
        },
    ) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Text(
                label,
                color = if (selected) PoroOverlayInk else PoroOverlayInk.copy(alpha = 0.8f),
                fontWeight = if (selected) FontWeight.Bold else FontWeight.Normal,
            )
            Box(
                Modifier
                    .padding(top = 2.dp)
                    .width(16.dp)
                    .height(2.dp)
                    .background(if (selected) PoroOrange else Color.Transparent),
            )
        }
    }
}

@Composable
private fun VideoPager(
    items: List<FeedItem>,
    player: VideoPlayer,
    onItemShown: (Int) -> Unit,
    onOpenAuthor: (FeedItem) -> Unit,
    actions: @Composable (FeedItem) -> Unit,
) {
    val pager = rememberPagerState(pageCount = { items.size })
    // La liste s'allonge pendant le défilement : l'effet doit lire la version courante.
    val currentItems by rememberUpdatedState(items)
    val currentOnItemShown by rememberUpdatedState(onItemShown)
    // La page posée (pas celle en cours de glissement) décide de la vidéo lue.
    LaunchedEffect(pager) {
        snapshotFlow { pager.settledPage }.collect { page ->
            currentOnItemShown(page)
            currentItems.getOrNull(page)?.hlsUrl?.let(player::play)
        }
    }
    VerticalPager(state = pager, key = { items[it].videoId }, modifier = Modifier.fillMaxSize()) { page ->
        VideoPage(
            item = items[page],
            player = player,
            active = page == pager.settledPage,
            onOpenAuthor = { onOpenAuthor(items[page]) },
            actions = { actions(items[page]) },
        )
    }
}

/** Une vidéo : miniature, surface de lecture si elle est affichée, puis titre, auteur et actions. */
@Composable
fun VideoPage(
    item: FeedItem,
    player: VideoPlayer,
    active: Boolean,
    onOpenAuthor: () -> Unit,
    actions: @Composable () -> Unit,
    description: String? = null,
) {
    val isPlaying by player.isPlaying
    val hasError by player.hasError
    val isBuffering by player.isBuffering
    val author = displayNameOf(item.author.displayName, item.author.username)
    val spoken = stringResource(R.string.feed_video_description, item.title, author)
    val tapLabel = stringResource(if (hasError) R.string.action_retry else R.string.feed_play_pause)
    Box(
        Modifier
            .fillMaxSize()
            .clickable(role = Role.Button, onClickLabel = tapLabel) { player.toggle() }
            .semantics { contentDescription = spoken },
    ) {
        AsyncImage(
            model = item.thumbnailUrl,
            contentDescription = null,
            contentScale = ContentScale.Crop,
            modifier = Modifier.fillMaxSize(),
        )
        if (active && !hasError) VideoSurface(player, Modifier.fillMaxSize())
        if (active && hasError) {
            Column(
                Modifier.align(Alignment.Center).padding(PoroSpace.lg),
                horizontalAlignment = Alignment.CenterHorizontally,
                verticalArrangement = Arrangement.spacedBy(PoroSpace.md),
            ) {
                Text(stringResource(R.string.feed_playback_error), color = PoroOverlayInk, textAlign = TextAlign.Center)
                OutlinedButton(
                    onClick = { player.retry() },
                    colors = ButtonDefaults.outlinedButtonColors(contentColor = PoroOverlayInk),
                    border = BorderStroke(1.dp, PoroOverlayInk),
                ) { Text(stringResource(R.string.action_retry)) }
            }
        } else if (active && isBuffering && !isPlaying) {
            val loading = stringResource(R.string.loading)
            CircularProgressIndicator(
                Modifier.align(Alignment.Center).semantics { contentDescription = loading },
                color = PoroOverlayInk,
            )
        } else if (active && !isPlaying) {
            Icon(
                Icons.Filled.PlayArrow,
                contentDescription = null,
                tint = PoroOverlayInk,
                modifier = Modifier
                    .align(Alignment.Center)
                    .size(PoroSize.playGlyph)
                    .shadow(8.dp, CircleShape),
            )
        }
        // Dégradé sous le texte : lisible sur une vidéo claire.
        Box(
            Modifier
                .align(Alignment.BottomCenter)
                .fillMaxWidth()
                .background(Brush.verticalGradient(listOf(Color.Transparent, PoroScrimBottomEnd)))
                .padding(PoroSpace.md),
        ) {
            Row(verticalAlignment = Alignment.Bottom) {
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(PoroSpace.xs),
                        modifier = Modifier.heightIn(min = PoroSize.touch).clickable(role = Role.Button, onClick = onOpenAuthor),
                    ) {
                        Avatar(item.author.avatarUrl, item.author.displayName ?: item.author.username, size = PoroSize.avatarM)
                        Text(author, color = PoroOverlayInk, style = MaterialTheme.typography.titleSmall)
                    }
                    Text(item.title, color = PoroOverlayInk, maxLines = 2, overflow = TextOverflow.Ellipsis)
                    description?.takeIf { it.isNotBlank() }?.let {
                        Text(it, color = PoroOverlayMuted, maxLines = 3, overflow = TextOverflow.Ellipsis)
                    }
                    if (item.hashtags.isNotEmpty()) {
                        Text(
                            item.hashtags.joinToString(" ") { "#$it" },
                            color = PoroOverlayMuted,
                            style = MaterialTheme.typography.bodySmall,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                }
                Column(horizontalAlignment = Alignment.CenterHorizontally, verticalArrangement = Arrangement.spacedBy(PoroSpace.sm)) {
                    if (active) MuteButton(player)
                    actions()
                }
            }
        }
    }
}

@Composable
private fun MuteButton(player: VideoPlayer) {
    val muted by player.isMuted
    IconButton(onClick = { player.toggleMute() }, modifier = Modifier.size(PoroSize.touch)) {
        Icon(
            if (muted) PoroIcons.VolumeOff else PoroIcons.VolumeOn,
            contentDescription = stringResource(if (muted) R.string.feed_unmute else R.string.feed_mute),
            tint = PoroOverlayInk,
            modifier = Modifier.size(PoroSize.actionIcon),
        )
    }
}
