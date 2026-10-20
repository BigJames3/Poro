package com.poro.app

import android.content.Context
import com.poro.app.core.network.NetworkModule
import com.poro.app.core.security.EncryptedTokenStore
import com.poro.app.core.security.TokenStore
import com.poro.app.core.time.Clock
import com.poro.app.core.time.SystemElapsedClock
import com.poro.app.data.auth.AuthRepository
import com.poro.app.data.auth.DefaultAuthRepository
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.asSharedFlow
import okhttp3.HttpUrl.Companion.toHttpUrl

/**
 * Dépendances de l'application, créées une fois au démarrage (injection manuelle).
 * Un framework d'injection (Hilt) pourra le remplacer quand l'application grandira.
 */
class AppContainer(context: Context) {

    /** Vit autant que le processus : travaux qui ne dépendent d'aucun écran. */
    private val applicationScope = CoroutineScope(SupervisorJob() + Dispatchers.IO)

    val tokenStore: TokenStore = EncryptedTokenStore(context, applicationScope)

    val clock: Clock = SystemElapsedClock

    private val sessionEvents = MutableSharedFlow<SessionEvent>(extraBufferCapacity = 1)

    /** Événements de session que l'interface doit afficher (session expirée, révoquée…). */
    val sessionExpired: SharedFlow<SessionEvent> = sessionEvents.asSharedFlow()

    val network = NetworkModule(
        baseUrl = BuildConfig.API_BASE_URL.toHttpUrl(),
        tokens = tokenStore,
        debug = BuildConfig.DEBUG,
        onSessionExpired = { sessionEvents.tryEmit(SessionEvent.Expired) },
    )

    val authRepository: AuthRepository = DefaultAuthRepository(network.authApi, tokenStore)
}

sealed interface SessionEvent {
    /** Le rafraîchissement a été refusé : jetons effacés, retour à la connexion. */
    data object Expired : SessionEvent
}
