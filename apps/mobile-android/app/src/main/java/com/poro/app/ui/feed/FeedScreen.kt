package com.poro.app.ui.feed

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.pager.VerticalPager
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
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
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
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
import com.poro.app.ui.common.displayNameOf
import com.poro.app.ui.player.VideoPlayer
import com.poro.app.ui.player.VideoSurface

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
    actions: @Composable (FeedItem) -> Unit = {},
    topEnd: @Composable () -> Unit = {},
) {
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
            list.isEmpty -> Text(
                stringResource(if (state.kind == FeedKind.Following) R.string.feed_following_empty else R.string.feed_empty),
                color = Color.White,
                textAlign = TextAlign.Center,
                modifier = Modifier.align(Alignment.Center).padding(32.dp),
            )
            else -> {
                val loading = stringResource(R.string.loading)
                CircularProgressIndicator(Modifier.align(Alignment.Center).semantics { contentDescription = loading })
            }
        }
        Row(
            modifier = Modifier.fillMaxWidth().padding(top = 8.dp, start = 8.dp, end = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Row(Modifier.weight(1f), horizontalArrangement = Arrangement.Center) {
                state.kinds.forEach { kind -> FeedTab(kind, selected = kind == state.kind, onClick = { onSelect(kind) }) }
            }
            topEnd()
        }
    }
}

@Composable
private fun FeedTab(kind: FeedKind, selected: Boolean, onClick: () -> Unit) {
    val label = stringResource(
        when (kind) {
            FeedKind.ForYou, FeedKind.Guest -> R.string.feed_for_you
            FeedKind.Following -> R.string.feed_following
            FeedKind.Trending -> R.string.feed_trending
        },
    )
    TextButton(onClick = onClick, modifier = Modifier.heightIn(min = 48.dp).semantics { this.selected = selected }) {
        Text(
            label,
            color = if (selected) Color.White else Color.White.copy(alpha = 0.7f),
            fontWeight = if (selected) FontWeight.Bold else FontWeight.Normal,
        )
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
    // La page posée (pas celle en cours de glissement) décide de la vidéo lue ; la suivante est
    // préparée ensuite, dans le même flux, pour ne jamais écraser celle qui va être lue.
    LaunchedEffect(pager) {
        var shownPage = -1
        snapshotFlow { pager.settledPage to currentItems.getOrNull(pager.settledPage + 1)?.hlsUrl }.collect { (page, nextUrl) ->
            if (page != shownPage) {
                shownPage = page
                currentOnItemShown(page)
                currentItems.getOrNull(page)?.hlsUrl?.let(player::play)
            }
            player.preload(nextUrl)
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
    val author = displayNameOf(item.author.displayName, item.author.username)
    val spoken = stringResource(R.string.feed_video_description, item.title, author)
    Box(
        Modifier
            .fillMaxSize()
            .clickable(role = Role.Button, onClickLabel = stringResource(R.string.feed_play_pause)) { player.toggle() }
            .semantics { contentDescription = spoken },
    ) {
        AsyncImage(
            model = item.thumbnailUrl,
            contentDescription = null,
            contentScale = ContentScale.Crop,
            modifier = Modifier.fillMaxSize(),
        )
        if (active) VideoSurface(player, Modifier.fillMaxSize())
        if (active && !isPlaying) {
            Icon(
                Icons.Filled.PlayArrow,
                contentDescription = null,
                tint = Color.White.copy(alpha = 0.8f),
                modifier = Modifier.align(Alignment.Center).size(72.dp),
            )
        }
        // Dégradé sous le texte : lisible sur une vidéo claire.
        Box(
            Modifier
                .align(Alignment.BottomCenter)
                .fillMaxWidth()
                .background(Brush.verticalGradient(listOf(Color.Transparent, Color.Black.copy(alpha = 0.6f))))
                .padding(16.dp),
        ) {
            Row(verticalAlignment = Alignment.Bottom) {
                Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                        modifier = Modifier.heightIn(min = 48.dp).clickable(role = Role.Button, onClick = onOpenAuthor),
                    ) {
                        Avatar(item.author.avatarUrl, item.author.displayName ?: item.author.username, size = 36.dp)
                        Text(author, color = Color.White, style = MaterialTheme.typography.titleSmall)
                    }
                    Text(item.title, color = Color.White, maxLines = 2, overflow = TextOverflow.Ellipsis)
                    description?.takeIf { it.isNotBlank() }?.let {
                        Text(it, color = Color.White.copy(alpha = 0.85f), maxLines = 3, overflow = TextOverflow.Ellipsis)
                    }
                    if (item.hashtags.isNotEmpty()) {
                        Text(
                            item.hashtags.joinToString(" ") { "#$it" },
                            color = Color.White.copy(alpha = 0.85f),
                            style = MaterialTheme.typography.bodySmall,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                }
                actions()
            }
        }
    }
}
