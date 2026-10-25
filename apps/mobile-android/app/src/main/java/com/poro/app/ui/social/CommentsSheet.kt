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
import com.poro.app.ui.common.EmptyState
import com.poro.app.ui.common.displayNameOf
import com.poro.app.ui.theme.PoroSpacing

/** Actions de la feuille des commentaires, transmises par l'écran qui l'ouvre. */
class CommentsActions(
    val onLoadMore: () -> Unit,
    val onToggleReplies: (String) -> Unit,
    val onLoadMoreReplies: (String) -> Unit,
    val onReplyTo: (CommentDto?) -> Unit,
    val onDraftChange: (String) -> Unit,
    val onPost: () -> Unit,
    val onLike: (CommentDto) -> Unit,
    val onEdit: (CommentDto) -> Unit,
    val onEditDraftChange: (String) -> Unit,
    val onSaveEdit: () -> Unit,
    val onCancelEdit: () -> Unit,
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
                modifier = Modifier.padding(horizontal = PoroSpacing.L, vertical = PoroSpacing.S).semantics { heading() },
            )
            LazyColumn(Modifier.weight(1f)) {
                if (state.endReached && state.comments.isEmpty()) {
                    item { EmptyState(stringResource(R.string.comments_empty)) }
                }
                items(state.comments, key = { it.id }) { comment ->
                    CommentRow(comment, state, actions)
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
                        replies.items.forEach { reply -> CommentRow(reply, state, actions, Modifier.padding(start = 40.dp)) }
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
                        CircularProgressIndicator(Modifier.padding(PoroSpacing.L))
                    }
                }
            }
            state.error?.let { ErrorMessageText(it, Modifier.padding(horizontal = PoroSpacing.L)) }
            state.replyingTo?.let {
                Row(Modifier.padding(horizontal = PoroSpacing.L), verticalAlignment = Alignment.CenterVertically) {
                    Text(stringResource(R.string.comments_replying), style = MaterialTheme.typography.bodySmall, modifier = Modifier.weight(1f))
                    TextButton(onClick = { actions.onReplyTo(null) }) { Text(stringResource(R.string.action_cancel)) }
                }
            }
            Row(Modifier.fillMaxWidth().padding(PoroSpacing.S), verticalAlignment = Alignment.CenterVertically) {
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
private fun CommentRow(
    comment: CommentDto,
    state: CommentsUiState,
    actions: CommentsActions,
    modifier: Modifier = Modifier,
) {
    val myUserId = state.myUserId
    val authors = state.authors
    val mine = comment.userId != null && comment.userId == myUserId
    val editingThis = state.editing?.id == comment.id
    // Profil lu par lot ; avant sa réponse, ou pour un compte supprimé : « Membre Poro ».
    val profile = comment.userId?.let { authors[it] }
    val author = when {
        mine -> stringResource(R.string.comments_you)
        profile != null && (profile.displayName != null || profile.username != null) -> displayNameOf(profile.displayName, profile.username)
        else -> stringResource(R.string.member_fallback)
    }
    Row(modifier.fillMaxWidth().padding(horizontal = PoroSpacing.L, vertical = PoroSpacing.S), horizontalArrangement = Arrangement.spacedBy(PoroSpacing.M)) {
        Avatar(url = profile?.avatarUrl, name = author, size = 32.dp)
        Column(Modifier.weight(1f), verticalArrangement = Arrangement.spacedBy(2.dp)) {
            Text(
                "$author · ${Formats.relative(comment.createdAt, System.currentTimeMillis())}" +
                    if (comment.edited) " · " + stringResource(R.string.comments_edited) else "",
                style = MaterialTheme.typography.labelMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            if (comment.deleted) {
                Text(stringResource(R.string.comments_deleted), fontStyle = FontStyle.Italic)
            } else if (editingThis) {
                OutlinedTextField(
                    value = state.editDraft,
                    onValueChange = actions.onEditDraftChange,
                    label = { Text(stringResource(R.string.comments_edit)) },
                    enabled = !state.posting,
                    maxLines = 4,
                    modifier = Modifier.fillMaxWidth(),
                )
                Row {
                    TextButton(onClick = actions.onSaveEdit, enabled = state.canSaveEdit) { Text(stringResource(R.string.comments_save)) }
                    TextButton(onClick = actions.onCancelEdit, enabled = !state.posting) { Text(stringResource(R.string.action_cancel)) }
                }
            } else {
                Text(comment.content.orEmpty())
                Row(verticalAlignment = Alignment.CenterVertically) {
                    TextButton(onClick = { actions.onReplyTo(comment) }) { Text(stringResource(R.string.comments_reply)) }
                    // Proposé dans les 15 minutes selon l'horloge du téléphone ; le serveur a le dernier mot.
                    if (mine && state.editing == null && CommentRules.canEdit(comment, myUserId, System.currentTimeMillis())) {
                        TextButton(onClick = { actions.onEdit(comment) }) { Text(stringResource(R.string.comments_edit)) }
                    }
                    if (mine) {
                        TextButton(onClick = { actions.onDelete(comment) }) { Text(stringResource(R.string.action_delete)) }
                    } else if (myUserId != null) {
                        // Avant de connaître son compte, on ne sait pas si le commentaire est le sien.
                        TextButton(onClick = { actions.onReport(comment) }) { Text(stringResource(R.string.report_action)) }
                    }
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
