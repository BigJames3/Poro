package com.poro.app.ui.upload

import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.PickVisualMediaRequest
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.LiveRegionMode
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.liveRegion
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.core.account.TextLimits
import com.poro.app.core.format.Formats
import com.poro.app.data.video.UploadState
import com.poro.app.data.video.VideoDto
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.auth.toErrorMessage

@Composable
fun UploadScreen(
    form: UploadFormState,
    upload: UploadState,
    onVideoPicked: (String) -> Unit,
    onTitleChange: (String) -> Unit,
    onDescriptionChange: (String) -> Unit,
    onPublish: () -> Unit,
    onCancelUpload: () -> Unit,
    onDismissResult: () -> Unit,
    onOpenVideo: (String) -> Unit,
) {
    val picker = rememberLauncherForActivityResult(ActivityResultContracts.PickVisualMedia()) { uri ->
        uri?.let { onVideoPicked(it.toString()) }
    }
    Column(
        modifier = Modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(24.dp),
        verticalArrangement = Arrangement.spacedBy(16.dp),
    ) {
        Text(
            stringResource(R.string.upload_title),
            style = MaterialTheme.typography.headlineMedium,
            modifier = Modifier.semantics { heading() },
        )
        UploadStatusCard(upload, onCancelUpload, onDismissResult, onOpenVideo)
        if (upload is UploadState.Uploading) return@Column

        Text(stringResource(R.string.upload_rules), style = MaterialTheme.typography.bodyMedium)
        OutlinedButton(
            onClick = { picker.launch(PickVisualMediaRequest(ActivityResultContracts.PickVisualMedia.VideoOnly)) },
            enabled = !form.checkingFile,
            modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp),
        ) {
            Text(stringResource(if (form.file == null) R.string.upload_pick else R.string.upload_pick_other))
        }
        if (form.checkingFile) CircularProgressIndicator()
        form.fileError?.let { ErrorMessageText(it) }
        form.file?.let { file ->
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
            supportingText = { Text("${form.title.length} / ${UploadFormState.TITLE_MAX}") },
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
                    stringResource(R.string.upload_hashtags_hint) + " · " +
                        stringResource(R.string.upload_description_counter, TextLimits.runeCount(form.description), UploadFormState.DESCRIPTION_MAX),
                )
            },
            minLines = 3,
            keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Sentences),
            modifier = Modifier.fillMaxWidth(),
        )
        Button(onClick = onPublish, enabled = form.canPublish, modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp)) {
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
                    Text(stringResource(R.string.upload_sending, upload.title, percent))
                    LinearProgressIndicator(progress = { upload.progress }, modifier = Modifier.fillMaxWidth())
                    Text(stringResource(R.string.upload_keep_open), style = MaterialTheme.typography.bodySmall)
                    TextButton(onClick = onCancel) { Text(stringResource(R.string.action_cancel)) }
                }
                is UploadState.Processing -> {
                    Text(stringResource(R.string.video_processing))
                    LinearProgressIndicator(modifier = Modifier.fillMaxWidth())
                    TextButton(onClick = onDismiss) { Text(stringResource(R.string.upload_new)) }
                }
                is UploadState.Finished -> {
                    val ready = upload.video.status == VideoDto.STATUS_READY
                    Text(stringResource(if (ready) R.string.upload_ready else R.string.video_failed))
                    if (ready) TextButton(onClick = { onOpenVideo(upload.video.id) }) { Text(stringResource(R.string.upload_watch)) }
                    TextButton(onClick = onDismiss) { Text(stringResource(R.string.upload_new)) }
                }
                is UploadState.Failed -> {
                    upload.failure.toErrorMessage()?.let { ErrorMessageText(it) }
                    TextButton(onClick = onDismiss) { Text(stringResource(R.string.action_retry)) }
                }
                UploadState.Idle -> Unit
            }
        }
    }
}
