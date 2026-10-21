package com.poro.app.ui.player

import android.content.Context
import android.graphics.Color
import androidx.annotation.OptIn
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.Stable
import androidx.compose.runtime.State
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
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
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.DefaultMediaSourceFactory
import androidx.media3.ui.AspectRatioFrameLayout
import androidx.media3.ui.PlayerView
import okhttp3.OkHttpClient

/**
 * Lecteur HLS partagé : une seule instance pour tout un écran, qui ne lit que la vidéo visible.
 * Tout l'usage de Media3 est dans ce fichier.
 */
@Stable
class VideoPlayer internal constructor(internal val exo: ExoPlayer) {
    private val playing = mutableStateOf(false)
    private var currentUrl: String? = null

    /** L'utilisateur a mis en pause : le retour au premier plan ne relance pas la lecture. */
    private var pausedByUser = false

    val isPlaying: State<Boolean> get() = playing

    init {
        exo.addListener(object : Player.Listener {
            override fun onIsPlayingChanged(isPlaying: Boolean) {
                playing.value = isPlaying
            }
        })
    }

    /** Lit [url] en boucle depuis le début (nouvelle vidéo affichée). */
    fun play(url: String) {
        pausedByUser = false
        if (url != currentUrl) {
            currentUrl = url
            exo.setMediaItem(MediaItem.Builder().setUri(url).setMimeType(MimeTypes.APPLICATION_M3U8).build())
            exo.prepare()
        }
        exo.playWhenReady = true
    }

    /** Appui sur la vidéo : pause ou reprise. */
    fun toggle() {
        pausedByUser = exo.playWhenReady
        exo.playWhenReady = !exo.playWhenReady
    }

    internal fun onBackground() {
        exo.playWhenReady = false
    }

    internal fun onForeground() {
        if (currentUrl != null && !pausedByUser) exo.playWhenReady = true
    }
}

/** Crée le lecteur pour la durée de l'écran ; pause en arrière-plan, libéré à la sortie. */
@Composable
fun rememberVideoPlayer(client: OkHttpClient): VideoPlayer {
    val context = LocalContext.current
    val player = remember { VideoPlayer(buildExoPlayer(context, client)) }
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
            player.exo.release()
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

/** Données lues par le client des médias : pas de jeton, réécriture de localhost en debug. */
@OptIn(UnstableApi::class)
private fun buildExoPlayer(context: Context, client: OkHttpClient): ExoPlayer =
    ExoPlayer.Builder(context)
        .setMediaSourceFactory(DefaultMediaSourceFactory(OkHttpDataSource.Factory(client)))
        .build()
        .apply { repeatMode = Player.REPEAT_MODE_ONE }
