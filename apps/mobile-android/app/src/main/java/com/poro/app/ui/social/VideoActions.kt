package com.poro.app.ui.social

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Favorite
import androidx.compose.material.icons.filled.FavoriteBorder
import androidx.compose.material.icons.filled.Share
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.core.format.Formats
import com.poro.app.data.social.VideoStatsDto
import com.poro.app.ui.common.PoroIcons

/** Colonne d'actions sur une vidéo : j'aime, commentaires, partage, avec leurs compteurs. */
@Composable
fun VideoActions(
    stats: VideoStatsDto,
    onLike: () -> Unit,
    onComments: () -> Unit,
    onShare: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(modifier, verticalArrangement = Arrangement.spacedBy(12.dp), horizontalAlignment = Alignment.CenterHorizontally) {
        ActionButton(
            icon = if (stats.likedByMe) Icons.Filled.Favorite else Icons.Filled.FavoriteBorder,
            tint = if (stats.likedByMe) Color(0xFFFF3B5C) else Color.White,
            count = stats.likes,
            description = stringResource(if (stats.likedByMe) R.string.action_unlike else R.string.action_like, Formats.count(stats.likes)),
            onClick = onLike,
        )
        ActionButton(
            icon = PoroIcons.Comment,
            count = stats.comments,
            description = stringResource(R.string.action_comments, Formats.count(stats.comments)),
            onClick = onComments,
        )
        ActionButton(
            icon = Icons.Filled.Share,
            count = stats.shares,
            description = stringResource(R.string.action_share, Formats.count(stats.shares)),
            onClick = onShare,
        )
    }
}

@Composable
private fun ActionButton(icon: ImageVector, count: Long, description: String, onClick: () -> Unit, tint: Color = Color.White) {
    Column(horizontalAlignment = Alignment.CenterHorizontally) {
        IconButton(onClick = onClick) {
            Icon(icon, contentDescription = description, tint = tint, modifier = Modifier.size(32.dp))
        }
        // Le compteur est déjà dans la description du bouton : TalkBack ne le lit pas deux fois.
        Text(
            Formats.count(count),
            color = Color.White,
            style = MaterialTheme.typography.labelMedium,
            modifier = Modifier.clearAndSetSemantics {},
        )
    }
}
