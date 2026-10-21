package com.poro.app.ui.profile

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.core.network.ApiResult
import com.poro.app.data.auth.AccountRepository
import com.poro.app.data.auth.UserDto
import com.poro.app.data.user.ProfileDto
import com.poro.app.data.user.ProfileRepository
import com.poro.app.ui.auth.ErrorMessage
import com.poro.app.ui.auth.toErrorMessage
import com.poro.app.ui.common.Loadable
import kotlinx.coroutines.async
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** Profil (service user) et compte (service auth : email vérifié, téléphone). */
data class MyAccount(val profile: ProfileDto, val account: UserDto) {
    /** Compte email dont l'adresse n'est pas encore prouvée : proposer la vérification. */
    val needsEmailVerification: Boolean get() = account.email != null && !account.emailVerified
}

data class ProfileUiState(
    val content: Loadable<MyAccount> = Loadable.Loading,
    val becomingCreator: Boolean = false,
    val error: ErrorMessage? = null,
)

class ProfileViewModel(
    private val profiles: ProfileRepository,
    private val accounts: AccountRepository,
) : ViewModel() {

    private val _state = MutableStateFlow(ProfileUiState())
    val state: StateFlow<ProfileUiState> = _state.asStateFlow()

    init {
        refresh()
    }

    /** Recharge, par exemple au retour de la modification du profil ou de la vérification. */
    fun refresh() {
        if (_state.value.content !is Loadable.Loaded) _state.update { it.copy(content = Loadable.Loading) }
        viewModelScope.launch {
            val profile = async { profiles.me() }
            val account = async { accounts.account() }
            val p = profile.await()
            val a = account.await()
            val content = if (p is ApiResult.Success && a is ApiResult.Success) {
                Loadable.Loaded(MyAccount(p.data, a.data))
            } else {
                Loadable.Failed((if (p is ApiResult.Success) a else p).toErrorMessage()!!)
            }
            _state.update { it.copy(content = content) }
        }
    }

    fun becomeCreator() {
        val current = _state.value.content.valueOrNull ?: return
        if (_state.value.becomingCreator || current.profile.isCreator) return
        _state.update { it.copy(becomingCreator = true, error = null) }
        viewModelScope.launch {
            val result = profiles.becomeCreator()
            _state.update {
                when (result) {
                    is ApiResult.Success -> it.copy(becomingCreator = false, content = Loadable.Loaded(current.copy(profile = result.data)))
                    else -> it.copy(becomingCreator = false, error = result.toErrorMessage())
                }
            }
        }
    }
}
