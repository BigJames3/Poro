package com.poro.app.ui

import androidx.annotation.StringRes
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.AddCircle
import androidx.compose.material.icons.filled.Home
import androidx.compose.material.icons.filled.Person
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
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
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
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
import com.poro.app.ui.profile.profileGraph
import com.poro.app.ui.social.MyFollowCounts
import com.poro.app.ui.social.SocialVideoActions
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
    Upload(UploadRoute, R.string.tab_upload, Icons.Filled.AddCircle, { it.hasRoute<UploadRoute>() }),
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
    val start: Any = remember { if (startSignedIn) HomeRoute else PhoneRoute }
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

    Scaffold(
        modifier = modifier,
        // Les barres système sont déjà gérées par le Scaffold de MainActivity.
        contentWindowInsets = WindowInsets(0),
        bottomBar = {
            if (currentTab != null) {
                NavigationBar {
                    TopLevelTab.entries.forEach { tab ->
                        NavigationBarItem(
                            selected = tab == currentTab,
                            onClick = {
                                navController.navigate(tab.route) {
                                    // Un onglet garde sa pile et son défilement quand on y revient.
                                    popUpTo(navController.graph.findStartDestination().id) { saveState = true }
                                    launchSingleTop = true
                                    restoreState = true
                                }
                            },
                            icon = { Icon(tab.icon, contentDescription = null) },
                            label = { Text(stringResource(tab.label)) },
                        )
                    }
                }
            }
        },
    ) { padding ->
        NavHost(navController = navController, startDestination = start, modifier = Modifier.padding(padding)) {
            authGraph(nav)
            feedGraph(
                nav,
                onOpenAuthor = { userId, username -> nav.controller.navigate(PublicProfileRoute(userId, username)) },
                feedActions = { item ->
                    SocialVideoActions(nav, item.videoId, item.title, VideoStatsDto(item.videoId, item.stats.likes, item.stats.comments, item.stats.shares))
                },
                videoActions = { video -> SocialVideoActions(nav, video.id, video.title, VideoStatsDto(video.id)) },
            )
            socialGraph(nav)
            uploadGraph(nav)
            profileGraph(nav) { me ->
                MyFollowCounts(nav, me.profile.userId)
                MyVideos(nav)
            }
        }
    }
}

/** Pendant la lecture des jetons au démarrage (quelques dizaines de millisecondes). */
@Composable
private fun LoadingScreen(modifier: Modifier = Modifier) {
    val loading = stringResource(R.string.loading)
    Box(modifier = modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
        CircularProgressIndicator(modifier = Modifier.semantics { contentDescription = loading })
    }
}
