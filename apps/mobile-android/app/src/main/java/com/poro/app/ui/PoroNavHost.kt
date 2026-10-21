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
import androidx.navigation.NavDestination
import androidx.navigation.NavDestination.Companion.hasRoute
import androidx.navigation.NavHostController
import androidx.navigation.compose.NavHost
import androidx.navigation.compose.composable
import androidx.navigation.compose.rememberNavController
import androidx.navigation.toRoute
import com.poro.app.AppContainer
import com.poro.app.R
import com.poro.app.core.security.SessionState
import com.poro.app.ui.auth.email.EmailLoginEvent
import com.poro.app.ui.auth.email.EmailLoginScreen
import com.poro.app.ui.auth.email.EmailLoginViewModel
import com.poro.app.ui.auth.email.EmailRegisterEvent
import com.poro.app.ui.auth.email.EmailRegisterScreen
import com.poro.app.ui.auth.email.EmailRegisterViewModel
import com.poro.app.ui.auth.email.EmailVerifyEvent
import com.poro.app.ui.auth.email.EmailVerifyScreen
import com.poro.app.ui.auth.email.EmailVerifyViewModel
import com.poro.app.ui.auth.email.ForgotPasswordEvent
import com.poro.app.ui.auth.email.ForgotPasswordScreen
import com.poro.app.ui.auth.email.ForgotPasswordViewModel
import com.poro.app.ui.auth.email.ResetPasswordEvent
import com.poro.app.ui.auth.email.ResetPasswordScreen
import com.poro.app.ui.auth.email.ResetPasswordViewModel
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

@Serializable
data object EmailLoginRoute

@Serializable
data object EmailRegisterRoute

/** Après l'inscription, session ouverte : vérification de l'adresse, que l'on peut remettre à plus tard. */
@Serializable
data object EmailVerifyRoute

/** [email] : adresse déjà saisie sur l'écran de connexion, reprise pour éviter de la retaper. */
@Serializable
data class ForgotPasswordRoute(val email: String = "")

@Serializable
data class ResetPasswordRoute(val email: String)

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
    val emailVerifiedMessage = stringResource(R.string.email_verified)
    val passwordChangedMessage = stringResource(R.string.password_changed)

    // Déconnexion, session expirée ou révoquée depuis un autre appareil : retour à la connexion.
    LaunchedEffect(navController) {
        container.tokenStore.session.collect { state ->
            val destination = navController.currentDestination ?: return@collect
            if (state == SessionState.SignedOut && !destination.isSignInScreen()) {
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
                onUseEmail = { navController.navigate(EmailLoginRoute) },
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
                            navController.goHome()
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
        composable<EmailLoginRoute> {
            val vm: EmailLoginViewModel = viewModel(
                factory = viewModelFactory {
                    initializer { EmailLoginViewModel(container.authRepository, createSavedStateHandle(), container.clock) }
                },
            )
            val state by vm.state.collectAsStateWithLifecycle()
            LaunchedEffect(vm) {
                vm.events.collect { event ->
                    when (event) {
                        is EmailLoginEvent.SignedIn -> {
                            navController.goHome()
                            if (event.deletionCancelled) scope.launch { snackbar.showSnackbar(deletionCancelledMessage) }
                        }
                    }
                }
            }
            EmailLoginScreen(
                state = state,
                onEmailChange = vm::onEmailChange,
                onPasswordChange = vm::onPasswordChange,
                onSubmit = vm::submit,
                onForgotPassword = { navController.navigate(ForgotPasswordRoute(state.email)) },
                onCreateAccount = { navController.navigate(EmailRegisterRoute) },
                onUsePhone = {
                    if (!navController.popBackStack<PhoneRoute>(inclusive = false)) navController.navigate(PhoneRoute)
                },
            )
        }
        composable<EmailRegisterRoute> {
            val vm: EmailRegisterViewModel = viewModel(
                factory = viewModelFactory {
                    initializer { EmailRegisterViewModel(container.authRepository, createSavedStateHandle(), container.clock) }
                },
            )
            val state by vm.state.collectAsStateWithLifecycle()
            LaunchedEffect(vm) {
                vm.events.collect { event ->
                    when (event) {
                        // Session ouverte : les écrans de connexion quittent la pile.
                        EmailRegisterEvent.Registered ->
                            navController.navigate(EmailVerifyRoute) { popUpTo(navController.graph.id) { inclusive = true } }
                    }
                }
            }
            EmailRegisterScreen(
                state = state,
                onFullNameChange = vm::onFullNameChange,
                onEmailChange = vm::onEmailChange,
                onPasswordChange = vm::onPasswordChange,
                onSubmit = vm::submit,
                onHaveAccount = {
                    if (!navController.popBackStack<EmailLoginRoute>(inclusive = false)) navController.navigate(EmailLoginRoute)
                },
            )
        }
        composable<EmailVerifyRoute> {
            val vm: EmailVerifyViewModel = viewModel(
                factory = viewModelFactory {
                    initializer { EmailVerifyViewModel(container.authRepository, createSavedStateHandle(), container.clock) }
                },
            )
            val state by vm.state.collectAsStateWithLifecycle()
            LaunchedEffect(vm) {
                vm.events.collect { event ->
                    when (event) {
                        EmailVerifyEvent.Verified -> {
                            navController.goHome()
                            scope.launch { snackbar.showSnackbar(emailVerifiedMessage) }
                        }
                    }
                }
            }
            EmailVerifyScreen(
                state = state,
                onCodeChange = vm::onCodeChange,
                onVerify = vm::verify,
                onResend = vm::resend,
                onLater = { navController.goHome() },
            )
        }
        composable<ForgotPasswordRoute> { entry ->
            val route = entry.toRoute<ForgotPasswordRoute>()
            val vm: ForgotPasswordViewModel = viewModel(
                factory = viewModelFactory {
                    initializer {
                        ForgotPasswordViewModel(route.email, container.authRepository, createSavedStateHandle(), container.clock)
                    }
                },
            )
            val state by vm.state.collectAsStateWithLifecycle()
            LaunchedEffect(vm) {
                vm.events.collect { event ->
                    when (event) {
                        is ForgotPasswordEvent.CodeSent -> navController.navigate(ResetPasswordRoute(event.email))
                    }
                }
            }
            ForgotPasswordScreen(state = state, onEmailChange = vm::onEmailChange, onSubmit = vm::submit)
        }
        composable<ResetPasswordRoute> { entry ->
            val route = entry.toRoute<ResetPasswordRoute>()
            val vm: ResetPasswordViewModel = viewModel(
                factory = viewModelFactory {
                    initializer {
                        ResetPasswordViewModel(route.email, container.authRepository, createSavedStateHandle(), container.clock)
                    }
                },
            )
            val state by vm.state.collectAsStateWithLifecycle()
            LaunchedEffect(vm) {
                vm.events.collect { event ->
                    when (event) {
                        // Retour à la connexion par email (adresse conservée) avec le nouveau mot de passe.
                        ResetPasswordEvent.PasswordChanged -> {
                            if (!navController.popBackStack<EmailLoginRoute>(inclusive = false)) {
                                navController.navigate(EmailLoginRoute) { popUpTo(navController.graph.id) { inclusive = true } }
                            }
                            scope.launch { snackbar.showSnackbar(passwordChangedMessage) }
                        }
                    }
                }
            }
            ResetPasswordScreen(
                state = state,
                onCodeChange = vm::onCodeChange,
                onPasswordChange = vm::onPasswordChange,
                onSubmit = vm::submit,
                onResend = vm::resend,
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

/** Session ouverte : accueil, en retirant les écrans de connexion de la pile (« retour » ferme l'app). */
private fun NavHostController.goHome() {
    navigate(HomeRoute) { popUpTo(graph.id) { inclusive = true } }
}

/** Écrans utilisables sans session : une déconnexion n'a pas à les quitter. */
private fun NavDestination.isSignInScreen(): Boolean =
    hasRoute<PhoneRoute>() || hasRoute<OtpRoute>() || hasRoute<EmailLoginRoute>() || hasRoute<EmailRegisterRoute>() ||
        hasRoute<ForgotPasswordRoute>() || hasRoute<ResetPasswordRoute>()

/** Pendant la lecture des jetons au démarrage (quelques dizaines de millisecondes). */
@Composable
private fun LoadingScreen(modifier: Modifier = Modifier) {
    val loading = stringResource(R.string.loading)
    Box(modifier = modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
        CircularProgressIndicator(modifier = Modifier.semantics { contentDescription = loading })
    }
}
