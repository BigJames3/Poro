package com.poro.app.ui.feed

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import com.poro.app.data.feed.FeedItem
import com.poro.app.data.video.VideoDto
import com.poro.app.ui.AppNav
import com.poro.app.ui.HomeRoute
import com.poro.app.ui.VideoRoute
import com.poro.app.ui.player.rememberVideoPlayer
import com.poro.app.ui.poroViewModel

/**
 * Fil (onglet d'accueil) et vidéo seule. Les actions sur une vidéo et le coin haut droit du fil
 * viennent d'autres fonctionnalités (interactions, notifications).
 */
fun NavGraphBuilder.feedGraph(
    nav: AppNav,
    onOpenAuthor: (FeedItem) -> Unit,
    feedActions: @Composable (FeedItem) -> Unit = {},
    videoActions: @Composable (VideoDto) -> Unit = {},
    feedTopEnd: @Composable () -> Unit = {},
) {
    val container = nav.container

    composable<HomeRoute> {
        val vm = poroViewModel { FeedViewModel(container.feedRepository) }
        val state by vm.state.collectAsStateWithLifecycle()
        val player = rememberVideoPlayer(container.network.mediaClient)
        FeedScreen(
            state = state,
            player = player,
            // Toucher l'onglet déjà choisi recharge le fil.
            onSelect = { kind -> if (kind == state.kind) vm.refresh() else vm.select(kind) },
            onItemShown = vm::onItemShown,
            onRetry = { vm.refresh() },
            onOpenAuthor = onOpenAuthor,
            actions = feedActions,
            topEnd = feedTopEnd,
        )
    }
    composable<VideoRoute> { entry ->
        val route = entry.toRoute<VideoRoute>()
        val vm = poroViewModel { VideoViewModel(route.videoId, container.videoRepository) }
        val state by vm.video.collectAsStateWithLifecycle()
        val player = rememberVideoPlayer(container.network.mediaClient)
        VideoScreen(state = state, player = player, onBack = nav::back, onRetry = vm::load, actions = videoActions)
    }
}
