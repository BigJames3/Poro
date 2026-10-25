package com.poro.app.ui.social

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import coil3.compose.AsyncImage
import com.poro.app.R
import com.poro.app.data.video.VideoDto
import com.poro.app.ui.auth.ErrorMessageText

/**
 * Grille des publications d'un compte (3 par ligne), dans le défilement de son profil public.
 * Toucher une publication l'ouvre.
 */
@Composable
fun PublicPostsSection(
    state: PublicProfileUiState,
    onOpen: (VideoDto) -> Unit,
    onLoadMore: () -> Unit,
) {
    Column(Modifier.fillMaxWidth(), verticalArrangement = Arrangement.spacedBy(4.dp)) {
        Text(
            stringResource(R.string.profile_posts_title),
            style = MaterialTheme.typography.titleMedium,
            modifier = Modifier.semantics { heading() },
        )
        if (state.videosStarted && !state.videosLoading && state.videos.isEmpty() && state.videosError == null) {
            Text(stringResource(R.string.profile_posts_empty), style = MaterialTheme.typography.bodyMedium)
        }
        val photoLabel = stringResource(R.string.post_photo_label)
        state.videos.chunked(COLUMNS).forEach { row ->
            Row(horizontalArrangement = Arrangement.spacedBy(4.dp)) {
                row.forEach { video ->
                    val label = video.headline.ifBlank { if (video.isPhoto) photoLabel else "" }
                    Box(
                        Modifier
                            .weight(1f)
                            .aspectRatio(9f / 16f)
                            .background(Color.DarkGray)
                            .clickable(role = Role.Button, onClick = { onOpen(video) })
                            .semantics { contentDescription = label },
                    ) {
                        AsyncImage(video.thumbnailUrl, contentDescription = null, contentScale = ContentScale.Crop, modifier = Modifier.fillMaxSize())
                        if (video.isPhoto) {
                            Text(
                                photoLabel,
                                color = Color.White,
                                style = MaterialTheme.typography.labelSmall,
                                modifier = Modifier.align(Alignment.TopEnd).background(Color.Black.copy(alpha = 0.6f)).padding(4.dp),
                            )
                        }
                    }
                }
                repeat(COLUMNS - row.size) { Spacer(Modifier.weight(1f)) }
            }
        }
        if (state.videosLoading) {
            CircularProgressIndicator(Modifier.align(Alignment.CenterHorizontally).padding(8.dp))
        }
        state.videosError?.let {
            ErrorMessageText(it)
            TextButton(onClick = onLoadMore) { Text(stringResource(R.string.action_retry)) }
        }
        if (state.canLoadMoreVideos) TextButton(onClick = onLoadMore) { Text(stringResource(R.string.my_videos_more)) }
    }
}

private const val COLUMNS = 3
