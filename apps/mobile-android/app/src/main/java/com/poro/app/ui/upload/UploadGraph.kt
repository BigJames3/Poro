package com.poro.app.ui.upload

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.compose.LifecycleEventEffect
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.createSavedStateHandle
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import com.poro.app.data.video.UploadState
import com.poro.app.ui.AppNav
import com.poro.app.ui.UploadRoute
import com.poro.app.ui.VideoRoute
import com.poro.app.ui.poroViewModel

/** Onglet « Publier » : une vidéo ou des photos, légende, envoi et suivi du traitement. */
fun NavGraphBuilder.uploadGraph(nav: AppNav) {
    composable<UploadRoute> {
        val container = nav.container
        val vm = poroViewModel {
            UploadViewModel(container.uploadManager, container.mediaReader, createSavedStateHandle(), container.imagePreparer)
        }
        val form by vm.form.collectAsStateWithLifecycle()
        val upload by vm.upload.collectAsStateWithLifecycle()
        val publish = rememberNotificationPermission(then = vm::publish)
        UploadScreen(
            form = form,
            upload = upload,
            onVideoPicked = vm::onVideoPicked,
            onTitleChange = vm::onTitleChange,
            onDescriptionChange = vm::onDescriptionChange,
            onPublish = publish,
            onRetry = vm::retry,
            onCancelUpload = vm::cancelUpload,
            onDismissResult = vm::dismissResult,
            onOpenVideo = { nav.controller.navigate(VideoRoute(it)) },
            onModeChange = vm::setMode,
            onPhotosPicked = vm::onPhotosPicked,
            onRemovePhoto = vm::removePhoto,
            onMovePhoto = vm::movePhoto,
            onCaptionChange = vm::onCaptionChange,
        )
    }
}

/**
 * Ses vidéos sur l'onglet Profil, relues au retour sur l'onglet et quand une publication en cours
 * change d'état (envoyée, en ligne ou en échec).
 */
@Composable
fun MyVideos(nav: AppNav) {
    val vm = poroViewModel { MyVideosViewModel(nav.container.videoRepository) }
    val state by vm.state.collectAsStateWithLifecycle()
    LifecycleEventEffect(Lifecycle.Event.ON_RESUME) { vm.reload() }
    val upload by nav.container.uploadManager.state.collectAsStateWithLifecycle()
    val settled = when (val u = upload) {
        is UploadState.Processing -> u.video.id + ":processing"
        is UploadState.Finished -> u.video.id + ":" + u.video.status
        else -> null
    }
    LaunchedEffect(settled) { if (settled != null) vm.reload() }
    MyVideosSection(
        state = state,
        onOpen = { nav.controller.navigate(VideoRoute(it.id)) },
        onDelete = vm::delete,
        onLoadMore = vm::loadMore,
    )
}
