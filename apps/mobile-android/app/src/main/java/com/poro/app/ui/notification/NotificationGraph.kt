package com.poro.app.ui.notification

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.graphics.Color
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LifecycleEventEffect
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import com.poro.app.ui.AppNav
import com.poro.app.ui.NotificationPreferencesRoute
import com.poro.app.ui.NotificationsRoute
import com.poro.app.ui.PublicProfileRoute
import com.poro.app.ui.VideoRoute
import com.poro.app.ui.poroViewModel

/** Liste des notifications et préférences. [onOpenOrder] : commande (acheteur ou vendeur). */
fun NavGraphBuilder.notificationGraph(nav: AppNav, onOpenOrder: (orderId: String, asSeller: Boolean) -> Unit) {
    composable<NotificationsRoute> {
        val vm = poroViewModel { NotificationsViewModel(nav.container.notificationRepository) }
        val state by vm.state.collectAsStateWithLifecycle()
        NotificationsScreen(
            state = state,
            onBack = nav::back,
            onOpen = { notification ->
                vm.open(notification)
                when (val target = notification.target()) {
                    is NotificationTarget.Video -> nav.controller.navigate(VideoRoute(target.videoId))
                    is NotificationTarget.Profile -> nav.controller.navigate(PublicProfileRoute(target.userId, target.username))
                    is NotificationTarget.Order -> onOpenOrder(target.orderId, target.asSeller)
                    NotificationTarget.None -> Unit
                }
            },
            onReadAll = vm::readAll,
            onLoadMore = vm::loadMore,
            onRetry = vm::retry,
            onRefresh = vm::reload,
        )
    }
    composable<NotificationPreferencesRoute> {
        val vm = poroViewModel { NotificationPreferencesViewModel(nav.container.notificationRepository) }
        val state by vm.state.collectAsStateWithLifecycle()
        NotificationPreferencesScreen(state = state, onBack = nav::back, onRetry = vm::load, onChange = vm::update)
    }
}

/** Cloche du fil : le compteur est relu chaque fois que l'écran revient au premier plan. */
@Composable
fun FeedNotificationBell(nav: AppNav, tint: Color = Color.White) {
    val vm = poroViewModel { UnreadCountViewModel(nav.container.notificationRepository) }
    val count by vm.count.collectAsStateWithLifecycle()
    LifecycleEventEffect(Lifecycle.Event.ON_RESUME) { vm.refresh() }
    NotificationBell(count = count, onClick = { nav.controller.navigate(NotificationsRoute) }, tint = tint)
}
