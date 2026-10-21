package com.poro.app.ui.social

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.core.network.ApiResult
import com.poro.app.data.social.ShareChannel
import com.poro.app.data.social.SocialRepository
import com.poro.app.data.social.VideoStatsDto
import com.poro.app.ui.auth.ErrorMessage
import com.poro.app.ui.auth.toErrorMessage
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class InteractionsUiState(
    /** Compteurs à jour et `liked_by_me`, que le fil ne donne pas, par vidéo affichée. */
    val stats: Map<String, VideoStatsDto> = emptyMap(),
    val error: ErrorMessage? = null,
)

/**
 * J'aime, partages et compteurs des vidéos d'un écran (fil ou vidéo seule). Un j'aime s'affiche
 * tout de suite ; la réponse du serveur fait foi, un échec remet l'état précédent.
 */
class VideoInteractionsViewModel(private val social: SocialRepository) : ViewModel() {

    private val _state = MutableStateFlow(InteractionsUiState())
    val state: StateFlow<InteractionsUiState> = _state.asStateFlow()

    private val requested = HashSet<String>()
    private val liking = HashSet<String>()

    /** Vidéo affichée : on lit ses compteurs une fois. */
    fun onShown(videoId: String) {
        if (!requested.add(videoId)) return
        viewModelScope.launch {
            val result = social.videoStats(videoId)
            if (result is ApiResult.Success) put(result.data.copy(videoId = videoId))
            else requested.remove(videoId)
        }
    }

    fun toggleLike(videoId: String, fallback: VideoStatsDto) {
        if (!liking.add(videoId)) return
        val before = _state.value.stats[videoId] ?: fallback
        val liked = !before.likedByMe
        put(before.copy(likedByMe = liked, likes = (before.likes + if (liked) 1 else -1).coerceAtLeast(0)))
        viewModelScope.launch {
            val result = social.setLiked(videoId, liked)
            if (result is ApiResult.Success) put(result.data.copy(videoId = videoId)) else put(before)
            _state.update { it.copy(error = result.toErrorMessage()) }
            liking.remove(videoId)
        }
    }

    /** Le partage est compté après que l'utilisateur a choisi une application. */
    fun recordShare(videoId: String, channel: ShareChannel, fallback: VideoStatsDto) {
        viewModelScope.launch {
            val result = social.share(videoId, channel)
            if (result is ApiResult.Success) {
                val current = _state.value.stats[videoId] ?: fallback
                put(current.copy(shares = result.data.sharesCount))
            }
        }
    }

    /** Commentaire ajouté (+1) ou supprimé (-1) depuis la feuille des commentaires. */
    fun onCommentsChanged(videoId: String, delta: Int, fallback: VideoStatsDto) {
        val current = _state.value.stats[videoId] ?: fallback
        put(current.copy(comments = (current.comments + delta).coerceAtLeast(0)))
    }

    fun clearError() = _state.update { it.copy(error = null) }

    private fun put(stats: VideoStatsDto) = _state.update { it.copy(stats = it.stats + (stats.videoId to stats)) }
}
