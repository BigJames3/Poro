package com.poro.app.ui.feed

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
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
import androidx.compose.foundation.pager.HorizontalPager
import androidx.compose.foundation.pager.VerticalPager
import androidx.compose.foundation.pager.rememberPagerState
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.selected
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import coil3.compose.AsyncImage
import com.poro.app.R
import com.poro.app.data.feed.FeedImage
import com.poro.app.data.feed.FeedItem
import com.poro.app.data.feed.FeedKind
import com.poro.app.ui.auth.ErrorMessage
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.common.Avatar
import com.poro.app.ui.common.PoroIcons
import com.poro.app.ui.common.displayNameOf
import com.poro.app.ui.player.VideoPlayer
import com.poro.app.ui.player.VideoSurface
import kotlinx.coroutines.delay

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
    onHashtag: (String) -> Unit = {},
) {
    val list = state.current
    Box(modifier.fillMaxSize().background(Color.Black)) {
        when {
            list.items.isNotEmpty() -> key(state.kind) {
                VideoPager(list.items, list.position, player, onItemShown, onOpenAuthor, actions, onHashtag)
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
        Column(Modifier.fillMaxWidth().padding(top = 8.dp, start = 8.dp, end = 8.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Row(Modifier.weight(1f), horizontalArrangement = Arrangement.Center) {
                    state.kinds.forEach { kind -> FeedTab(kind, selected = kind == state.kind, onClick = { onSelect(kind) }) }
                }
                topEnd()
            }
            // La suite du fil n'a pas pu être chargée : on le dit, au lieu d'un fil qui semble fini.
            val error = list.error
            if (error != null && list.items.isNotEmpty()) MoreFailed(error, onRetry)
        }
    }
}

@Composable
private fun MoreFailed(error: ErrorMessage, onRetry: () -> Unit) {
    Row(
        Modifier
            .fillMaxWidth()
            .padding(top = 8.dp)
            .background(Color.Black.copy(alpha = 0.7f), RoundedCornerShape(12.dp))
            .padding(horizontal = 12.dp, vertical = 4.dp)
            .semantics { liveRegion = LiveRegionMode.Polite },
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(stringResource(error.text), color = Color.White, style = MaterialTheme.typography.bodySmall, modifier = Modifier.weight(1f))
        TextButton(onClick = onRetry, modifier = Modifier.heightIn(min = 48.dp)) {
            Text(stringResource(R.string.action_retry), color = Color.White, fontWeight = FontWeight.Bold)
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
    initialPage: Int,
    player: VideoPlayer,
    onItemShown: (Int) -> Unit,
    onOpenAuthor: (FeedItem) -> Unit,
    actions: @Composable (FeedItem) -> Unit,
    onHashtag: (String) -> Unit,
) {
    val pager = rememberPagerState(initialPage = initialPage.coerceIn(0, maxOf(items.size - 1, 0)), pageCount = { items.size })
    // La liste s'allonge pendant le défilement : l'effet doit lire la version courante.
    val currentItems by rememberUpdatedState(items)
    val currentOnItemShown by rememberUpdatedState(onItemShown)
    // La page posée (pas celle en cours de glissement) décide de la vidéo lue ; la suivante est
    // préparée ensuite, dans le même flux, pour ne jamais écraser celle qui va être lue.
    LaunchedEffect(pager) {
        var shownPage = -1
        snapshotFlow { pager.settledPage to currentItems.getOrNull(pager.settledPage + 1)?.playableUrl }.collect { (page, nextUrl) ->
            if (page != shownPage) {
                shownPage = page
                currentOnItemShown(page)
                // Une publication photo ne joue rien : la vidéo précédente s'arrête.
                val url = currentItems.getOrNull(page)?.playableUrl
                if (url != null) player.play(url) else player.pause()
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
            onHashtag = onHashtag,
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
    onHashtag: (String) -> Unit = {},
) {
    val isPlaying by player.isPlaying
    val isMuted by player.isMuted
    val isBuffering by player.isBuffering
    val hasFailed by player.hasFailed
    val author = displayNameOf(item.author.displayName, item.author.username)
    val spoken = if (item.isPhoto) {
        stringResource(R.string.feed_photo_description, author, item.images.size)
    } else {
        stringResource(R.string.feed_video_description, item.title, author)
    }
    // Une publication photo se parcourt en glissant de côté : toucher ne lit ni ne met en pause.
    val playToggle = if (item.isPhoto) {
        Modifier
    } else {
        Modifier.clickable(role = Role.Button, onClickLabel = stringResource(R.string.feed_play_pause)) { player.toggle() }
    }
    Box(
        Modifier
            .fillMaxSize()
            .then(playToggle)
            .semantics { contentDescription = spoken },
    ) {
        if (item.isPhoto) {
            PhotoCarousel(item.images, Modifier.fillMaxSize())
        } else {
            AsyncImage(
                model = item.thumbnailUrl,
                contentDescription = null,
                contentScale = ContentScale.Crop,
                modifier = Modifier.fillMaxSize(),
            )
            if (active) VideoSurface(player, Modifier.fillMaxSize())
        }
        when {
            !active || item.isPhoto -> Unit
            hasFailed -> PlaybackFailed(onRetry = player::retry, modifier = Modifier.align(Alignment.Center))
            isBuffering -> {
                val loading = stringResource(R.string.loading)
                CircularProgressIndicator(
                    color = Color.White,
                    modifier = Modifier.align(Alignment.Center).semantics { contentDescription = loading },
                )
            }
            !isPlaying -> Icon(
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
                    if (item.hashtags.isNotEmpty()) Hashtags(item.hashtags, onHashtag)
                }
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    actions()
                    if (!item.isPhoto) {
                        IconButton(onClick = player::toggleMute) {
                            Icon(
                                if (isMuted) PoroIcons.VolumeOff else PoroIcons.VolumeOn,
                                contentDescription = stringResource(if (isMuted) R.string.feed_unmute else R.string.feed_mute),
                                tint = Color.White,
                            )
                        }
                    }
                }
            }
        }
        if (active && !item.isPhoto) PlaybackProgress(player, Modifier.align(Alignment.BottomCenter))
    }
}

/** Hashtags d'une vidéo : chacun lance la recherche des vidéos qui le portent. */
@Composable
private fun Hashtags(tags: List<String>, onHashtag: (String) -> Unit) {
    Row(Modifier.horizontalScroll(rememberScrollState())) {
        tags.forEach { tag ->
            val label = stringResource(R.string.feed_hashtag_search, tag)
            Box(
                Modifier
                    .heightIn(min = 48.dp)
                    .clickable(role = Role.Button, onClickLabel = label) { onHashtag(tag) }
                    .padding(end = 8.dp),
                contentAlignment = Alignment.CenterStart,
            ) {
                Text("#$tag", color = Color.White, style = MaterialTheme.typography.bodySmall, fontWeight = FontWeight.Bold)
            }
        }
    }
}

/** Fine barre d'avancement de la vidéo lue, décorative pour TalkBack. */
@Composable
private fun PlaybackProgress(player: VideoPlayer, modifier: Modifier = Modifier) {
    var progress by remember { mutableFloatStateOf(0f) }
    LaunchedEffect(player) {
        while (true) {
            progress = player.progress()
            delay(PROGRESS_TICK_MS)
        }
    }
    LinearProgressIndicator(
        progress = { progress },
        color = Color.White,
        trackColor = Color.White.copy(alpha = 0.25f),
        modifier = modifier.fillMaxWidth().height(2.dp).clearAndSetSemantics {},
    )
}

private const val PROGRESS_TICK_MS = 250L

/** La vidéo n'a pas pu être lue : un message et « Réessayer », au lieu d'une image figée. */
@Composable
private fun PlaybackFailed(onRetry: () -> Unit, modifier: Modifier = Modifier) {
    Column(
        modifier
            .background(Color.Black.copy(alpha = 0.7f), RoundedCornerShape(12.dp))
            .padding(16.dp)
            .semantics { liveRegion = LiveRegionMode.Polite },
        verticalArrangement = Arrangement.spacedBy(8.dp),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(stringResource(R.string.video_playback_failed), color = Color.White, textAlign = TextAlign.Center)
        OutlinedButton(onClick = onRetry, modifier = Modifier.heightIn(min = 48.dp)) {
            Text(stringResource(R.string.action_retry), color = Color.White)
        }
    }
}

/** URL HLS d'une vidéo ; null pour une publication photo, qui ne se lit pas. */
private val FeedItem.playableUrl: String?
    get() = hlsUrl?.takeIf { it.isNotEmpty() && !isPhoto }

/**
 * Carrousel d'une publication photo : une image par page, en entier sur fond noir, et des points
 * qui disent où l'on en est. Glisser de côté change d'image, glisser vers le haut de publication.
 */
@Composable
fun PhotoCarousel(images: List<FeedImage>, modifier: Modifier = Modifier) {
    val pager = rememberPagerState(pageCount = { images.size })
    Box(modifier.background(Color.Black)) {
        HorizontalPager(state = pager, modifier = Modifier.fillMaxSize()) { page ->
            val label = stringResource(R.string.feed_photo_page, page + 1, images.size)
            AsyncImage(
                model = images[page].url,
                contentDescription = label,
                contentScale = ContentScale.Fit,
                modifier = Modifier.fillMaxSize(),
            )
        }
        if (images.size > 1) {
            Row(
                Modifier.align(Alignment.TopCenter).padding(top = 64.dp).clearAndSetSemantics {},
                horizontalArrangement = Arrangement.spacedBy(6.dp),
            ) {
                repeat(images.size) { index ->
                    Box(
                        Modifier
                            .size(if (index == pager.currentPage) 8.dp else 6.dp)
                            .background(Color.White.copy(alpha = if (index == pager.currentPage) 1f else 0.5f), CircleShape),
                    )
                }
            }
        }
    }
}
