package com.poro.app.ui.upload

import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import coil3.compose.AsyncImage
import com.poro.app.R
import com.poro.app.data.video.VideoDto
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.feed.videoStatusText

/**
 * Grille de ses vidéos (3 par ligne), dans le défilement du profil. Toucher une vidéo prête
 * l'ouvre ; un appui long propose de la supprimer.
 */
@OptIn(ExperimentalFoundationApi::class)
@Composable
fun MyVideosSection(
    state: MyVideosUiState,
    onOpen: (VideoDto) -> Unit,
    onDelete: (VideoDto) -> Unit,
    onLoadMore: () -> Unit,
) {
    var toDelete by remember { mutableStateOf<VideoDto?>(null) }
    Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Text(
            stringResource(R.string.my_videos_title),
            style = MaterialTheme.typography.titleMedium,
            modifier = Modifier.semantics { heading() },
        )
        if (state.started && !state.loading && state.videos.isEmpty() && state.error == null) {
            Text(stringResource(R.string.my_videos_empty), style = MaterialTheme.typography.bodyMedium)
        }
        state.videos.chunked(COLUMNS).forEach { row ->
            Row(horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                row.forEach { video ->
                    val name = video.title.ifBlank { if (video.isPhoto) stringResource(R.string.post_photo_label) else "" }
                    val label = name + if (!video.isReady) ", " + stringResource(videoStatusText(video)) else ""
                    Box(
                        Modifier
                            .weight(1f)
                            .aspectRatio(9f / 16f)
                            .background(Color.DarkGray)
                            .combinedClickable(
                                onClick = { if (video.isReady) onOpen(video) },
                                onLongClick = { toDelete = video },
                                onLongClickLabel = stringResource(R.string.action_delete),
                            )
                            .semantics { contentDescription = label },
                    ) {
                        AsyncImage(video.thumbnailUrl, contentDescription = null, contentScale = ContentScale.Crop, modifier = Modifier.fillMaxSize())
                        if (video.isPhoto && video.isReady) {
                            Text(
                                stringResource(R.string.post_photo_label),
                                color = Color.White,
                                style = MaterialTheme.typography.labelSmall,
                                modifier = Modifier.align(Alignment.TopEnd).background(Color.Black.copy(alpha = 0.6f)).padding(4.dp),
                            )
                        }
                        if (!video.isReady) {
                            Text(
                                stringResource(statusLabel(video)),
                                color = Color.White,
                                style = MaterialTheme.typography.labelSmall,
                                textAlign = TextAlign.Center,
                                modifier = Modifier.align(Alignment.Center).background(Color.Black.copy(alpha = 0.6f)).padding(4.dp),
                            )
                        }
                    }
                }
                repeat(COLUMNS - row.size) { Spacer(Modifier.weight(1f)) }
            }
        }
        state.error?.let { ErrorMessageText(it) }
        if (state.canLoadMore) TextButton(onClick = onLoadMore) { Text(stringResource(R.string.my_videos_more)) }
    }
    toDelete?.let { video ->
        AlertDialog(
            onDismissRequest = { toDelete = null },
            title = { Text(stringResource(R.string.my_videos_delete_title)) },
            text = { Text(stringResource(R.string.my_videos_delete_text, video.title)) },
            confirmButton = {
                TextButton(onClick = {
                    onDelete(video)
                    toDelete = null
                }) { Text(stringResource(R.string.action_delete)) }
            },
            dismissButton = { TextButton(onClick = { toDelete = null }) { Text(stringResource(R.string.action_cancel)) } },
        )
    }
}

/** Libellé court sur la miniature d'une vidéo non lisible. */
private fun statusLabel(video: VideoDto): Int = when {
    video.moderationStatus == VideoDto.MODERATION_REMOVED -> R.string.status_removed
    video.status == VideoDto.STATUS_FAILED -> R.string.status_failed
    else -> R.string.status_processing
}

private const val COLUMNS = 3
