package com.poro.app.ui.social

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.core.network.ApiResult
import com.poro.app.data.social.FollowEntryDto
import com.poro.app.data.social.SocialRepository
import com.poro.app.data.user.ProfileDto
import com.poro.app.data.user.ProfileRepository
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
    /** Noms et avatars lus par lots ; un compte supprimé en est absent (« Membre Poro »). */
    val profiles: Map<String, ProfileDto> = emptyMap(),
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
    private val profileRepository: ProfileRepository,
) : ViewModel() {

    private val requestedProfiles = HashSet<String>()

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
                is ApiResult.Success -> {
                    _state.update { state ->
                        val known = state.items.mapTo(HashSet()) { it.userId }
                        state.copy(
                            items = state.items + result.data.items.filter { it.userId !in known },
                            nextCursor = result.data.nextCursor,
                            loading = false,
                        )
                    }
                    resolveProfiles(result.data.items.map { it.userId })
                }
                else -> _state.update { it.copy(loading = false, error = result.toErrorMessage()) }
            }
        }
    }

    /**
     * Une requête `GET /users?ids=` par page (100 au plus, comme la page). Un échec n'est pas affiché :
     * la liste reste utilisable avec « Membre Poro », et la page suivante réessaiera ces comptes.
     */
    private suspend fun resolveProfiles(userIds: List<String>) {
        val missing = userIds.filter { requestedProfiles.add(it) }.take(MAX_IDS)
        if (missing.isEmpty()) return
        when (val result = profileRepository.profiles(missing)) {
            is ApiResult.Success -> _state.update { state -> state.copy(profiles = state.profiles + result.data.associateBy { it.userId }) }
            else -> requestedProfiles.removeAll(missing.toSet())
        }
    }

    private companion object {
        const val MAX_IDS = 100
    }
}
