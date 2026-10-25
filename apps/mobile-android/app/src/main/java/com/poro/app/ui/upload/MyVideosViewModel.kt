package com.poro.app.ui.upload

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.core.network.ApiResult
import com.poro.app.data.video.VideoDto
import com.poro.app.data.video.VideoRepository
import com.poro.app.ui.auth.ErrorMessage
import com.poro.app.ui.auth.toErrorMessage
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class MyVideosUiState(
    val videos: List<VideoDto> = emptyList(),
    val nextCursor: String? = null,
    val loading: Boolean = false,
    val started: Boolean = false,
    val error: ErrorMessage? = null,
) {
    val canLoadMore: Boolean get() = started && !loading && nextCursor != null
}

/** Ses propres vidéos, tous statuts (en traitement, échouées, retirées), avec suppression. */
class MyVideosViewModel(private val videos: VideoRepository) : ViewModel() {

    private val _state = MutableStateFlow(MyVideosUiState())
    val state: StateFlow<MyVideosUiState> = _state.asStateFlow()

    init {
        refresh()
    }

    fun refresh() {
        _state.value = MyVideosUiState()
        loadMore()
    }

    /**
     * Relit la première page sans vider la grille (retour sur le profil, publication terminée) :
     * une vidéo publiée ou passée en ligne apparaît, sans clignotement.
     */
    fun reload() {
        if (_state.value.loading) return
        _state.update { it.copy(loading = true, started = true) }
        viewModelScope.launch {
            when (val result = videos.mine(null)) {
                is ApiResult.Success -> _state.value = MyVideosUiState(
                    videos = result.data.items,
                    nextCursor = result.data.nextCursor,
                    started = true,
                )
                else -> _state.update { it.copy(loading = false, error = result.toErrorMessage()) }
            }
        }
    }

    fun loadMore() {
        val current = _state.value
        // Après une erreur, la même page est redemandée (la première aussi).
        if (current.loading || (current.started && current.nextCursor == null && current.error == null)) return
        _state.update { it.copy(loading = true, started = true, error = null) }
        viewModelScope.launch {
            when (val result = videos.mine(current.nextCursor)) {
                is ApiResult.Success -> _state.update {
                    it.copy(videos = it.videos + result.data.items, nextCursor = result.data.nextCursor, loading = false)
                }
                else -> _state.update { it.copy(loading = false, error = result.toErrorMessage()) }
            }
        }
    }

    fun delete(video: VideoDto) {
        viewModelScope.launch {
            when (val result = videos.delete(video.id)) {
                is ApiResult.Success -> _state.update { state -> state.copy(videos = state.videos.filterNot { it.id == video.id }) }
                else -> _state.update { it.copy(error = result.toErrorMessage()) }
            }
        }
    }
}
