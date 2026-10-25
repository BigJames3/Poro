package com.poro.app.ui.feed

import androidx.compose.foundation.layout.heightIn
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import com.poro.app.R
import com.poro.app.data.feed.FeedItem
import com.poro.app.data.feed.FeedKind
import com.poro.app.data.video.VideoDto
import com.poro.app.ui.AppNav
import com.poro.app.ui.GuestFeedRoute
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
    onOpenAuthor: (userId: String, username: String?) -> Unit,
    feedActions: @Composable (FeedItem) -> Unit = {},
    videoActions: @Composable (VideoDto) -> Unit = {},
    feedTopEnd: @Composable () -> Unit = {},
    /** Vidéos du fil affiché, à chaque nouvelle page (préchargement des compteurs). */
    onFeedItems: @Composable (List<FeedItem>) -> Unit = {},
    /** Colonne d'actions du fil invité : chaque bouton ouvre la connexion. */
    guestActions: @Composable (FeedItem) -> Unit = {},
) {
    val container = nav.container

    composable<HomeRoute> {
        val vm = poroViewModel { FeedViewModel(container.feedRepository) }
        val state by vm.state.collectAsStateWithLifecycle()
        val player = rememberVideoPlayer(container.network.mediaClient)
        onFeedItems(state.current.items)
        FeedScreen(
            state = state,
            player = player,
            // Toucher l'onglet déjà choisi recharge le fil.
            onSelect = { kind -> if (kind == state.kind) vm.refresh() else vm.select(kind) },
            onItemShown = vm::onItemShown,
            onRetry = vm::retry,
            onOpenAuthor = { item -> onOpenAuthor(item.author.id, item.author.username) },
            actions = feedActions,
            topEnd = feedTopEnd,
        )
    }
    // Invité : un seul fil, public ; l'auteur, les actions et « Se connecter » ouvrent la connexion.
    composable<GuestFeedRoute> {
        val vm = poroViewModel { FeedViewModel(container.feedRepository, kinds = listOf(FeedKind.Guest)) }
        val state by vm.state.collectAsStateWithLifecycle()
        val player = rememberVideoPlayer(container.network.mediaClient)
        FeedScreen(
            state = state,
            player = player,
            onSelect = { vm.refresh() },
            onItemShown = vm::onItemShown,
            onRetry = vm::retry,
            onOpenAuthor = { nav.signInPrompt() },
            actions = guestActions,
            topEnd = {
                TextButton(onClick = nav::signInPrompt, modifier = Modifier.heightIn(min = 48.dp)) {
                    Text(stringResource(R.string.action_sign_in), color = Color.White, fontWeight = FontWeight.Bold)
                }
            },
        )
    }
    composable<VideoRoute> { entry ->
        val route = entry.toRoute<VideoRoute>()
        val vm = poroViewModel { VideoViewModel(route.videoId, container.videoRepository, container.profileRepository) }
        val state by vm.video.collectAsStateWithLifecycle()
        val author by vm.author.collectAsStateWithLifecycle()
        val player = rememberVideoPlayer(container.network.mediaClient)
        VideoScreen(
            state = state,
            player = player,
            onBack = nav::back,
            onRetry = vm::load,
            onOpenAuthor = onOpenAuthor,
            actions = videoActions,
            author = author,
        )
    }
}
