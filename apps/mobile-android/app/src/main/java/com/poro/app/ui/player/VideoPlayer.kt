package com.poro.app.ui.player

import android.content.Context
import android.graphics.Color
import androidx.annotation.OptIn
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.Stable
import androidx.compose.runtime.State
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.viewinterop.AndroidView
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.media3.common.MediaItem
import androidx.media3.common.MimeTypes
import androidx.media3.common.Player
import androidx.media3.common.util.UnstableApi
import androidx.media3.datasource.okhttp.OkHttpDataSource
import androidx.media3.exoplayer.DefaultLoadControl
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.DefaultMediaSourceFactory
import androidx.media3.ui.AspectRatioFrameLayout
import androidx.media3.ui.PlayerView
import okhttp3.OkHttpClient

/**
 * Lecteur HLS partagé : un seul pour tout un écran, qui ne lit que la vidéo visible. Dans le fil,
 * un second lecteur prépare la vidéo suivante en pause ([preload]) ; au glissement, les deux
 * échangent leurs rôles et la vidéo démarre sans attendre le réseau. Tout l'usage de Media3 est
 * dans ce fichier.
 */
@Stable
class VideoPlayer internal constructor(private val newPlayer: () -> ExoPlayer) {
    private val playing = mutableStateOf(false)

    /** Lecteur affiché ; état Compose pour que la surface suive l'échange des lecteurs. */
    private var active by mutableStateOf(newPlayer().also(::listen))

    /** Lecteur de la vidéo suivante, créé au premier préchargement (jamais pour une vidéo seule). */
    private var standby: ExoPlayer? = null

    private var currentUrl: String? = null
    private var preloadedUrl: String? = null

    /** L'utilisateur a mis en pause : le retour au premier plan ne relance pas la lecture. */
    private var pausedByUser = false

    val isPlaying: State<Boolean> get() = playing

    internal val exo: ExoPlayer get() = active

    private fun listen(player: ExoPlayer) {
        player.addListener(object : Player.Listener {
            override fun onIsPlayingChanged(isPlaying: Boolean) {
                if (player === active) playing.value = isPlaying
            }
        })
    }

    /** Lit [url] en boucle depuis le début (nouvelle vidéo affichée). */
    fun play(url: String) {
        pausedByUser = false
        if (url != currentUrl) {
            val next = standby
            if (url == preloadedUrl && next != null) {
                // Vidéo déjà préparée : on échange les lecteurs, l'ancien redevient disponible.
                val previous = active
                active = next
                standby = previous
                previous.playWhenReady = false
                previous.clearMediaItems()
            } else {
                active.setMediaItem(hlsItem(url))
                active.prepare()
            }
            preloadedUrl = null
            currentUrl = url
        }
        active.playWhenReady = true
        playing.value = active.isPlaying
    }

    /**
     * Prépare [url] (la vidéo suivante du fil) sans la lire ni l'afficher. Seuls les premiers
     * segments sont chargés (tampon plafonné), pour ne pas gaspiller le forfait si l'on ne glisse pas.
     */
    fun preload(url: String?) {
        if (url == null || url == currentUrl || url == preloadedUrl) return
        val player = standby ?: newPlayer().also { listen(it); standby = it }
        preloadedUrl = url
        player.playWhenReady = false
        player.setMediaItem(hlsItem(url))
        player.prepare()
    }

    /** Appui sur la vidéo : pause ou reprise. */
    fun toggle() {
        pausedByUser = active.playWhenReady
        active.playWhenReady = !active.playWhenReady
    }

    internal fun onBackground() {
        active.playWhenReady = false
    }

    internal fun onForeground() {
        if (currentUrl != null && !pausedByUser) active.playWhenReady = true
    }

    internal fun release() {
        active.release()
        standby?.release()
        standby = null
    }

    private fun hlsItem(url: String) = MediaItem.Builder().setUri(url).setMimeType(MimeTypes.APPLICATION_M3U8).build()
}

/** Crée le lecteur pour la durée de l'écran ; pause en arrière-plan, libéré à la sortie. */
@Composable
fun rememberVideoPlayer(client: OkHttpClient): VideoPlayer {
    val context = LocalContext.current
    val player = remember { VideoPlayer { buildExoPlayer(context, client) } }
    val lifecycle = LocalLifecycleOwner.current.lifecycle
    DisposableEffect(lifecycle, player) {
        val observer = LifecycleEventObserver { _, event ->
            when (event) {
                Lifecycle.Event.ON_PAUSE -> player.onBackground()
                Lifecycle.Event.ON_RESUME -> player.onForeground()
                else -> Unit
            }
        }
        lifecycle.addObserver(observer)
        onDispose {
            lifecycle.removeObserver(observer)
            player.release()
        }
    }
    return player
}

/** Surface d'affichage, sans contrôles : la vidéo remplit l'écran (rognée si besoin). */
@OptIn(UnstableApi::class)
@Composable
fun VideoSurface(player: VideoPlayer, modifier: Modifier = Modifier) {
    AndroidView(
        modifier = modifier,
        factory = { context ->
            PlayerView(context).apply {
                useController = false
                resizeMode = AspectRatioFrameLayout.RESIZE_MODE_ZOOM
                // Transparent : la miniature, dessinée dessous, reste visible jusqu'à la première image.
                setShutterBackgroundColor(Color.TRANSPARENT)
                this.player = player.exo
            }
        },
        update = { view -> view.player = player.exo },
        onRelease = { view -> view.player = null },
    )
}

/**
 * Données lues par le client des médias : pas de jeton, réécriture de localhost en debug. Tampon
 * plus court que par défaut (50 s) : des vidéos courtes, des forfaits mobiles chers, et un lecteur
 * de préchargement qui ne doit charger que le début de la vidéo suivante.
 */
@OptIn(UnstableApi::class)
private fun buildExoPlayer(context: Context, client: OkHttpClient): ExoPlayer =
    ExoPlayer.Builder(context)
        .setMediaSourceFactory(DefaultMediaSourceFactory(OkHttpDataSource.Factory(client)))
        .setLoadControl(
            DefaultLoadControl.Builder()
                .setBufferDurationsMs(MIN_BUFFER_MS, MAX_BUFFER_MS, BUFFER_FOR_PLAYBACK_MS, BUFFER_AFTER_REBUFFER_MS)
                .build(),
        )
        .build()
        .apply { repeatMode = Player.REPEAT_MODE_ONE }

private const val MIN_BUFFER_MS = 4_000
private const val MAX_BUFFER_MS = 15_000
private const val BUFFER_FOR_PLAYBACK_MS = 1_000
private const val BUFFER_AFTER_REBUFFER_MS = 2_000
