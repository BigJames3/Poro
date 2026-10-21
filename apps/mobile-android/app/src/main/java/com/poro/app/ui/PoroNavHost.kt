package com.poro.app.ui

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.lifecycle.viewmodel.initializer
import androidx.lifecycle.viewmodel.viewModelFactory
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import com.poro.app.AppContainer
import com.poro.app.ui.home.HomeScreen
import com.poro.app.ui.home.HomeViewModel
import kotlinx.serialization.Serializable

/** Destinations de l'application (navigation typée). Les écrans de connexion arrivent à l'étape 3. */
@Serializable
data object HomeRoute

@Composable
fun PoroNavHost(container: AppContainer, modifier: Modifier = Modifier) {
    val navController = rememberNavController()
    val signedIn by container.tokenStore.isSignedIn.collectAsStateWithLifecycle()

    NavHost(navController = navController, startDestination = HomeRoute, modifier = modifier) {
        composable<HomeRoute> {
            val vm: HomeViewModel = viewModel(
                factory = viewModelFactory {
                    initializer { HomeViewModel(container.network.baseClient, container.network.healthUrl) }
                },
            )
            val status by vm.status.collectAsStateWithLifecycle()
            HomeScreen(status = status, signedIn = signedIn, onRetry = vm::check)
        }
    }
}
