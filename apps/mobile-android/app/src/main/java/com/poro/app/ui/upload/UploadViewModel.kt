package com.poro.app.ui.upload

import androidx.lifecycle.SavedStateHandle
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.poro.app.R
import com.poro.app.core.media.LocalMedia
import com.poro.app.core.media.LocalMediaReader
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
    val canPublish: Boolean
        get() = file != null && title.trim().length in 1..TITLE_MAX && description.length <= DESCRIPTION_MAX

    companion object {
        const val TITLE_MAX = 100
        const val DESCRIPTION_MAX = 1000
    }
}

/**
 * Formulaire de publication. Le fichier est contrôlé ici (ADR-0007 : MP4, MOV ou WebM, 256 Mo,
 * 3 minutes) : refuser avant l'envoi épargne le forfait de données de l'utilisateur.
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
    }

    fun onVideoPicked(uri: String) {
        _form.update { it.copy(checkingFile = true, fileError = null, file = null) }
        viewModelScope.launch {
            val file = media.describe(uri)
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
        val title = value.take(UploadFormState.TITLE_MAX)
        savedState[KEY_TITLE] = title
        _form.update { it.copy(title = title) }
    }

    fun onDescriptionChange(value: String) {
        val description = value.take(UploadFormState.DESCRIPTION_MAX)
        savedState[KEY_DESCRIPTION] = description
        _form.update { it.copy(description = description) }
    }

    fun publish() {
        val form = _form.value
        val file = form.file ?: return
        if (!form.canPublish) return
        if (manager.start(file, form.title.trim(), form.description.trim().ifEmpty { null })) {
            // Le formulaire repart à vide : l'envoi continue même si l'on quitte l'écran.
            savedState[KEY_URI] = null
            onTitleChange("")
            onDescriptionChange("")
            _form.update { it.copy(file = null) }
        }
    }

    fun cancelUpload() = manager.cancel()

    fun dismissResult() = manager.dismiss()

    private companion object {
        const val KEY_URI = "uri"
        const val KEY_TITLE = "title"
        const val KEY_DESCRIPTION = "description"
        const val MAX_BYTES = 256L * 1024 * 1024
        const val MAX_DURATION_MS = 3 * 60 * 1000L
        val ACCEPTED_TYPES = setOf("video/mp4", "video/quicktime", "video/webm")
    }
}
