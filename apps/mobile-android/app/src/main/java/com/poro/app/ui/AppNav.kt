package com.poro.app.ui

import android.content.res.Resources
import androidx.annotation.StringRes
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.SnackbarResult
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewmodel.CreationExtras
import androidx.lifecycle.viewmodel.compose.viewModel
import androidx.lifecycle.viewmodel.initializer
import androidx.lifecycle.viewmodel.viewModelFactory
import androidx.navigation.NavBackStackEntry
import androidx.navigation.NavDestination
import androidx.navigation.NavDestination.Companion.hasRoute
import androidx.navigation.NavHostController
import com.poro.app.AppContainer
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.launch

/** Ce dont chaque graphe de navigation a besoin : pile, dépendances et messages éphémères. */
class AppNav(
    val controller: NavHostController,
    val container: AppContainer,
    private val snackbar: SnackbarHostState,
    private val scope: CoroutineScope,
    private val resources: Resources,
) {
    /** Message en bas de l'écran ; il survit à la navigation qui le déclenche. */
    fun showMessage(@StringRes text: Int, vararg args: Any) {
        val message = resources.getString(text, *args)
        scope.launch { snackbar.showSnackbar(message) }
    }

    /** Session ouverte : accueil, en retirant les écrans de connexion de la pile (« retour » ferme l'app). */
    fun goHome() {
        controller.navigate(HomeRoute) { popUpTo(controller.graph.id) { inclusive = true } }
    }

    /** Message avec une action (« Voir le panier ») ; [onAction] s'exécute si l'utilisateur l'appuie. */
    fun showMessageWithAction(@StringRes text: Int, @StringRes action: Int, onAction: () -> Unit, vararg args: Any) {
        val message = resources.getString(text, *args)
        val label = resources.getString(action)
        scope.launch {
            if (snackbar.showSnackbar(message, actionLabel = label) == SnackbarResult.ActionPerformed) onAction()
        }
    }

    /** Retour à la connexion, pile vidée (déconnexion, session expirée ou supprimée). */
    fun goToSignIn() {
        controller.navigate(PhoneRoute) { popUpTo(controller.graph.id) { inclusive = true } }
    }

    fun back() {
        controller.popBackStack()
    }

    /** Revient à l'écran précédent en lui signalant [key] (qu'il lit avec [ConsumeResult]). */
    fun backWithResult(key: String) {
        controller.previousBackStackEntry?.savedStateHandle?.set(key, true)
        back()
    }
}

/** Un écran enfant du profil l'a modifié (pseudo, avatar, adresse vérifiée) : le recharger au retour. */
const val RESULT_PROFILE_CHANGED = "profile_changed"

/** Exécute [onResult] une fois quand un écran enfant a posé [key] (voir [AppNav.backWithResult]). */
@Composable
fun ConsumeResult(entry: NavBackStackEntry, key: String, onResult: () -> Unit) {
    val flag by entry.savedStateHandle.getStateFlow(key, false).collectAsStateWithLifecycle()
    LaunchedEffect(flag) {
        if (flag) {
            entry.savedStateHandle[key] = false
            onResult()
        }
    }
}

/** Écrans utilisables sans session : une déconnexion n'a pas à les quitter. */
fun NavDestination.isSignInScreen(): Boolean =
    hasRoute<PhoneRoute>() || hasRoute<OtpRoute>() || hasRoute<EmailLoginRoute>() || hasRoute<EmailRegisterRoute>() ||
        hasRoute<ForgotPasswordRoute>() || hasRoute<ResetPasswordRoute>()

/**
 * ViewModel d'une destination, créé avec ses dépendances (injection manuelle). [key] distingue
 * plusieurs instances du même type sur un écran (les commentaires de chaque vidéo du fil).
 */
@Composable
inline fun <reified VM : ViewModel> poroViewModel(key: String? = null, noinline create: CreationExtras.() -> VM): VM =
    viewModel(key = key, factory = viewModelFactory { initializer(create) })

/** Événements ponctuels d'un ViewModel (navigation, messages), reçus tant que l'écran est affiché. */
@Composable
fun <T> CollectEvents(events: Flow<T>, onEvent: suspend (T) -> Unit) {
    LaunchedEffect(events) { events.collect { onEvent(it) } }
}
