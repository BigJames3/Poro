package com.poro.app.ui.social

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.data.social.SocialRepository
import com.poro.app.data.social.UserStatsDto
import com.poro.app.data.user.ProfileDto
import com.poro.app.data.user.ProfileRepository
import com.poro.app.ui.auth.ErrorMessage
import com.poro.app.ui.auth.toErrorMessage
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class PublicProfileUiState(
    /** Null tant que le profil n'est pas chargé, ou si l'auteur n'a pas encore de pseudo. */
    val profile: ProfileDto? = null,
    val stats: UserStatsDto? = null,
    val loading: Boolean = true,
    val following: Boolean = false,
    val followBusy: Boolean = false,
    /** Son propre compte : pas de bouton « S'abonner ». */
    val isMe: Boolean = false,
    val error: ErrorMessage? = null,
)

/**
 * Profil d'un autre compte. Le service user ne trouve un profil que par pseudo : un auteur sans
 * pseudo n'affiche que ses compteurs et l'abonnement (par identifiant).
 */
class PublicProfileViewModel(
    private val userId: String,
    private val username: String?,
    private val profiles: ProfileRepository,
    private val social: SocialRepository,
) : ViewModel() {

    private val _state = MutableStateFlow(PublicProfileUiState())
    val state: StateFlow<PublicProfileUiState> = _state.asStateFlow()

    init {
        load()
    }

    fun load() {
        _state.update { it.copy(loading = true, error = null) }
        viewModelScope.launch {
            val profile = username?.let { profiles.byUsername(it) }
            val stats = social.userStats(userId)
            val me = profiles.me()
            _state.update {
                it.copy(
                    loading = false,
                    profile = (profile as? ApiResult.Success)?.data,
                    stats = (stats as? ApiResult.Success)?.data,
                    following = (stats as? ApiResult.Success)?.data?.followedByMe ?: false,
                    isMe = (me as? ApiResult.Success)?.data?.userId == userId,
                    error = stats.toErrorMessage(),
                )
            }
        }
    }

    fun toggleFollow() {
        val current = _state.value
        if (current.followBusy || current.isMe) return
        val follow = !current.following
        _state.update { it.copy(following = follow, followBusy = true, error = null) }
        viewModelScope.launch {
            when (val result = social.setFollowing(userId, follow)) {
                is ApiResult.Success -> _state.update {
                    it.copy(stats = result.data, following = result.data.followedByMe, followBusy = false)
                }
                is ApiResult.Failure -> _state.update {
                    it.copy(
                        following = !follow,
                        followBusy = false,
                        isMe = it.isMe || result.code == ApiErrorCodes.CANNOT_FOLLOW_SELF,
                        error = result.toErrorMessage().takeUnless { result.code == ApiErrorCodes.CANNOT_FOLLOW_SELF },
                    )
                }
                is ApiResult.NetworkError -> _state.update { it.copy(following = !follow, followBusy = false, error = result.toErrorMessage()) }
            }
        }
    }
}
