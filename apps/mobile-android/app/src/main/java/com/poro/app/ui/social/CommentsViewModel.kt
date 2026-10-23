package com.poro.app.ui.social

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.core.network.ApiResult
import com.poro.app.data.social.CommentDto
import com.poro.app.data.social.SocialRepository
import com.poro.app.data.user.ProfileRepository
import com.poro.app.ui.auth.ErrorMessage
import com.poro.app.ui.auth.toErrorMessage
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.receiveAsFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** Réponses d'un commentaire, chargées quand on les déplie. */
data class Replies(
    val items: List<CommentDto> = emptyList(),
    val nextCursor: String? = null,
    val loading: Boolean = false,
    val expanded: Boolean = false,
)

data class CommentsUiState(
    val comments: List<CommentDto> = emptyList(),
    val nextCursor: String? = null,
    val loading: Boolean = false,
    val started: Boolean = false,
    val replies: Map<String, Replies> = emptyMap(),
    val draft: String = "",
    /** Commentaire de premier niveau auquel on répond, sinon nouveau commentaire. */
    val replyingTo: CommentDto? = null,
    /** Commentaire en cours d'édition (fenêtre de 15 minutes). */
    val editing: CommentDto? = null,
    val editDraft: String = "",
    val posting: Boolean = false,
    /** Identifiant du compte connecté : ses commentaires peuvent être supprimés. */
    val myUserId: String? = null,
    val error: ErrorMessage? = null,
) {
    val canPost: Boolean get() = editing == null && draft.isNotBlank() && draft.trim().length <= MAX_LENGTH && !posting
    val canSaveEdit: Boolean get() = editing != null && editDraft.isNotBlank() && editDraft.trim().length <= MAX_LENGTH && !posting
    val endReached: Boolean get() = started && !loading && nextCursor == null

    companion object {
        const val MAX_LENGTH = 1000
    }
}

sealed interface CommentsEvent {
    /** Le nombre de commentaires visibles a changé (+1 publié, -1 supprimé). */
    data class CountChanged(val delta: Int) : CommentsEvent
}

class CommentsViewModel(
    private val videoId: String,
    private val social: SocialRepository,
    private val profiles: ProfileRepository,
) : ViewModel() {

    private val _state = MutableStateFlow(CommentsUiState())
    val state: StateFlow<CommentsUiState> = _state.asStateFlow()

    private val _events = Channel<CommentsEvent>(Channel.BUFFERED)
    val events: Flow<CommentsEvent> = _events.receiveAsFlow()

    init {
        loadMore()
        viewModelScope.launch {
            (profiles.me() as? ApiResult.Success)?.let { me -> _state.update { it.copy(myUserId = me.data.userId) } }
        }
    }

    fun loadMore() {
        val current = _state.value
        if (current.loading || current.endReached) return
        _state.update { it.copy(loading = true, started = true, error = null) }
        viewModelScope.launch {
            when (val result = social.comments(videoId, current.nextCursor)) {
                is ApiResult.Success -> _state.update { state ->
                    val known = state.comments.mapTo(HashSet()) { it.id }
                    state.copy(
                        comments = state.comments + result.data.items.filter { it.id !in known },
                        nextCursor = result.data.nextCursor,
                        loading = false,
                    )
                }
                else -> _state.update { it.copy(loading = false, error = result.toErrorMessage()) }
            }
        }
    }

    /** Déplie (et charge) ou replie les réponses d'un commentaire. */
    fun toggleReplies(commentId: String) {
        val replies = _state.value.replies[commentId] ?: Replies()
        if (replies.expanded) {
            putReplies(commentId, replies.copy(expanded = false))
        } else if (replies.items.isEmpty()) {
            loadReplies(commentId)
        } else {
            putReplies(commentId, replies.copy(expanded = true))
        }
    }

    fun loadReplies(commentId: String) {
        val replies = _state.value.replies[commentId] ?: Replies()
        if (replies.loading) return
        putReplies(commentId, replies.copy(loading = true, expanded = true))
        viewModelScope.launch {
            val result = social.replies(commentId, replies.nextCursor)
            val current = _state.value.replies[commentId] ?: Replies()
            putReplies(
                commentId,
                if (result is ApiResult.Success) {
                    current.copy(items = current.items + result.data.items, nextCursor = result.data.nextCursor, loading = false)
                } else {
                    current.copy(loading = false)
                },
            )
            if (result !is ApiResult.Success) _state.update { it.copy(error = result.toErrorMessage()) }
        }
    }

    fun onDraftChange(value: String) = _state.update { it.copy(draft = value.take(CommentsUiState.MAX_LENGTH), error = null) }

    fun onEditDraftChange(value: String) = _state.update { it.copy(editDraft = value.take(CommentsUiState.MAX_LENGTH), error = null) }

    /** Répondre à [comment] ; une réponse à une réponse va à son commentaire parent (un seul niveau). */
    fun replyTo(comment: CommentDto?) = _state.update { it.copy(replyingTo = comment, editing = null) }

    fun startEdit(comment: CommentDto, nowMs: Long = System.currentTimeMillis()) {
        if (!CommentRules.canEdit(comment, _state.value.myUserId, nowMs)) return
        _state.update { it.copy(editing = comment, editDraft = comment.content.orEmpty(), replyingTo = null, error = null) }
    }

    fun cancelEdit() = _state.update { it.copy(editing = null, editDraft = "") }

    fun saveEdit() {
        val current = _state.value
        val editing = current.editing ?: return
        if (!current.canSaveEdit) return
        _state.update { it.copy(posting = true, error = null) }
        viewModelScope.launch {
            when (val result = social.editComment(editing.id, current.editDraft)) {
                is ApiResult.Success -> {
                    replace(result.data)
                    _state.update { it.copy(posting = false, editing = null, editDraft = "") }
                }
                else -> _state.update { it.copy(posting = false, error = result.toErrorMessage()) }
            }
        }
    }

    fun post() {
        val current = _state.value
        if (!current.canPost) return
        val parentId = current.replyingTo?.let { it.parentId ?: it.id }
        _state.update { it.copy(posting = true, error = null) }
        viewModelScope.launch {
            when (val result = social.postComment(videoId, current.draft, parentId)) {
                is ApiResult.Success -> {
                    val comment = result.data
                    _state.update { state ->
                        if (parentId == null) {
                            state.copy(comments = listOf(comment) + state.comments, draft = "", posting = false)
                        } else {
                            val replies = state.replies[parentId] ?: Replies()
                            state.copy(
                                replies = state.replies + (parentId to replies.copy(items = replies.items + comment, expanded = true)),
                                comments = state.comments.map { if (it.id == parentId) it.copy(repliesCount = it.repliesCount + 1) else it },
                                draft = "",
                                replyingTo = null,
                                posting = false,
                            )
                        }
                    }
                    _events.send(CommentsEvent.CountChanged(+1))
                }
                else -> _state.update { it.copy(posting = false, error = result.toErrorMessage()) }
            }
        }
    }

    fun toggleLike(comment: CommentDto) {
        val liked = !comment.likedByMe
        val optimistic = comment.copy(likedByMe = liked, likesCount = (comment.likesCount + if (liked) 1 else -1).coerceAtLeast(0))
        replace(optimistic)
        viewModelScope.launch {
            when (val result = social.setCommentLiked(comment.id, liked)) {
                is ApiResult.Success -> replace(optimistic.copy(likesCount = result.data.likesCount, likedByMe = result.data.likedByMe))
                else -> {
                    replace(comment)
                    _state.update { it.copy(error = result.toErrorMessage()) }
                }
            }
        }
    }

    fun delete(comment: CommentDto) {
        viewModelScope.launch {
            when (val result = social.deleteComment(comment.id)) {
                is ApiResult.Success -> {
                    _state.update { state ->
                        // Un commentaire avec des réponses reste affiché, vidé ; sinon il disparaît.
                        val keepShell = comment.parentId == null && comment.repliesCount > 0
                        val tombstone = comment.copy(deleted = true, content = null, userId = null)
                        state.copy(
                            comments = if (keepShell) {
                                state.comments.map { if (it.id == comment.id) tombstone else it }
                            } else {
                                state.comments.filterNot { it.id == comment.id }
                                    .map { if (it.id == comment.parentId) it.copy(repliesCount = (it.repliesCount - 1).coerceAtLeast(0)) else it }
                            },
                            replies = state.replies.mapValues { (_, r) -> r.copy(items = r.items.filterNot { it.id == comment.id }) },
                        )
                    }
                    _events.send(CommentsEvent.CountChanged(-1))
                }
                else -> _state.update { it.copy(error = result.toErrorMessage()) }
            }
        }
    }

    private fun replace(comment: CommentDto) = _state.update { state ->
        state.copy(
            comments = state.comments.map { if (it.id == comment.id) comment else it },
            replies = state.replies.mapValues { (_, r) -> r.copy(items = r.items.map { if (it.id == comment.id) comment else it }) },
        )
    }

    private fun putReplies(commentId: String, replies: Replies) =
        _state.update { it.copy(replies = it.replies + (commentId to replies)) }
}
