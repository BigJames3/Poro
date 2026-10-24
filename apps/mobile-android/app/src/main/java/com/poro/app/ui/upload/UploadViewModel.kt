package com.poro.app.ui.upload

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.R
import com.poro.app.core.media.LocalMedia
import com.poro.app.core.media.LocalMediaReader
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.core.text.Runes
import com.poro.app.data.video.UploadManager
import com.poro.app.data.video.UploadState
import com.poro.app.ui.auth.ErrorMessage
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class UploadFormState(
    val file: LocalMedia? = null,
    val title: String = "",
    val description: String = "",
    val checkingFile: Boolean = false,
    /** Vidéo refusée avant tout envoi (format, taille, durée). */
    val fileError: ErrorMessage? = null,
) {
    /** Longueurs comptées comme le service video : en points de code, espaces de bord retirés. */
    val titleLength: Int get() = Runes.count(title.trim())
    val descriptionLength: Int get() = Runes.count(description.trim())

    /** Caractère invisible refusé par le serveur (`title_invalid` / `description_invalid`). */
    val titleHasForbidden: Boolean get() = Runes.hasForbidden(title, allowNewlines = false)
    val descriptionHasForbidden: Boolean get() = Runes.hasForbidden(description, allowNewlines = true)

    val canPublish: Boolean
        get() = file != null && titleLength in 1..TITLE_MAX && descriptionLength <= DESCRIPTION_MAX &&
            !titleHasForbidden && !descriptionHasForbidden

    companion object {
        /** services/video/internal/model : MaxTitleRunes = 100, MaxDescriptionRunes = 500. */
        const val TITLE_MAX = 100
        const val DESCRIPTION_MAX = 500
    }
}

/**
 * Échec que « Réessayer » relance tel quel sans risque de doublon : réseau, refus du stockage
 * (URL expirée, signature) ou erreur serveur. Un refus de contenu (titre, taille…) ou un fichier
 * devenu illisible demande d'abord une correction.
 */
fun UploadState.Failed.canRetry(): Boolean = when (val failure = failure) {
    is ApiResult.NetworkError -> true
    is ApiResult.Failure -> failure.code == ApiErrorCodes.UPLOAD_REJECTED || failure.httpStatus >= 500
    is ApiResult.Success -> false
}

/** Le fichier choisi n'est plus lisible : il faut le choisir à nouveau. */
val UploadState.Failed.mediaLost: Boolean
    get() = (failure as? ApiResult.Failure)?.code == ApiErrorCodes.MEDIA_UNREADABLE

/**
 * Formulaire de publication. Le fichier est contrôlé ici (ADR-0007 : MP4, MOV ou WebM, 256 Mo,
 * 3 minutes) : refuser avant l'envoi épargne le forfait de données de l'utilisateur.
 * Vidéo, titre et description sont gardés jusqu'à la confirmation du serveur : un échec se
 * corrige ou se relance sans tout ressaisir.
 */
class UploadViewModel(
    private val manager: UploadManager,
    private val media: LocalMediaReader,
    private val savedState: SavedStateHandle,
) : ViewModel() {

    private val _form = MutableStateFlow(
        UploadFormState(title = savedState[KEY_TITLE] ?: "", description = savedState[KEY_DESCRIPTION] ?: ""),
    )
    val form: StateFlow<UploadFormState> = _form.asStateFlow()

    val upload: StateFlow<UploadState> = manager.state

    init {
        savedState.get<String>(KEY_URI)?.let(::onVideoPicked)
        viewModelScope.launch { manager.state.collect(::onUploadState) }
    }

    private fun onUploadState(state: UploadState) {
        when (state) {
            // Le serveur a accepté la vidéo : le formulaire repart à vide, une seule fois par vidéo.
            is UploadState.Processing -> onPublished(state.video.id)
            is UploadState.Finished -> onPublished(state.video.id)
            // Fichier illisible : on l'oublie, le titre et la description restent.
            is UploadState.Failed -> if (state.mediaLost) {
                savedState[KEY_URI] = null
                _form.update { it.copy(file = null, fileError = ErrorMessage(R.string.upload_error_unreadable)) }
            }
            else -> Unit
        }
    }

    private fun onPublished(videoId: String) {
        if (savedState.get<String>(KEY_PUBLISHED) == videoId) return
        savedState[KEY_PUBLISHED] = videoId
        savedState[KEY_URI] = null
        onTitleChange("")
        onDescriptionChange("")
        _form.update { it.copy(file = null, fileError = null) }
    }

    fun onVideoPicked(uri: String) {
        _form.update { it.copy(checkingFile = true, fileError = null, file = null) }
        viewModelScope.launch {
            val file = try {
                media.describe(uri)
            } catch (e: SecurityException) {
                // Vidéo restaurée après un arrêt de l'app, sans autorisation durable.
                savedState[KEY_URI] = null
                _form.update { it.copy(checkingFile = false, fileError = ErrorMessage(R.string.upload_error_unreadable)) }
                return@launch
            }
            val error = when {
                file == null || file.mimeType !in ACCEPTED_TYPES -> R.string.upload_error_format
                file.sizeBytes > MAX_BYTES -> R.string.upload_error_size
                (file.durationMs ?: 0) > MAX_DURATION_MS -> R.string.upload_error_duration
                else -> null
            }
            savedState[KEY_URI] = if (error == null) uri else null
            _form.update {
                it.copy(checkingFile = false, file = file.takeIf { error == null }, fileError = error?.let(::ErrorMessage))
            }
        }
    }

    fun onTitleChange(value: String) {
        val title = Runes.take(value, UploadFormState.TITLE_MAX)
        savedState[KEY_TITLE] = title
        _form.update { it.copy(title = title) }
    }

    fun onDescriptionChange(value: String) {
        val description = Runes.take(value, UploadFormState.DESCRIPTION_MAX)
        savedState[KEY_DESCRIPTION] = description
        _form.update { it.copy(description = description) }
    }

    /**
     * Lance l'envoi ; sans effet si un envoi est déjà en cours (double appui). Le formulaire n'est
     * vidé qu'à la confirmation du serveur ([onUploadState]).
     */
    fun publish() {
        val form = _form.value
        val file = form.file ?: return
        if (!form.canPublish) return
        manager.start(file, form.title.trim(), form.description.trim().ifEmpty { null })
    }

    /**
     * Relance le même envoi après un échec réseau ou serveur. Le précédent a été annulé, ou reste
     * mémorisé si son issue est inconnue : le nouvel essai le relit avant tout (pas de doublon).
     */
    fun retry() {
        val failed = upload.value as? UploadState.Failed ?: return
        if (!failed.canRetry() || !_form.value.canPublish) return
        manager.dismiss()
        publish()
    }

    fun cancelUpload() = manager.cancel()

    fun dismissResult() = manager.dismiss()

    private companion object {
        const val KEY_URI = "uri"
        const val KEY_TITLE = "title"
        const val KEY_DESCRIPTION = "description"
        const val KEY_PUBLISHED = "published_video"
        const val MAX_BYTES = 256L * 1024 * 1024
        const val MAX_DURATION_MS = 3 * 60 * 1000L
        val ACCEPTED_TYPES = setOf("video/mp4", "video/quicktime", "video/webm")
    }
}
