package com.poro.app.ui.social

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import androidx.navigation.toRoute
import com.poro.app.BuildConfig
import com.poro.app.data.moderation.ReportTargets
import com.poro.app.data.social.ShareChannel
import com.poro.app.data.social.VideoStatsDto
import com.poro.app.ui.AppNav
import com.poro.app.ui.CollectEvents
import com.poro.app.ui.PublicProfileRoute
import com.poro.app.ui.moderation.ReportHost
import com.poro.app.ui.poroViewModel

/** Profil public d'un autre compte (depuis le fil, la recherche ou une notification). */
fun NavGraphBuilder.socialGraph(nav: AppNav) {
    composable<PublicProfileRoute> { entry ->
        val route = entry.toRoute<PublicProfileRoute>()
        val vm = poroViewModel {
            PublicProfileViewModel(route.userId, route.username, nav.container.profileRepository, nav.container.socialRepository)
        }
        val state by vm.state.collectAsStateWithLifecycle()
        var showReport by rememberSaveable { mutableStateOf(false) }
        PublicProfileScreen(state = state, onBack = nav::back, onToggleFollow = vm::toggleFollow, onReport = { showReport = true })
        if (showReport) {
            ReportHost(nav, ReportTargets.USER, route.userId, onDismiss = { showReport = false })
        }
    }
}

/**
 * Actions sur une vidéo : j'aime, commentaires (feuille) et partage (WhatsApp ou autre).
 * [fallback] : compteurs connus avant la lecture des statistiques (ceux du fil).
 */
@Composable
fun SocialVideoActions(nav: AppNav, videoId: String, title: String, fallback: VideoStatsDto, modifier: Modifier = Modifier) {
    val container = nav.container
    // Un seul ViewModel par écran, partagé par toutes les vidéos du fil.
    val vm = poroViewModel { VideoInteractionsViewModel(container.socialRepository) }
    val state by vm.state.collectAsStateWithLifecycle()
    val context = LocalContext.current
    var showComments by rememberSaveable(videoId) { mutableStateOf(false) }
    var showShare by rememberSaveable(videoId) { mutableStateOf(false) }
    var showReport by rememberSaveable(videoId) { mutableStateOf(false) }
    val stats = state.stats[videoId] ?: fallback.copy(videoId = videoId)

    LaunchedEffect(videoId) { vm.onShown(videoId) }
    LaunchedEffect(state.error) {
        state.error?.let {
            nav.showMessage(it.text)
            vm.clearError()
        }
    }

    VideoActions(
        stats = stats,
        onLike = { vm.toggleLike(videoId, stats) },
        onComments = { showComments = true },
        onShare = { showShare = true },
        onReport = { showReport = true },
        modifier = modifier,
    )
    if (showComments) {
        CommentsHost(nav, videoId, onCountChanged = { delta -> vm.onCommentsChanged(videoId, delta, stats) }, onDismiss = { showComments = false })
    }
    if (showReport) {
        ReportHost(nav, ReportTargets.VIDEO, videoId, onDismiss = { showReport = false })
    }
    if (showShare) {
        ShareSheet(
            onChoose = { channel ->
                showShare = false
                val text = shareText(title, videoId, BuildConfig.SHARE_BASE_URL)
                // WhatsApp absent : le partage générique prend le relais, compté comme « autre ».
                val used = when {
                    launchShare(context, text, channel) -> channel
                    launchShare(context, text, ShareChannel.Other) -> ShareChannel.Other
                    else -> null
                }
                used?.let { vm.recordShare(videoId, it, stats) }
            },
            onDismiss = { showShare = false },
        )
    }
}

@Composable
private fun CommentsHost(nav: AppNav, videoId: String, onCountChanged: (Int) -> Unit, onDismiss: () -> Unit) {
    val container = nav.container
    val vm = poroViewModel(key = "comments-$videoId") {
        CommentsViewModel(videoId, container.socialRepository, container.profileRepository)
    }
    val state by vm.state.collectAsStateWithLifecycle()
    var reportCommentId by rememberSaveable { mutableStateOf<String?>(null) }
    CollectEvents(vm.events) { event ->
        when (event) {
            is CommentsEvent.CountChanged -> onCountChanged(event.delta)
        }
    }
    CommentsSheet(
        state = state,
        actions = CommentsActions(
            onLoadMore = vm::loadMore,
            onToggleReplies = vm::toggleReplies,
            onLoadMoreReplies = vm::loadReplies,
            onReplyTo = vm::replyTo,
            onDraftChange = vm::onDraftChange,
            onPost = vm::post,
            onLike = vm::toggleLike,
            onDelete = vm::delete,
            onReport = { comment -> reportCommentId = comment.id },
        ),
        onDismiss = onDismiss,
    )
    reportCommentId?.let { commentId ->
        ReportHost(nav, ReportTargets.COMMENT, commentId, onDismiss = { reportCommentId = null })
    }
}

/** Abonnés et abonnements de son propre compte, sur l'onglet Profil. */
@Composable
fun MyFollowCounts(nav: AppNav, userId: String) {
    val vm = poroViewModel(key = "stats-$userId") { UserStatsViewModel(userId, nav.container.socialRepository) }
    val stats by vm.stats.collectAsStateWithLifecycle()
    stats?.let { FollowCounts(it) }
}
