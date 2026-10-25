package com.poro.app.ui.feed

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.style.TextAlign
import com.poro.app.R
import com.poro.app.data.feed.Author
import com.poro.app.data.feed.FeedImage
import com.poro.app.data.feed.FeedItem
import com.poro.app.data.user.ProfileDto
import com.poro.app.data.video.VideoDto
import com.poro.app.ui.common.Loadable
import com.poro.app.ui.common.LoadableContent
import com.poro.app.ui.player.VideoPlayer
import com.poro.app.ui.theme.PoroSpacing

/** Une vidéo plein écran ; une vidéo pas encore prête (ou retirée) n'affiche que son statut. */
@Composable
fun VideoScreen(
    state: Loadable<VideoDto>,
    player: VideoPlayer,
    onBack: () -> Unit,
    onRetry: () -> Unit,
    onOpenAuthor: (userId: String, username: String?) -> Unit,
    actions: @Composable (VideoDto) -> Unit = {},
    author: ProfileDto? = null,
) {
    Box(Modifier.fillMaxSize().background(Color.Black)) {
        LoadableContent(state, onRetry) { video ->
            if (video.isReady && video.isPhoto && video.images.isNotEmpty()) {
                // Publication photo : le carrousel et sa légende, rien à lire.
                VideoPage(
                    item = video.asFeedItem(author),
                    player = player,
                    active = true,
                    onOpenAuthor = { onOpenAuthor(video.userId, author?.username) },
                    actions = { actions(video) },
                    description = video.description,
                )
            } else if (video.isReady && video.hlsUrl != null) {
                LaunchedEffect(video.hlsUrl) { player.play(video.hlsUrl) }
                VideoPage(
                    item = video.asFeedItem(author),
                    player = player,
                    active = true,
                    onOpenAuthor = { onOpenAuthor(video.userId, author?.username) },
                    actions = { actions(video) },
                    description = video.description,
                )
            } else {
                Text(
                    stringResource(videoStatusText(video)),
                    color = Color.White,
                    textAlign = TextAlign.Center,
                    modifier = Modifier.align(Alignment.Center).padding(PoroSpacing.XXL),
                )
            }
        }
        IconButton(onClick = onBack, modifier = Modifier.padding(PoroSpacing.S)) {
            Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = stringResource(R.string.action_back), tint = Color.White)
        }
    }
}

/** Statut d'une vidéo qui ne peut pas être lue (seul son auteur la voit dans cet état). */
fun videoStatusText(video: VideoDto): Int = when {
    video.moderationStatus == VideoDto.MODERATION_REMOVED -> R.string.video_removed
    video.status == VideoDto.STATUS_FAILED -> R.string.video_failed
    video.status == VideoDto.STATUS_READY -> R.string.video_ready
    else -> R.string.video_processing
}

/** La page de fil sert aussi à une vidéo seule ; son auteur vient de son profil, lu à part. */
private fun VideoDto.asFeedItem(author: ProfileDto?) = FeedItem(
    videoId = id,
    title = title,
    durationMs = durationMs,
    thumbnailUrl = thumbnailUrl,
    hlsUrl = hlsUrl,
    mediaType = mediaType,
    images = images.sortedBy { it.position }.map { FeedImage(it.url, it.thumbnailUrl, it.width, it.height) },
    author = Author(id = userId, username = author?.username, displayName = author?.displayName, avatarUrl = author?.avatarUrl),
)
