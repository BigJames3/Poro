package com.poro.app

import android.content.Context
import com.poro.app.core.network.NetworkModule
import com.poro.app.core.security.EncryptedTokenStore
import com.poro.app.core.security.TokenStore
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.asSharedFlow
import okhttp3.HttpUrl.Companion.toHttpUrl

/**
 * Dépendances de l'application, créées une fois au démarrage (injection manuelle).
 * Un framework d'injection (Hilt) pourra le remplacer quand l'application grandira.
 */
class AppContainer(context: Context) {

    val tokenStore: TokenStore = EncryptedTokenStore(context)

    private val sessionEvents = MutableSharedFlow<SessionEvent>(extraBufferCapacity = 1)

    /** Événements de session que l'interface doit afficher (session expirée, révoquée…). */
    val sessionExpired: SharedFlow<SessionEvent> = sessionEvents.asSharedFlow()

    val network = NetworkModule(
        baseUrl = BuildConfig.API_BASE_URL.toHttpUrl(),
        tokens = tokenStore,
        debug = BuildConfig.DEBUG,
        onSessionExpired = { sessionEvents.tryEmit(SessionEvent.Expired) },
    )
}

sealed interface SessionEvent {
    /** Le rafraîchissement a été refusé : jetons effacés, retour à la connexion. */
    data object Expired : SessionEvent
}
