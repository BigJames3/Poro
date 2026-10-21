package com.poro.app

import android.content.Context
import com.poro.app.core.media.ContentResolverMediaReader
import com.poro.app.core.media.LocalMediaReader
import com.poro.app.core.network.NetworkModule
import com.poro.app.core.network.PresignedUploader
import com.poro.app.core.security.EncryptedTokenStore
import com.poro.app.core.security.TokenStore
import com.poro.app.core.time.Clock
import com.poro.app.core.time.SystemElapsedClock
import com.poro.app.data.auth.AccountRepository
import com.poro.app.data.auth.AuthRepository
import com.poro.app.data.auth.DefaultAccountRepository
import com.poro.app.data.auth.DefaultAuthRepository
import com.poro.app.data.user.DefaultProfileRepository
import com.poro.app.data.user.ProfileRepository
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

    val accountRepository: AccountRepository = DefaultAccountRepository(network.authApi, tokenStore)

    val mediaReader: LocalMediaReader = ContentResolverMediaReader(context)

    val uploader = PresignedUploader(network.mediaClient)

    val profileRepository: ProfileRepository = DefaultProfileRepository(network.api(), uploader)
}

sealed interface SessionEvent {
    /** Le rafraîchissement a été refusé : jetons effacés, retour à la connexion. */
    data object Expired : SessionEvent
}
