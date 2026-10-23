package com.poro.app.ui.social

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.core.network.ApiResult
import com.poro.app.data.social.FollowEntryDto
import com.poro.app.data.social.SocialRepository
import com.poro.app.ui.auth.ErrorMessage
import com.poro.app.ui.auth.toErrorMessage
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

enum class FollowListKind { Followers, Following }

data class FollowListUiState(
    val items: List<FollowEntryDto> = emptyList(),
    val nextCursor: String? = null,
    val loading: Boolean = false,
    val started: Boolean = false,
    val error: ErrorMessage? = null,
) {
    val endReached: Boolean get() = started && !loading && nextCursor == null
}

class FollowListViewModel(
    private val userId: String,
    val kind: FollowListKind,
    private val social: SocialRepository,
) : ViewModel() {

    private val _state = MutableStateFlow(FollowListUiState())
    val state: StateFlow<FollowListUiState> = _state.asStateFlow()

    init {
        loadMore()
    }

    fun loadMore() {
        val current = _state.value
        if (current.loading || current.endReached) return
        _state.update { it.copy(loading = true, started = true, error = null) }
        viewModelScope.launch {
            val result = when (kind) {
                FollowListKind.Followers -> social.followers(userId, current.nextCursor)
                FollowListKind.Following -> social.following(userId, current.nextCursor)
            }
            when (result) {
                is ApiResult.Success -> _state.update { state ->
                    val known = state.items.mapTo(HashSet()) { it.userId }
                    state.copy(
                        items = state.items + result.data.items.filter { it.userId !in known },
                        nextCursor = result.data.nextCursor,
                        loading = false,
                    )
                }
                else -> _state.update { it.copy(loading = false, error = result.toErrorMessage()) }
            }
        }
    }
}
