package com.poro.app.ui.profile

import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.PickVisualMediaRequest
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardCapitalization
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.core.account.AccountRules
import com.poro.app.ui.auth.ErrorMessageText
import com.poro.app.ui.auth.SubmitButton
import com.poro.app.ui.common.Avatar
import com.poro.app.ui.common.BackTopBar

@Composable
fun EditProfileScreen(
    state: EditProfileUiState,
    onBack: () -> Unit,
    onUsernameChange: (String) -> Unit,
    onDisplayNameChange: (String) -> Unit,
    onBioChange: (String) -> Unit,
    onAvatarPicked: (String) -> Unit,
    onRemoveAvatar: () -> Unit,
    onSave: () -> Unit,
) {
    // Sélecteur de photos du système : aucune permission de stockage à demander.
    val picker = rememberLauncherForActivityResult(ActivityResultContracts.PickVisualMedia()) { uri ->
        uri?.let { onAvatarPicked(it.toString()) }
    }
    Scaffold(topBar = { BackTopBar(stringResource(R.string.edit_profile_title), onBack) }) { padding ->
        if (state.loading) {
            Box(Modifier.fillMaxSize().padding(padding), contentAlignment = Alignment.Center) { CircularProgressIndicator() }
            return@Scaffold
        }
        Column(
            modifier = Modifier.fillMaxSize().padding(padding).verticalScroll(rememberScrollState()).padding(24.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp),
        ) {
            Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(16.dp)) {
                Avatar(state.avatarUrl, state.displayName.ifBlank { state.username }, size = 80.dp)
                Column {
                    if (state.uploadingAvatar) {
                        CircularProgressIndicator()
                    } else {
                        TextButton(onClick = { picker.launch(PickVisualMediaRequest(ActivityResultContracts.PickVisualMedia.ImageOnly)) }) {
                            Text(stringResource(R.string.avatar_change))
                        }
                        if (state.avatarUrl != null) {
                            TextButton(onClick = onRemoveAvatar) { Text(stringResource(R.string.avatar_remove)) }
                        }
                    }
                }
            }
            val usernameHint = state.usernameStatus.hint?.let { stringResource(it) } ?: stringResource(R.string.username_rules)
            OutlinedTextField(
                value = state.username,
                onValueChange = onUsernameChange,
                label = { Text(stringResource(R.string.username_label)) },
                prefix = { Text("@") },
                singleLine = true,
                isError = state.usernameStatus in setOf(UsernameStatus.Invalid, UsernameStatus.Reserved, UsernameStatus.Taken),
                supportingText = { Text(usernameHint) },
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Ascii, imeAction = ImeAction.Next),
                modifier = Modifier.fillMaxWidth(),
            )
            OutlinedTextField(
                value = state.displayName,
                onValueChange = onDisplayNameChange,
                label = { Text(stringResource(R.string.display_name_label)) },
                singleLine = true,
                keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Words, imeAction = ImeAction.Next),
                modifier = Modifier.fillMaxWidth(),
            )
            OutlinedTextField(
                value = state.bio,
                onValueChange = onBioChange,
                label = { Text(stringResource(R.string.bio_label)) },
                supportingText = { Text("${state.bio.length} / ${AccountRules.BIO_MAX}") },
                minLines = 3,
                keyboardOptions = KeyboardOptions(capitalization = KeyboardCapitalization.Sentences),
                modifier = Modifier.fillMaxWidth(),
            )
            state.error?.let { ErrorMessageText(it) }
            SubmitButton(
                text = stringResource(R.string.action_save),
                enabled = state.canSave,
                loading = state.saving,
                onClick = onSave,
            )
        }
    }
}
