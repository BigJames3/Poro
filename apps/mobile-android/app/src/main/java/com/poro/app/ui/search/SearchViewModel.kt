package com.poro.app.ui.search

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.core.network.ApiResult
import com.poro.app.data.search.HashtagHit
import com.poro.app.data.search.SearchPage
import com.poro.app.data.search.SearchRepository
import com.poro.app.data.search.Suggestions
import com.poro.app.data.search.UserHit
import com.poro.app.data.search.VideoHit
import com.poro.app.ui.auth.ErrorMessage
import com.poro.app.ui.auth.toErrorMessage
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

enum class SearchTab { Videos, Users, Hashtags }

/** Résultats d'un onglet pour la requête soumise. */
data class Results<T>(
    val items: List<T> = emptyList(),
    val nextCursor: String? = null,
    val loading: Boolean = false,
    val started: Boolean = false,
    val error: ErrorMessage? = null,
) {
    val isEmpty: Boolean get() = started && !loading && items.isEmpty() && error == null
    val canLoadMore: Boolean get() = started && !loading && nextCursor != null
}

data class SearchUiState(
    val query: String = "",
    /** Requête dont les résultats sont affichés ; null tant qu'on n'a rien soumis. */
    val submitted: String? = null,
    val tab: SearchTab = SearchTab.Videos,
    val suggestions: Suggestions = Suggestions(),
    val videos: Results<VideoHit> = Results(),
    val users: Results<UserHit> = Results(),
    val hashtags: Results<HashtagHit> = Results(),
)

/**
 * Suggestions pendant la frappe (après une courte pause), puis résultats par onglet, chargés à
 * la première ouverture de l'onglet et page par page.
 */
class SearchViewModel(
    private val search: SearchRepository,
    private val savedState: SavedStateHandle,
) : ViewModel() {

    private val _state = MutableStateFlow(SearchUiState(query = savedState[KEY_QUERY] ?: ""))
    val state: StateFlow<SearchUiState> = _state.asStateFlow()

    private var suggestJob: Job? = null

    init {
        savedState.get<String>(KEY_SUBMITTED)?.let(::submit)
    }

    fun onQueryChange(value: String) {
        val query = value.take(MAX_QUERY)
        savedState[KEY_QUERY] = query
        _state.update { it.copy(query = query) }
        suggestJob?.cancel()
        val trimmed = query.trim()
        if (trimmed.isEmpty()) {
            _state.update { it.copy(suggestions = Suggestions()) }
            return
        }
        suggestJob = viewModelScope.launch {
            delay(SUGGEST_DELAY_MS)
            val result = search.suggest(trimmed)
            if (result is ApiResult.Success) _state.update { if (it.query.trim() == trimmed) it.copy(suggestions = result.data) else it }
        }
    }

    /** Lance la recherche ; une requête « @pseudo » ouvre l'onglet Comptes. */
    fun submit(query: String = _state.value.query) {
        val q = query.trim()
        if (q.isEmpty()) return
        suggestJob?.cancel()
        savedState[KEY_SUBMITTED] = q
        _state.update {
            it.copy(
                query = q,
                submitted = q,
                suggestions = Suggestions(),
                videos = Results(),
                users = Results(),
                hashtags = Results(),
                tab = if (q.startsWith("@")) SearchTab.Users else it.tab,
            )
        }
        loadMore(_state.value.tab)
    }

    fun selectTab(tab: SearchTab) {
        _state.update { it.copy(tab = tab) }
        val started = when (tab) {
            SearchTab.Videos -> _state.value.videos.started
            SearchTab.Users -> _state.value.users.started
            SearchTab.Hashtags -> _state.value.hashtags.started
        }
        if (!started) loadMore(tab)
    }

    fun loadMore(tab: SearchTab = _state.value.tab) {
        val q = _state.value.submitted ?: return
        when (tab) {
            SearchTab.Videos -> page(q, _state.value.videos, { search.videos(q, it) }) { s, r -> s.copy(videos = r) }
            SearchTab.Users -> page(q, _state.value.users, { search.users(q, it) }) { s, r -> s.copy(users = r) }
            SearchTab.Hashtags -> page(q, _state.value.hashtags, { search.hashtags(q, it) }) { s, r -> s.copy(hashtags = r) }
        }
    }

    private fun <T> page(
        query: String,
        current: Results<T>,
        fetch: suspend (String?) -> ApiResult<SearchPage<T>>,
        put: (SearchUiState, Results<T>) -> SearchUiState,
    ) {
        if (current.loading || (current.started && current.nextCursor == null)) return
        _state.update { put(it, current.copy(loading = true, started = true, error = null)) }
        viewModelScope.launch {
            val result = fetch(current.nextCursor)
            // Une autre recherche a été soumise entre-temps : cette réponse ne la concerne plus.
            if (_state.value.submitted != query) return@launch
            val next = when (result) {
                is ApiResult.Success -> current.copy(
                    items = current.items + result.data.items,
                    nextCursor = result.data.nextCursor,
                    loading = false,
                    started = true,
                )
                else -> current.copy(loading = false, started = true, error = result.toErrorMessage())
            }
            _state.update { put(it, next) }
        }
    }

    private companion object {
        const val KEY_QUERY = "query"
        const val KEY_SUBMITTED = "submitted"
        const val MAX_QUERY = 100
        const val SUGGEST_DELAY_MS = 300L
    }
}
