package com.poro.app.ui.upload

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.R
import com.poro.app.core.media.LocalImagePreparer
import com.poro.app.core.media.LocalMedia
import com.poro.app.core.media.LocalMediaReader
import com.poro.app.core.media.PrepareResult
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

/** Ce que l'on publie : une vidéo, ou une à dix photos (ADR-0020). */
enum class PublishMode { Video, Photo }

data class UploadFormState(
    val file: LocalMedia? = null,
    val title: String = "",
    val description: String = "",
    val checkingFile: Boolean = false,
    /** Vidéo refusée avant tout envoi (format, taille, durée). */
    val fileError: ErrorMessage? = null,
    val mode: PublishMode = PublishMode.Video,
    /** Photos choisies (URI du sélecteur), dans l'ordre de la publication. */
    val photos: List<String> = emptyList(),
    /** Légende facultative d'une publication photo, hashtags compris. */
    val caption: String = "",
    /** Photos en cours de préparation (orientation, réduction, JPEG) avant l'envoi. */
    val preparing: Boolean = false,
    /** Photo refusée avant l'envoi, ou trop de photos choisies. */
    val photoError: ErrorMessage? = null,
) {
    /** Longueurs comptées comme le service video : en points de code, espaces de bord retirés. */
    val titleLength: Int get() = Runes.count(title.trim())
    val descriptionLength: Int get() = Runes.count(description.trim())

    /** Caractère invisible refusé par le serveur (`title_invalid` / `description_invalid`). */
    val titleHasForbidden: Boolean get() = Runes.hasForbidden(title, allowNewlines = false)
    val descriptionHasForbidden: Boolean get() = Runes.hasForbidden(description, allowNewlines = true)

    val captionLength: Int get() = Runes.count(caption.trim())
    val captionHasForbidden: Boolean get() = Runes.hasForbidden(caption, allowNewlines = true)

    val canPublish: Boolean
        get() = when (mode) {
            PublishMode.Video -> file != null && titleLength in 1..TITLE_MAX && descriptionLength <= DESCRIPTION_MAX &&
                !titleHasForbidden && !descriptionHasForbidden
            PublishMode.Photo -> photos.size in 1..MAX_PHOTOS && captionLength <= DESCRIPTION_MAX &&
                !captionHasForbidden && !preparing
        }

    companion object {
        /** services/video/internal/model : MaxTitleRunes = 100, MaxDescriptionRunes = 500. */
        const val TITLE_MAX = 100
        const val DESCRIPTION_MAX = 500

        /** services/video/internal/model : MaxImages = 10. */
        const val MAX_PHOTOS = 10
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
    private val images: LocalImagePreparer? = null,
) : ViewModel() {

    private val _form = MutableStateFlow(
        UploadFormState(
            title = savedState[KEY_TITLE] ?: "",
            description = savedState[KEY_DESCRIPTION] ?: "",
            mode = savedState.get<String>(KEY_MODE)?.let(PublishMode::valueOf) ?: PublishMode.Video,
            photos = savedState.get<ArrayList<String>>(KEY_PHOTOS).orEmpty(),
            caption = savedState[KEY_CAPTION] ?: "",
        ),
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
            // Vidéo illisible : on l'oublie, le titre et la description restent. Les photos, elles,
            // sont préparées à nouveau depuis leur sélection au prochain essai.
            is UploadState.Failed -> if (state.mediaLost && _form.value.mode == PublishMode.Video) {
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
        onCaptionChange("")
        savePhotos(emptyList())
        _form.update { it.copy(file = null, fileError = null, photos = emptyList(), photoError = null) }
    }

    fun setMode(mode: PublishMode) {
        savedState[KEY_MODE] = mode.name
        _form.update { it.copy(mode = mode) }
    }

    /** Photos choisies dans le sélecteur, ajoutées à la suite ; au-delà de dix, les suivantes sont écartées. */
    fun onPhotosPicked(uris: List<String>) {
        val current = _form.value.photos
        val added = uris.filter { it !in current }
        val kept = (current + added).take(UploadFormState.MAX_PHOTOS)
        savePhotos(kept)
        _form.update {
            it.copy(
                photos = kept,
                photoError = if (current.size + added.size > UploadFormState.MAX_PHOTOS) ErrorMessage(R.string.upload_photo_too_many) else null,
            )
        }
    }

    fun removePhoto(index: Int) {
        val photos = _form.value.photos.toMutableList()
        if (index !in photos.indices) return
        photos.removeAt(index)
        savePhotos(photos)
        _form.update { it.copy(photos = photos, photoError = null) }
    }

    /** Réordonne le carrousel : la photo [from] prend la place [to]. */
    fun movePhoto(from: Int, to: Int) {
        val photos = _form.value.photos.toMutableList()
        if (from !in photos.indices || to !in photos.indices || from == to) return
        photos.add(to, photos.removeAt(from))
        savePhotos(photos)
        _form.update { it.copy(photos = photos) }
    }

    fun onCaptionChange(value: String) {
        val caption = Runes.take(value, UploadFormState.DESCRIPTION_MAX)
        savedState[KEY_CAPTION] = caption
        _form.update { it.copy(caption = caption) }
    }

    private fun savePhotos(photos: List<String>) {
        savedState[KEY_PHOTOS] = ArrayList(photos)
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
        if (!form.canPublish || manager.isBusy) return
        if (form.mode == PublishMode.Photo) {
            publishPhotos(form)
            return
        }
        val file = form.file ?: return
        manager.start(file, form.title.trim(), form.description.trim().ifEmpty { null })
    }

    /**
     * Prépare les photos (orientation, 2048 px, JPEG) puis les envoie. Une photo illisible arrête
     * tout avant l'envoi : rien n'est publié à moitié, et l'utilisateur sait laquelle retirer.
     */
    private fun publishPhotos(form: UploadFormState) {
        val preparer = images ?: return
        _form.update { it.copy(preparing = true, photoError = null) }
        viewModelScope.launch {
            val result = preparer.prepare(form.photos)
            _form.update { it.copy(preparing = false) }
            when (result) {
                is PrepareResult.Failed -> _form.update {
                    it.copy(photoError = ErrorMessage(result.reason.message(), arg = result.position))
                }
                is PrepareResult.Ready -> {
                    val caption = form.caption.trim().ifEmpty { null }
                    val label = caption?.let { Runes.take(it, LABEL_MAX) } ?: PHOTOS_LABEL
                    if (!manager.startPhotos(result.images, caption, label)) preparer.discard(result.images)
                }
            }
        }
    }

    /**
     * Relance le même envoi après un échec réseau ou serveur. Le précédent a été annulé, ou reste
     * mémorisé si son issue est inconnue : le nouvel essai le relit avant tout (pas de doublon).
     */
    fun retry() {
        val failed = upload.value as? UploadState.Failed ?: return
        val form = _form.value
        // Une publication photo repart des photos choisies : même un fichier préparé perdu se refait.
        val retryable = failed.canRetry() || (form.mode == PublishMode.Photo && failed.mediaLost)
        if (!retryable || !form.canPublish) return
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
        const val KEY_MODE = "mode"
        const val KEY_PHOTOS = "photos"
        const val KEY_CAPTION = "caption"

        /** Libellé d'une publication photo sans légende, pendant l'envoi. */
        const val PHOTOS_LABEL = "Photos"
        const val LABEL_MAX = 60
        const val MAX_BYTES = 256L * 1024 * 1024
        const val MAX_DURATION_MS = 3 * 60 * 1000L
        val ACCEPTED_TYPES = setOf("video/mp4", "video/quicktime", "video/webm")
    }
}

private fun PrepareResult.Reason.message(): Int = when (this) {
    PrepareResult.Reason.Unsupported -> R.string.upload_photo_unsupported
    PrepareResult.Reason.Unreadable -> R.string.upload_photo_unreadable
}
