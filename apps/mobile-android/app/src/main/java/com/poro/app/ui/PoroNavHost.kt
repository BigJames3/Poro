package com.poro.app.ui

import androidx.annotation.StringRes
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.wrapContentWidth
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.AddCircle
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.Person
import androidx.compose.material.icons.filled.Search
import androidx.compose.material.icons.filled.ShoppingCart
import androidx.compose.material3.Badge
import androidx.compose.material3.BadgedBox
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.NavigationRail
import androidx.compose.material3.NavigationRailItem
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.platform.LocalWindowInfo
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LifecycleEventEffect
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.navigation.NavDestination
import androidx.navigation.NavDestination.Companion.hasRoute
import androidx.navigation.NavGraph.Companion.findStartDestination
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.currentBackStackEntryAsState
import androidx.navigation.compose.rememberNavController
import com.poro.app.AppContainer
import com.poro.app.R
import com.poro.app.core.security.SessionState
import com.poro.app.ui.auth.authGraph
import com.poro.app.data.social.VideoStatsDto
import com.poro.app.ui.feed.feedGraph
import com.poro.app.ui.legal.legalGraph
import com.poro.app.ui.notification.FeedNotificationBell
import com.poro.app.ui.notification.notificationGraph
import com.poro.app.ui.profile.profileGraph
import com.poro.app.ui.search.searchGraph
import com.poro.app.ui.shop.shopGraph
import com.poro.app.ui.social.MyFollowCounts
import com.poro.app.ui.social.PrefetchVideoStats
import com.poro.app.ui.social.SocialVideoActions
import com.poro.app.ui.social.VideoActions
import com.poro.app.ui.social.socialGraph
import com.poro.app.ui.upload.MyVideos
import com.poro.app.ui.upload.uploadGraph

/** Onglets de la barre du bas, visibles sur leurs propres écrans seulement. */
enum class TopLevelTab(
    val route: Any,
    @StringRes val label: Int,
    val icon: ImageVector,
    val matches: (NavDestination) -> Boolean,
) {
    Home(HomeRoute, R.string.tab_feed, Icons.Filled.Home, { it.hasRoute<HomeRoute>() }),
    Search(SearchRoute(), R.string.tab_search, Icons.Filled.Search, { it.hasRoute<SearchRoute>() }),
    Upload(UploadRoute, R.string.tab_upload, Icons.Filled.AddCircle, { it.hasRoute<UploadRoute>() }),
    Shop(ShopHomeRoute, R.string.tab_shop, Icons.Filled.ShoppingCart, { it.hasRoute<ShopHomeRoute>() }),
    Profile(ProfileRoute, R.string.tab_profile, Icons.Filled.Person, { it.hasRoute<ProfileRoute>() }),
}

@Composable
fun PoroNavHost(container: AppContainer, snackbar: SnackbarHostState, modifier: Modifier = Modifier) {
    val session by container.tokenStore.session.collectAsStateWithLifecycle()
    if (session == SessionState.Unknown) {
        LoadingScreen(modifier)
    } else {
        PoroNavGraph(container, snackbar, startSignedIn = session == SessionState.SignedIn, modifier)
    }
}

@Composable
private fun PoroNavGraph(
    container: AppContainer,
    snackbar: SnackbarHostState,
    startSignedIn: Boolean,
    modifier: Modifier,
) {
    val navController = rememberNavController()
    // Figée à la première composition : changer de destination de départ réinitialiserait la pile.
    // Sans session : le fil public en lecture seule (mode invité).
    val start: Any = remember { if (startSignedIn) HomeRoute else GuestFeedRoute }
    val scope = rememberCoroutineScope()
    val resources = LocalContext.current.resources
    val nav = remember(navController) { AppNav(navController, container, snackbar, scope, resources) }

    // Déconnexion, session expirée, révoquée ou supprimée : retour à la connexion.
    LaunchedEffect(navController) {
        container.tokenStore.session.collect { state ->
            val destination = navController.currentDestination ?: return@collect
            if (state == SessionState.SignedOut && !destination.isSignInScreen()) nav.goToSignIn()
        }
    }
    LaunchedEffect(Unit) {
        container.sessionExpired.collect { nav.showMessage(R.string.session_expired) }
    }

    val backStack by navController.currentBackStackEntryAsState()
    val destination = backStack?.destination
    val currentTab = destination?.let { current -> TopLevelTab.entries.firstOrNull { it.matches(current) } }

    // Pastilles (non lues, panier) : relues à chaque onglet affiché et au retour dans l'app.
    val badgesVm = poroViewModel { TabBadgesViewModel(container.notificationRepository, container.orderRepository) }
    val badges by badgesVm.badges.collectAsStateWithLifecycle()
    LaunchedEffect(currentTab) { if (currentTab != null) badgesVm.refresh() }
    LifecycleEventEffect(Lifecycle.Event.ON_RESUME) { if (currentTab != null) badgesVm.refresh() }

    val onTab: (TopLevelTab) -> Unit = { tab ->
        navController.navigate(tab.route) {
            // Un onglet garde sa pile et son défilement quand on y revient.
            popUpTo(navController.graph.findStartDestination().id) { saveState = true }
            launchSingleTop = true
            restoreState = true
        }
    }
    // Tablette ou téléphone en paysage : rail à gauche plutôt que barre en bas.
    val windowWidth = with(LocalDensity.current) { LocalWindowInfo.current.containerSize.width.toDp() }
    val wide = windowWidth >= WIDE_SCREEN_DP.dp
    // Hors vidéo plein écran, le contenu garde une largeur lisible sur grand écran.
    val media = destination?.isMediaScreen() ?: true

    Scaffold(
        modifier = modifier,
        // Les barres système sont déjà gérées par le Scaffold de MainActivity.
        contentWindowInsets = WindowInsets(0),
        bottomBar = {
            if (currentTab != null && !wide) {
                NavigationBar {
                    TopLevelTab.entries.forEach { tab ->
                        NavigationBarItem(
                            selected = tab == currentTab,
                            onClick = { onTab(tab) },
                            icon = { TabIcon(tab, badges) },
                            label = { Text(stringResource(tab.label)) },
                        )
                    }
                }
            }
        },
    ) { padding ->
        Row(Modifier.fillMaxSize().padding(padding)) {
            if (currentTab != null && wide) {
                NavigationRail {
                    TopLevelTab.entries.forEach { tab ->
                        NavigationRailItem(
                            selected = tab == currentTab,
                            onClick = { onTab(tab) },
                            icon = { TabIcon(tab, badges) },
                            label = { Text(stringResource(tab.label)) },
                        )
                    }
                }
            }
            val hostModifier = if (media || !wide) {
                Modifier.weight(1f).fillMaxHeight()
            } else {
                Modifier.weight(1f).fillMaxHeight().wrapContentWidth(Alignment.CenterHorizontally).widthIn(max = MAX_CONTENT_WIDTH_DP.dp)
            }
            NavHost(navController = navController, startDestination = start, modifier = hostModifier) {
                authGraph(nav)
                feedGraph(
                    nav,
                    onOpenAuthor = { userId, username -> nav.controller.navigate(PublicProfileRoute(userId, username)) },
                    feedActions = { item ->
                        SocialVideoActions(
                            nav, item.videoId, item.title, VideoStatsDto(item.videoId, item.stats.likes, item.stats.comments, item.stats.shares),
                            ownerId = item.author.id.ifEmpty { null },
                        )
                    },
                    videoActions = { video -> SocialVideoActions(nav, video.id, video.title, VideoStatsDto(video.id), ownerId = video.userId.ifEmpty { null }) },
                    feedTopEnd = { FeedNotificationBell(nav) },
                    guestActions = { item ->
                        VideoActions(
                            VideoStatsDto(item.videoId, item.stats.likes, item.stats.comments, item.stats.shares),
                            onLike = nav::signInPrompt,
                            onComments = nav::signInPrompt,
                            onShare = nav::signInPrompt,
                        )
                    },
                    onFeedItems = { items -> PrefetchVideoStats(nav, items.map { it.videoId }) },
                )
                searchGraph(nav)
                notificationGraph(nav, onOpenOrder = { orderId, asSeller -> nav.controller.navigate(OrderRoute(orderId, asSeller)) })
                shopGraph(nav)
                legalGraph(nav)
                socialGraph(nav)
                uploadGraph(nav)
                profileGraph(nav, profileTopEnd = { FeedNotificationBell(nav, tint = MaterialTheme.colorScheme.onSurface) }) { me ->
                    MyFollowCounts(nav, me.profile.userId)
                    MyVideos(nav)
                }
            }
        }
    }
}

/** Icône d'onglet et sa pastille ; TalkBack lit le nombre avec le nom de l'onglet. */
@Composable
private fun TabIcon(tab: TopLevelTab, badges: TabBadges) {
    val count = when (tab) {
        TopLevelTab.Profile -> badges.unreadNotifications
        TopLevelTab.Shop -> badges.cartItems
        else -> 0
    }
    val description = when {
        count == 0 -> null
        tab == TopLevelTab.Profile -> quantityString(R.plurals.notifications_unread, count)
        else -> quantityString(R.plurals.cart_items, count)
    }
    BadgedBox(badge = { if (count > 0) Badge { Text(badgeText(count)) } }) {
        Icon(tab.icon, contentDescription = description)
    }
}

/** Écrans de vidéo plein écran : ni largeur bornée, ni marge. */
private fun NavDestination.isMediaScreen(): Boolean =
    hasRoute<HomeRoute>() || hasRoute<GuestFeedRoute>() || hasRoute<VideoRoute>()

/** Largeur à partir de laquelle la navigation passe en rail (Material : classe « medium »). */
private const val WIDE_SCREEN_DP = 600

/** Largeur maximale du contenu hors vidéo sur grand écran. */
private const val MAX_CONTENT_WIDTH_DP = 840

/** Pendant la lecture des jetons au démarrage (quelques dizaines de millisecondes). */
@Composable
private fun LoadingScreen(modifier: Modifier = Modifier) {
    val loading = stringResource(R.string.loading)
    Box(modifier = modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
        CircularProgressIndicator(modifier = Modifier.semantics { contentDescription = loading })
    }
}
