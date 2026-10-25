package com.poro.app.ui.notification

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.core.network.ApiResult
import com.poro.app.data.notification.NotificationDto
import com.poro.app.data.notification.NotificationRepository
import com.poro.app.ui.auth.ErrorMessage
import com.poro.app.ui.auth.toErrorMessage
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class NotificationsUiState(
    val items: List<NotificationDto> = emptyList(),
    val nextCursor: String? = null,
    val loading: Boolean = false,
    val started: Boolean = false,
    val error: ErrorMessage? = null,
    /** Rechargement demandé en tirant la liste : la liste affichée reste jusqu'à la réponse. */
    val refreshing: Boolean = false,
) {
    val isEmpty: Boolean get() = started && !loading && items.isEmpty() && error == null
    val canLoadMore: Boolean get() = started && !loading && nextCursor != null
    val hasUnread: Boolean get() = items.any { !it.read }
}

class NotificationsViewModel(private val notifications: NotificationRepository) : ViewModel() {

    private val _state = MutableStateFlow(NotificationsUiState())
    val state: StateFlow<NotificationsUiState> = _state.asStateFlow()

    init {
        loadMore()
    }

    fun refresh() {
        _state.value = NotificationsUiState()
        loadMore()
    }

    /** Tirer pour rafraîchir : relit la première page et remplace la liste, sans la vider avant. */
    fun reload() {
        val current = _state.value
        if (current.loading || current.refreshing) return
        _state.update { it.copy(refreshing = true) }
        viewModelScope.launch {
            when (val result = notifications.page(null)) {
                is ApiResult.Success -> _state.value = NotificationsUiState(
                    items = result.data.items,
                    nextCursor = result.data.nextCursor,
                    started = true,
                )
                else -> _state.update { it.copy(refreshing = false, error = result.toErrorMessage()) }
            }
        }
    }

    /** Relance après une erreur : la page qui a échoué, ou la première (sans curseur). */
    fun retry() {
        val current = _state.value
        if (current.error == null || current.loading) return
        if (current.items.isEmpty()) refresh() else loadMore()
    }

    fun loadMore() {
        val current = _state.value
        if (current.loading || (current.started && current.nextCursor == null)) return
        _state.update { it.copy(loading = true, started = true, error = null) }
        viewModelScope.launch {
            when (val result = notifications.page(current.nextCursor)) {
                is ApiResult.Success -> _state.update { state ->
                    // Une notification qui remonte pendant la lecture peut revenir une fois : on l'écarte.
                    val known = state.items.mapTo(HashSet()) { it.id }
                    state.copy(
                        items = state.items + result.data.items.filter { it.id !in known },
                        nextCursor = result.data.nextCursor,
                        loading = false,
                    )
                }
                else -> _state.update { it.copy(loading = false, error = result.toErrorMessage()) }
            }
        }
    }

    /** Touchée : marquée lue (sans attendre le serveur) ; l'écran ouvre sa cible. */
    fun open(notification: NotificationDto) {
        if (notification.read) return
        setRead { it.id == notification.id }
        viewModelScope.launch { notifications.markRead(notification.id) }
    }

    fun readAll() {
        if (!_state.value.hasUnread) return
        setRead { true }
        viewModelScope.launch {
            val result = notifications.readAll()
            if (result !is ApiResult.Success) _state.update { it.copy(error = result.toErrorMessage()) }
        }
    }

    private fun setRead(predicate: (NotificationDto) -> Boolean) = _state.update { state ->
        state.copy(items = state.items.map { if (predicate(it)) it.copy(read = true) else it })
    }
}
