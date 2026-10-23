package com.poro.app.ui.feed

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.data.feed.FeedItem
import com.poro.app.data.feed.FeedKind
import com.poro.app.data.feed.FeedRepository
import com.poro.app.data.feed.SIGNED_IN_FEEDS
import com.poro.app.ui.auth.ErrorMessage
import com.poro.app.ui.auth.toErrorMessage
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** Un fil chargé page par page. */
data class FeedList(
    val items: List<FeedItem> = emptyList(),
    val nextCursor: String? = null,
    val loading: Boolean = false,
    /** Au moins une page a été demandée (distingue « vide » de « pas encore chargé »). */
    val started: Boolean = false,
    val endReached: Boolean = false,
    val error: ErrorMessage? = null,
) {
    val isEmpty: Boolean get() = started && !loading && items.isEmpty() && error == null
}

data class FeedUiState(
    /** Onglets affichés, dans l'ordre. */
    val kinds: List<FeedKind> = SIGNED_IN_FEEDS,
    val kind: FeedKind = kinds.first(),
    val lists: Map<FeedKind, FeedList> = FeedKind.entries.associateWith { FeedList() },
) {
    val current: FeedList get() = lists.getValue(kind)
}

/**
 * Les fils [kinds], chacun avec sa propre pagination : Pour toi, Abonnements et Tendances pour un
 * compte connecté, le seul fil public pour un invité.
 */
class FeedViewModel(private val feed: FeedRepository, kinds: List<FeedKind> = SIGNED_IN_FEEDS) : ViewModel() {

    private val _state = MutableStateFlow(FeedUiState(kinds = kinds))
    val state: StateFlow<FeedUiState> = _state.asStateFlow()

    init {
        loadMore(kinds.first())
    }

    fun select(kind: FeedKind) {
        _state.update { it.copy(kind = kind) }
        if (!_state.value.lists.getValue(kind).started) loadMore(kind)
    }

    /** La vidéo [index] du fil courant est affichée : on charge la suite un peu avant la fin. */
    fun onItemShown(index: Int) {
        val list = _state.value.current
        if (index >= list.items.size - PREFETCH_DISTANCE) loadMore(_state.value.kind)
    }

    /** Recharge depuis la première page (tirer pour rafraîchir, erreur). */
    fun refresh(kind: FeedKind = _state.value.kind) {
        updateList(kind) { FeedList() }
        loadMore(kind)
    }

    fun loadMore(kind: FeedKind) {
        val list = _state.value.lists.getValue(kind)
        if (list.loading || list.endReached) return
        updateList(kind) { it.copy(loading = true, started = true, error = null) }
        viewModelScope.launch {
            when (val result = feed.page(kind, list.nextCursor)) {
                is ApiResult.Success -> updateList(kind) { current ->
                    // Une session « Pour toi » expirée repart de zéro côté serveur : on écarte les doublons.
                    val known = current.items.mapTo(HashSet()) { it.videoId }
                    current.copy(
                        items = current.items + result.data.items.filter { it.videoId !in known },
                        nextCursor = result.data.nextCursor,
                        endReached = result.data.nextCursor == null,
                        loading = false,
                    )
                }
                // Curseur périmé : on repart de la première page (une seule fois, sans curseur).
                is ApiResult.Failure -> if (result.code == ApiErrorCodes.INVALID_CURSOR && list.nextCursor != null) {
                    refresh(kind)
                } else {
                    updateList(kind) { it.copy(loading = false, error = result.toErrorMessage()) }
                }
                is ApiResult.NetworkError -> updateList(kind) { it.copy(loading = false, error = result.toErrorMessage()) }
            }
        }
    }

    private fun updateList(kind: FeedKind, transform: (FeedList) -> FeedList) {
        _state.update { it.copy(lists = it.lists + (kind to transform(it.lists.getValue(kind)))) }
    }

    private companion object {
        const val PREFETCH_DISTANCE = 3
    }
}
