package com.poro.app.ui.notification

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.core.network.ApiResult
import com.poro.app.data.notification.NotificationRepository
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

/** Pastille de la cloche : relue à chaque affichage de l'écran qui la porte (pas de push sans Firebase). */
class UnreadCountViewModel(private val notifications: NotificationRepository) : ViewModel() {

    private val _count = MutableStateFlow(0)
    val count: StateFlow<Int> = _count.asStateFlow()

    fun refresh() {
        viewModelScope.launch {
            (notifications.unreadCount() as? ApiResult.Success)?.let { _count.value = it.data }
        }
    }
}
