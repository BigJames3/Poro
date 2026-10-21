package com.poro.app.ui.social

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.Send
import androidx.compose.material.icons.filled.Favorite
import androidx.compose.material.icons.filled.FavoriteBorder
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontStyle
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.core.format.Formats
import com.poro.app.data.social.CommentDto
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.common.Avatar
import com.poro.app.ui.moderation.OverflowAction
import com.poro.app.ui.moderation.OverflowMenu

/** Actions de la feuille des commentaires, transmises par l'écran qui l'ouvre. */
class CommentsActions(
    val onLoadMore: () -> Unit,
    val onToggleReplies: (String) -> Unit,
    val onLoadMoreReplies: (String) -> Unit,
    val onReplyTo: (CommentDto?) -> Unit,
    val onDraftChange: (String) -> Unit,
    val onPost: () -> Unit,
    val onLike: (CommentDto) -> Unit,
    val onDelete: (CommentDto) -> Unit,
    val onReport: (CommentDto) -> Unit,
)

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun CommentsSheet(state: CommentsUiState, actions: CommentsActions, onDismiss: () -> Unit) {
    ModalBottomSheet(onDismissRequest = onDismiss) {
        Column(Modifier.fillMaxHeight(0.85f).imePadding()) {
            Text(
                stringResource(R.string.comments_title),
                style = MaterialTheme.typography.titleMedium,
                modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp).semantics { heading() },
            )
            LazyColumn(Modifier.weight(1f)) {
                if (state.endReached && state.comments.isEmpty()) {
                    item { Text(stringResource(R.string.comments_empty), modifier = Modifier.padding(16.dp)) }
                }
                items(state.comments, key = { it.id }) { comment ->
                    CommentRow(comment, state.myUserId, actions)
                    val replies = state.replies[comment.id]
                    if (comment.repliesCount > 0) {
                        TextButton(
                            onClick = { actions.onToggleReplies(comment.id) },
                            modifier = Modifier.padding(start = 56.dp),
                        ) {
                            Text(
                                if (replies?.expanded == true) {
                                    stringResource(R.string.comments_hide_replies)
                                } else {
                                    stringResource(R.string.comments_show_replies, comment.repliesCount)
                                },
                            )
                        }
                    }
                    if (replies?.expanded == true) {
                        replies.items.forEach { reply -> CommentRow(reply, state.myUserId, actions, Modifier.padding(start = 40.dp)) }
                        if (replies.loading) CircularProgressIndicator(Modifier.padding(start = 56.dp))
                        if (replies.nextCursor != null && !replies.loading) {
                            TextButton(onClick = { actions.onLoadMoreReplies(comment.id) }, modifier = Modifier.padding(start = 56.dp)) {
                                Text(stringResource(R.string.comments_more_replies))
                            }
                        }
                    }
                }
                if (!state.endReached) {
                    item {
                        // Dernier élément affiché : la page suivante se charge d'elle-même.
                        LaunchedEffect(state.comments.size) { actions.onLoadMore() }
                        CircularProgressIndicator(Modifier.padding(16.dp))
                    }
                }
            }
            state.error?.let { ErrorMessageText(it, Modifier.padding(horizontal = 16.dp)) }
            state.replyingTo?.let {
                Row(Modifier.padding(horizontal = 16.dp), verticalAlignment = Alignment.CenterVertically) {
                    Text(stringResource(R.string.comments_replying), style = MaterialTheme.typography.bodySmall, modifier = Modifier.weight(1f))
                    TextButton(onClick = { actions.onReplyTo(null) }) { Text(stringResource(R.string.action_cancel)) }
                }
            }
            Row(Modifier.fillMaxWidth().padding(8.dp), verticalAlignment = Alignment.CenterVertically) {
                OutlinedTextField(
                    value = state.draft,
                    onValueChange = actions.onDraftChange,
                    placeholder = { Text(stringResource(R.string.comments_placeholder)) },
                    maxLines = 4,
                    modifier = Modifier.weight(1f),
                )
                IconButton(onClick = actions.onPost, enabled = state.canPost) {
                    Icon(Icons.AutoMirrored.Filled.Send, contentDescription = stringResource(R.string.comments_send))
                }
            }
        }
    }
}

@Composable
private fun CommentRow(comment: CommentDto, myUserId: String?, actions: CommentsActions, modifier: Modifier = Modifier) {
    val mine = comment.userId != null && comment.userId == myUserId
    // Le service social ne renvoie que l'identifiant de l'auteur : pas de nom à afficher sauf pour soi.
    val author = stringResource(if (mine) R.string.comments_you else R.string.member_fallback)
    Row(modifier.fillMaxWidth().padding(horizontal = 16.dp, vertical = 8.dp), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
        Avatar(url = null, name = author, size = 32.dp)
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(
                "$author · ${Formats.relative(comment.createdAt, System.currentTimeMillis())}" +
                    if (comment.edited) " · " + stringResource(R.string.comments_edited) else "",
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            if (comment.deleted) {
                Text(stringResource(R.string.comments_deleted), fontStyle = FontStyle.Italic)
            } else {
                Text(comment.content.orEmpty())
                Row(verticalAlignment = Alignment.CenterVertically) {
                    TextButton(onClick = { actions.onReplyTo(comment) }) { Text(stringResource(R.string.comments_reply)) }
                    if (mine) TextButton(onClick = { actions.onDelete(comment) }) { Text(stringResource(R.string.action_delete)) }
                    OverflowMenu(items = listOf(OverflowAction(stringResource(R.string.action_report)) { actions.onReport(comment) }))
                }
            }
        }
        if (!comment.deleted) {
            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                IconButton(onClick = { actions.onLike(comment) }) {
                    Icon(
                        if (comment.likedByMe) Icons.Filled.Favorite else Icons.Filled.FavoriteBorder,
                        contentDescription = stringResource(
                            if (comment.likedByMe) R.string.action_unlike else R.string.action_like,
                            Formats.count(comment.likesCount),
                        ),
                    )
                }
                Text(Formats.count(comment.likesCount), style = MaterialTheme.typography.labelSmall)
            }
        }
    }
}
