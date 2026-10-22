package com.poro.app

import android.content.Context
import com.poro.app.core.legal.LegalTextSource
import com.poro.app.core.legal.RawLegalTextSource
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
import com.poro.app.data.feed.DefaultFeedRepository
import com.poro.app.data.feed.FeedRepository
import com.poro.app.data.moderation.DefaultModerationRepository
import com.poro.app.data.moderation.ModerationRepository
import com.poro.app.data.notification.DefaultNotificationRepository
import com.poro.app.data.notification.NotificationRepository
import com.poro.app.data.order.DefaultOrderRepository
import com.poro.app.data.order.OrderRepository
import com.poro.app.data.search.DefaultSearchRepository
import com.poro.app.data.search.SearchRepository
import com.poro.app.data.shop.DefaultShopRepository
import com.poro.app.data.shop.ShopRepository
import com.poro.app.data.social.DefaultSocialRepository
import com.poro.app.data.social.SocialRepository
import com.poro.app.data.user.DefaultProfileRepository
import com.poro.app.data.user.ProfileRepository
import com.poro.app.data.video.DefaultVideoRepository
import com.poro.app.data.video.UploadManager
import com.poro.app.data.video.VideoUploadClient
import com.poro.app.data.video.VideoUploader
import com.poro.app.data.video.WorkManagerUploadScheduler
import com.poro.app.data.video.VideoRepository
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

    /** Vit autant que le processus : travaux qui ne dépendent d'aucun écran (envoi de vidéo). */
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

    val feedRepository: FeedRepository = DefaultFeedRepository(network.api())

    val videoRepository: VideoRepository = DefaultVideoRepository(network.api())

    val socialRepository: SocialRepository = DefaultSocialRepository(network.api())

    val searchRepository: SearchRepository = DefaultSearchRepository(network.api())

    val notificationRepository: NotificationRepository = DefaultNotificationRepository(network.api())

    /** Documents légaux embarqués (textes provisoires « À valider » tant que non validés). */
    val legalTexts: LegalTextSource = RawLegalTextSource(context)

    val moderationRepository: ModerationRepository = DefaultModerationRepository(network.api())

    val shopRepository: ShopRepository = DefaultShopRepository(network.api(), uploader)

    val orderRepository: OrderRepository = DefaultOrderRepository(network.api())

    /** Envoi d'une vidéo, exécuté par le worker WorkManager. */
    val videoUploadClient: VideoUploadClient = VideoUploader(network.api(), uploader, mediaReader)

    val uploadManager = UploadManager(
        scheduler = WorkManagerUploadScheduler(context),
        videos = videoRepository,
        scope = applicationScope,
    )
}

sealed interface SessionEvent {
    /** Le rafraîchissement a été refusé : jetons effacés, retour à la connexion. */
    data object Expired : SessionEvent
}
