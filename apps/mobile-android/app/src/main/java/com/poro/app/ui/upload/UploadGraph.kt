package com.poro.app.ui.upload

import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.createSavedStateHandle
import androidx.navigation.NavGraphBuilder
import androidx.navigation.compose.composable
import com.poro.app.ui.AppNav
import com.poro.app.ui.UploadRoute
import com.poro.app.ui.VideoRoute
import com.poro.app.ui.poroViewModel

/** Onglet « Publier » : choix de la vidéo, titre, envoi et suivi du traitement. */
fun NavGraphBuilder.uploadGraph(nav: AppNav) {
    composable<UploadRoute> {
        val container = nav.container
        val vm = poroViewModel { UploadViewModel(container.uploadManager, container.mediaReader, createSavedStateHandle()) }
        val form by vm.form.collectAsStateWithLifecycle()
        val upload by vm.upload.collectAsStateWithLifecycle()
        UploadScreen(
            form = form,
            upload = upload,
            onVideoPicked = vm::onVideoPicked,
            onTitleChange = vm::onTitleChange,
            onDescriptionChange = vm::onDescriptionChange,
            onPublish = vm::publish,
            onRetry = vm::retry,
            onCancelUpload = vm::cancelUpload,
            onDismissResult = vm::dismissResult,
            onOpenVideo = { nav.controller.navigate(VideoRoute(it)) },
        )
    }
}

/** Ses vidéos sur l'onglet Profil. */
@Composable
fun MyVideos(nav: AppNav) {
    val vm = poroViewModel { MyVideosViewModel(nav.container.videoRepository) }
    val state by vm.state.collectAsStateWithLifecycle()
    MyVideosSection(
        state = state,
        onOpen = { nav.controller.navigate(VideoRoute(it.id)) },
        onDelete = vm::delete,
        onLoadMore = vm::loadMore,
    )
}
