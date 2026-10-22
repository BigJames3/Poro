package com.poro.app.ui.profile

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.core.network.ApiResult
import com.poro.app.data.auth.AccountRepository
import com.poro.app.data.auth.AuthRepository
import com.poro.app.data.auth.SessionDto
import com.poro.app.ui.auth.ErrorMessage
import com.poro.app.ui.auth.toErrorMessage
import com.poro.app.ui.common.Loadable
import com.poro.app.ui.common.toLoadable
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class SettingsUiState(
    val sessions: Loadable<List<SessionDto>> = Loadable.Loading,
    /** Sessions en cours de déconnexion (bouton désactivé). */
    val revoking: Set<String> = emptySet(),
    val revokingOthers: Boolean = false,
    val signingOut: Boolean = false,
    val error: ErrorMessage? = null,
)

/** Appareils connectés et déconnexion. La navigation suit la session (SignedOut → connexion). */
class SettingsViewModel(
    private val accounts: AccountRepository,
    private val auth: AuthRepository,
) : ViewModel() {

    private val _state = MutableStateFlow(SettingsUiState())
    val state: StateFlow<SettingsUiState> = _state.asStateFlow()

    init {
        loadSessions()
    }

    fun loadSessions() {
        _state.update { it.copy(sessions = Loadable.Loading) }
        viewModelScope.launch {
            val result = accounts.sessions()
            _state.update { it.copy(sessions = result.toLoadable()) }
        }
    }

    /** Déconnecte un autre appareil ; l'appareil courant passe par [logout]. */
    fun revoke(sessionId: String) {
        if (sessionId in _state.value.revoking) return
        _state.update { it.copy(revoking = it.revoking + sessionId, error = null) }
        viewModelScope.launch {
            val result = accounts.revokeSession(sessionId)
            _state.update { state ->
                val sessions = state.sessions.valueOrNull
                state.copy(
                    revoking = state.revoking - sessionId,
                    error = result.toErrorMessage(),
                    sessions = if (result is ApiResult.Success && sessions != null) {
                        Loadable.Loaded(sessions.filterNot { it.id == sessionId })
                    } else {
                        state.sessions
                    },
                )
            }
        }
    }

    fun revokeOthers() {
        if (_state.value.revokingOthers) return
        _state.update { it.copy(revokingOthers = true, error = null) }
        viewModelScope.launch {
            val result = accounts.revokeOtherSessions()
            _state.update { state ->
                val sessions = state.sessions.valueOrNull
                state.copy(
                    revokingOthers = false,
                    error = result.toErrorMessage(),
                    sessions = if (result is ApiResult.Success && sessions != null) {
                        Loadable.Loaded(sessions.filter { it.current })
                    } else {
                        state.sessions
                    },
                )
            }
        }
    }

    fun logout() {
        if (_state.value.signingOut) return
        _state.update { it.copy(signingOut = true) }
        viewModelScope.launch { auth.logout() }
    }
}
