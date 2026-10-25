package com.poro.app.ui.social

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.foundation.clickable
import com.poro.app.R
import com.poro.app.data.social.ShareChannel
import com.poro.app.ui.theme.PoroSpacing

/** WhatsApp en premier (premier canal de partage dans les pays de lancement), puis les autres. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ShareSheet(onChoose: (ShareChannel) -> Unit, onDismiss: () -> Unit) {
    ModalBottomSheet(onDismissRequest = onDismiss) {
        Column(Modifier.padding(bottom = PoroSpacing.XL)) {
            Text(
                stringResource(R.string.share_title),
                style = MaterialTheme.typography.titleMedium,
                modifier = Modifier.padding(PoroSpacing.L).semantics { heading() },
            )
            ShareOption(stringResource(R.string.share_whatsapp)) { onChoose(ShareChannel.WhatsApp) }
            ShareOption(stringResource(R.string.share_other)) { onChoose(ShareChannel.Other) }
        }
    }
}

@Composable
private fun ShareOption(label: String, onClick: () -> Unit) {
    ListItem(
        headlineContent = { Text(label) },
        modifier = Modifier.fillMaxWidth().clickable(role = Role.Button, onClick = onClick),
    )
}
