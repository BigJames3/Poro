package com.poro.app.ui.upload

import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.PickVisualMediaRequest
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.itemsIndexed
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.KeyboardArrowLeft
import androidx.compose.material.icons.automirrored.filled.KeyboardArrowRight
import androidx.compose.material.icons.filled.Close
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.unit.dp
import coil3.compose.AsyncImage
import com.poro.app.R
import com.poro.app.core.format.Formats
import com.poro.app.data.video.UploadState
import com.poro.app.data.video.VideoDto
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.auth.toErrorMessage
import com.poro.app.ui.player.LocalVideoPreview
import com.poro.app.ui.theme.brandButtonColors

@Composable
fun UploadScreen(
    form: UploadFormState,
    upload: UploadState,
    onVideoPicked: (String) -> Unit,
    onTitleChange: (String) -> Unit,
    onDescriptionChange: (String) -> Unit,
    onPublish: () -> Unit,
    onRetry: () -> Unit,
    onCancelUpload: () -> Unit,
    onDismissResult: () -> Unit,
    onOpenVideo: (String) -> Unit,
    onModeChange: (PublishMode) -> Unit = {},
    onPhotosPicked: (List<String>) -> Unit = {},
    onRemovePhoto: (Int) -> Unit = {},
    onMovePhoto: (from: Int, to: Int) -> Unit = { _, _ -> },
    onCaptionChange: (String) -> Unit = {},
) {
    val picker = rememberLauncherForActivityResult(ActivityResultContracts.PickVisualMedia()) { uri ->
        uri?.let { onVideoPicked(it.toString()) }
    }
    val pickVideo = { picker.launch(PickVisualMediaRequest(ActivityResultContracts.PickVisualMedia.VideoOnly)) }
    val photoPicker = rememberLauncherForActivityResult(ActivityResultContracts.PickMultipleVisualMedia(UploadFormState.MAX_PHOTOS)) { uris ->
        if (uris.isNotEmpty()) onPhotosPicked(uris.map { it.toString() })
    }
    val pickPhotos = { photoPicker.launch(PickVisualMediaRequest(ActivityResultContracts.PickVisualMedia.ImageOnly)) }
    Column(
        modifier = Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Text(
            stringResource(R.string.upload_title),
            style = MaterialTheme.typography.headlineMedium,
            modifier = Modifier.semantics { heading() },
        )
        UploadStatusCard(
            upload = upload,
            onCancel = onCancelUpload,
            onDismiss = onDismissResult,
            onOpenVideo = onOpenVideo,
            // « Réessayer » seulement si l'envoi peut être relancé tel quel (vidéo toujours lisible).
            onRetry = onRetry.takeIf { upload is UploadState.Failed && upload.canRetry() && form.canPublish },
            onRepick = {
                onDismissResult()
                pickVideo()
            },
        )
        if (upload is UploadState.Uploading) return@Column

        ModeSelector(form.mode, enabled = !form.preparing, onModeChange)
        if (form.mode == PublishMode.Photo) {
            PhotoForm(form, pickPhotos, onRemovePhoto, onMovePhoto, onCaptionChange, onPublish)
            return@Column
        }

        Text(stringResource(R.string.upload_rules), style = MaterialTheme.typography.bodyMedium)
        OutlinedButton(
            onClick = pickVideo,
            enabled = !form.checkingFile,
            modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp),
        ) {
            Text(stringResource(if (form.file == null) R.string.upload_pick else R.string.upload_pick_other))
        }
        if (form.checkingFile) CircularProgressIndicator()
        form.fileError?.let { ErrorMessageText(it) }
        form.file?.let { file ->
            val preview = stringResource(R.string.upload_preview)
            LocalVideoPreview(
                uri = file.uri,
                modifier = Modifier.fillMaxWidth().height(320.dp).semantics { contentDescription = preview },
            )
            Text(
                listOfNotNull(
                    Formats.megabytes(file.sizeBytes),
                    file.durationMs?.let { Formats.duration(it) },
                ).joinToString(" · "),
                style = MaterialTheme.typography.bodyMedium,
            )
        }
        OutlinedTextField(
            value = form.title,
            onValueChange = onTitleChange,
            label = { Text(stringResource(R.string.upload_video_title)) },
            supportingText = {
                Text(if (form.titleHasForbidden) stringResource(R.string.upload_forbidden_chars) else "${form.titleLength} / ${UploadFormState.TITLE_MAX}")
            },
            isError = form.titleHasForbidden,
            singleLine = true,
            keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Sentences),
            modifier = Modifier.fillMaxWidth(),
        )
        OutlinedTextField(
            value = form.description,
            onValueChange = onDescriptionChange,
            label = { Text(stringResource(R.string.upload_video_description)) },
            supportingText = {
                Text(
                    if (form.descriptionHasForbidden) {
                        stringResource(R.string.upload_forbidden_chars)
                    } else {
                        stringResource(R.string.upload_description_count, form.descriptionLength, UploadFormState.DESCRIPTION_MAX)
                    },
                )
            },
            isError = form.descriptionHasForbidden,
            minLines = 3,
            keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Sentences),
            modifier = Modifier.fillMaxWidth(),
        )
        Button(onClick = onPublish, enabled = form.canPublish, colors = brandButtonColors(), modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp)) {
            Text(stringResource(R.string.upload_publish))
        }
    }
}

/** Progression de l'envoi, puis du traitement, puis le résultat. */
@Composable
private fun UploadStatusCard(
    upload: UploadState,
    onCancel: () -> Unit,
    onDismiss: () -> Unit,
    onOpenVideo: (String) -> Unit,
    onRetry: (() -> Unit)?,
    onRepick: () -> Unit,
) {
    if (upload == UploadState.Idle) return
    Card(Modifier.fillMaxWidth()) {
        Column(
            Modifier.padding(16.dp).semantics { liveRegion = LiveRegionMode.Polite },
            verticalArrangement = Arrangement.spacedBy(8.dp),
        ) {
            when (upload) {
                is UploadState.Uploading -> {
                    val percent = (upload.progress * 100).toInt()
                    Text(
                        if (upload.title.isBlank()) {
                            stringResource(R.string.upload_sending_untitled, percent)
                        } else {
                            stringResource(R.string.upload_sending, upload.title, percent)
                        },
                    )
                    LinearProgressIndicator(progress = { upload.progress }, modifier = Modifier.fillMaxWidth())
                    Text(stringResource(R.string.upload_keep_open), style = MaterialTheme.typography.bodySmall)
                    TextButton(onClick = onCancel) { Text(stringResource(R.string.action_cancel)) }
                }
                is UploadState.Processing -> {
                    Text(stringResource(if (upload.video.isPhoto) R.string.upload_photo_processing else R.string.video_processing))
                    LinearProgressIndicator(modifier = Modifier.fillMaxWidth())
                    TextButton(onClick = onDismiss) { Text(stringResource(R.string.upload_new)) }
                }
                is UploadState.Finished -> {
                    val ready = upload.video.status == VideoDto.STATUS_READY
                    val done = when {
                        !ready -> R.string.video_failed
                        upload.video.isPhoto -> R.string.upload_photo_ready
                        else -> R.string.upload_ready
                    }
                    Text(stringResource(done))
                    if (ready) TextButton(onClick = { onOpenVideo(upload.video.id) }) { Text(stringResource(R.string.upload_watch)) }
                    TextButton(onClick = onDismiss) { Text(stringResource(R.string.upload_new)) }
                }
                is UploadState.Failed -> {
                    upload.failure.toErrorMessage()?.let { ErrorMessageText(it) }
                    when {
                        upload.mediaLost -> TextButton(onClick = onRepick) { Text(stringResource(R.string.upload_repick)) }
                        onRetry != null -> TextButton(onClick = onRetry) { Text(stringResource(R.string.action_retry)) }
                    }
                    // Retour au formulaire, conservé, pour corriger le titre, la description ou la vidéo.
                    TextButton(onClick = onDismiss) { Text(stringResource(R.string.upload_edit)) }
                }
                UploadState.Idle -> Unit
            }
        }
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun ModeSelector(mode: PublishMode, enabled: Boolean, onModeChange: (PublishMode) -> Unit) {
    SingleChoiceSegmentedButtonRow(Modifier.fillMaxWidth()) {
        PublishMode.entries.forEachIndexed { index, option ->
            SegmentedButton(
                selected = mode == option,
                onClick = { onModeChange(option) },
                enabled = enabled,
                shape = SegmentedButtonDefaults.itemShape(index, PublishMode.entries.size),
                modifier = Modifier.heightIn(min = 48.dp),
            ) {
                Text(stringResource(if (option == PublishMode.Video) R.string.upload_mode_video else R.string.upload_mode_photo))
            }
        }
    }
}

/** Publication photo : une à dix photos, dans l'ordre choisi, et une légende facultative. */
@Composable
private fun PhotoForm(
    form: UploadFormState,
    onPick: () -> Unit,
    onRemove: (Int) -> Unit,
    onMove: (Int, Int) -> Unit,
    onCaptionChange: (String) -> Unit,
    onPublish: () -> Unit,
) {
    Text(stringResource(R.string.upload_photo_rules), style = MaterialTheme.typography.bodyMedium)
    OutlinedButton(
        onClick = onPick,
        enabled = !form.preparing && form.photos.size < UploadFormState.MAX_PHOTOS,
        modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp),
    ) {
        Text(stringResource(if (form.photos.isEmpty()) R.string.upload_photo_pick else R.string.upload_photo_add))
    }
    if (form.photos.isNotEmpty()) {
        Text(stringResource(R.string.upload_photo_count, form.photos.size), style = MaterialTheme.typography.bodyMedium)
        LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
            itemsIndexed(form.photos, key = { _, uri -> uri }) { index, uri ->
                PhotoTile(uri, index, form.photos.size, enabled = !form.preparing, onRemove = onRemove, onMove = onMove)
            }
        }
    }
    form.photoError?.let { ErrorMessageText(it) }
    OutlinedTextField(
        value = form.caption,
        onValueChange = onCaptionChange,
        label = { Text(stringResource(R.string.upload_photo_caption)) },
        supportingText = {
            Text(
                if (form.captionHasForbidden) {
                    stringResource(R.string.upload_forbidden_chars)
                } else {
                    stringResource(R.string.upload_description_count, form.captionLength, UploadFormState.DESCRIPTION_MAX)
                },
            )
        },
        isError = form.captionHasForbidden,
        minLines = 3,
        enabled = !form.preparing,
        keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Sentences),
        modifier = Modifier.fillMaxWidth(),
    )
    if (form.preparing) {
        val preparing = stringResource(R.string.upload_photo_preparing)
        Text(preparing, modifier = Modifier.semantics { liveRegion = LiveRegionMode.Polite })
        LinearProgressIndicator(Modifier.fillMaxWidth())
    }
    Button(onClick = onPublish, enabled = form.canPublish, colors = brandButtonColors(), modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp)) {
        Text(stringResource(R.string.upload_publish))
    }
}

/** Une photo du carrousel en préparation : aperçu, position, et boutons pour la déplacer ou la retirer. */
@Composable
private fun PhotoTile(uri: String, index: Int, count: Int, enabled: Boolean, onRemove: (Int) -> Unit, onMove: (Int, Int) -> Unit) {
    val position = index + 1
    val label = stringResource(R.string.upload_photo_item, position, count)
    Column(horizontalAlignment = Alignment.CenterHorizontally) {
        AsyncImage(
            model = uri,
            contentDescription = label,
            contentScale = ContentScale.Crop,
            modifier = Modifier.size(width = 120.dp, height = 160.dp).clip(RoundedCornerShape(8.dp)),
        )
        Row {
            IconButton(onClick = { onMove(index, index - 1) }, enabled = enabled && index > 0) {
                Icon(Icons.AutoMirrored.Filled.KeyboardArrowLeft, contentDescription = stringResource(R.string.upload_photo_move_left, position))
            }
            IconButton(onClick = { onRemove(index) }, enabled = enabled) {
                Icon(Icons.Filled.Close, contentDescription = stringResource(R.string.upload_photo_remove, position))
            }
            IconButton(onClick = { onMove(index, index + 1) }, enabled = enabled && index < count - 1) {
                Icon(Icons.AutoMirrored.Filled.KeyboardArrowRight, contentDescription = stringResource(R.string.upload_photo_move_right, position))
            }
        }
    }
}
