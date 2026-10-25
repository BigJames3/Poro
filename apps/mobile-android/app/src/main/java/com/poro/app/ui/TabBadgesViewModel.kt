package com.poro.app.ui

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.core.network.ApiResult
import com.poro.app.data.notification.NotificationRepository
import com.poro.app.data.order.OrderRepository
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

/** Pastilles des onglets : notifications non lues (Profil) et articles du panier (Boutique). */
data class TabBadges(val unreadNotifications: Int = 0, val cartItems: Int = 0)

/**
 * Relues à chaque retour sur un onglet et au retour dans l'app (pas de push sans Firebase). Un
 * échec garde la dernière valeur connue : une pastille ne clignote pas sur un réseau instable.
 */
class TabBadgesViewModel(
    private val notifications: NotificationRepository,
    private val orders: OrderRepository,
) : ViewModel() {

    private val _badges = MutableStateFlow(TabBadges())
    val badges: StateFlow<TabBadges> = _badges.asStateFlow()

    fun refresh() {
        viewModelScope.launch {
            (notifications.unreadCount() as? ApiResult.Success)?.let { result -> _badges.update { it.copy(unreadNotifications = result.data) } }
        }
        viewModelScope.launch {
            (orders.cart() as? ApiResult.Success)?.let { result -> _badges.update { it.copy(cartItems = result.data.itemCount) } }
        }
    }
}

/** Texte d'une pastille : au-delà de 99, « 99+ ». */
fun badgeText(count: Int): String = if (count > 99) "99+" else count.toString()
