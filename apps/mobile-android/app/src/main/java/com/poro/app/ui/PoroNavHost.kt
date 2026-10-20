package com.poro.app.ui

import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.SnackbarHostState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.createSavedStateHandle
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.lifecycle.viewmodel.initializer
import androidx.lifecycle.viewmodel.viewModelFactory
import androidx.navigation.NavDestination.Companion.hasRoute
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import androidx.navigation.toRoute
import com.poro.app.AppContainer
import com.poro.app.R
import com.poro.app.core.security.SessionState
import com.poro.app.ui.auth.otp.OtpEvent
import com.poro.app.ui.auth.otp.OtpVerifyScreen
import com.poro.app.ui.auth.otp.OtpVerifyViewModel
import com.poro.app.ui.auth.phone.PhoneEntryEvent
import com.poro.app.ui.auth.phone.PhoneEntryScreen
import com.poro.app.ui.auth.phone.PhoneEntryViewModel
import com.poro.app.ui.home.HomeScreen
import com.poro.app.ui.home.HomeViewModel
import kotlinx.coroutines.launch
import kotlinx.serialization.Serializable

/** Destinations de l'application (navigation typée). */
@Serializable
data object PhoneRoute

/** [phone] au format E.164 ; [expiresInSeconds] : durée de vie du code envoyé. */
@Serializable
data class OtpRoute(val phone: String, val expiresInSeconds: Int)

@Serializable
data object HomeRoute

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
    val expiredMessage = stringResource(R.string.session_expired)
    val deletionCancelledMessage = stringResource(R.string.home_deletion_cancelled)

    // Déconnexion, session expirée ou révoquée depuis un autre appareil : retour à la connexion.
    LaunchedEffect(navController) {
        container.tokenStore.session.collect { state ->
            val destination = navController.currentDestination ?: return@collect
            val onAuthScreen = destination.hasRoute<PhoneRoute>() || destination.hasRoute<OtpRoute>()
            if (state == SessionState.SignedOut && !onAuthScreen) {
                navController.navigate(PhoneRoute) { popUpTo(navController.graph.id) { inclusive = true } }
            }
        }
    }
    LaunchedEffect(Unit) {
        container.sessionExpired.collect { snackbar.showSnackbar(expiredMessage) }
    }

    NavHost(navController = navController, startDestination = start, modifier = modifier) {
        composable<PhoneRoute> {
            val vm: PhoneEntryViewModel = viewModel(
                factory = viewModelFactory {
                    initializer { PhoneEntryViewModel(container.authRepository, createSavedStateHandle(), container.clock) }
                },
            )
            val state by vm.state.collectAsStateWithLifecycle()
            LaunchedEffect(vm) {
                vm.events.collect { event ->
                    when (event) {
                        is PhoneEntryEvent.CodeSent -> navController.navigate(OtpRoute(event.phone, event.expiresInSeconds))
                    }
                }
            }
            PhoneEntryScreen(
                state = state,
                onCountryChange = vm::onCountryChange,
                onInputChange = vm::onInputChange,
                onSubmit = vm::submit,
            )
        }
        composable<OtpRoute> { entry ->
            val route = entry.toRoute<OtpRoute>()
            val vm: OtpVerifyViewModel = viewModel(
                factory = viewModelFactory {
                    initializer {
                        OtpVerifyViewModel(
                            phone = route.phone,
                            expiresInSeconds = route.expiresInSeconds,
                            auth = container.authRepository,
                            savedState = createSavedStateHandle(),
                            clock = container.clock,
                        )
                    }
                },
            )
            val state by vm.state.collectAsStateWithLifecycle()
            LaunchedEffect(vm) {
                vm.events.collect { event ->
                    when (event) {
                        is OtpEvent.SignedIn -> {
                            // L'écran de connexion quitte la pile : « retour » ferme l'application.
                            navController.navigate(HomeRoute) { popUpTo(navController.graph.id) { inclusive = true } }
                            if (event.deletionCancelled) scope.launch { snackbar.showSnackbar(deletionCancelledMessage) }
                        }
                    }
                }
            }
            OtpVerifyScreen(
                state = state,
                onCodeChange = vm::onCodeChange,
                onVerify = vm::verify,
                onResend = vm::resend,
                onChangeNumber = { navController.popBackStack() },
            )
        }
        composable<HomeRoute> {
            val vm: HomeViewModel = viewModel(
                factory = viewModelFactory {
                    initializer { HomeViewModel(container.network.baseClient, container.network.healthUrl, container.authRepository) }
                },
            )
            val status by vm.status.collectAsStateWithLifecycle()
            val signingOut by vm.signingOut.collectAsStateWithLifecycle()
            HomeScreen(status = status, signingOut = signingOut, onRetry = vm::check, onLogout = vm::logout)
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
