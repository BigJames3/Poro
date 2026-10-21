package com.poro.app.ui.legal

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.ui.common.BackTopBar

@Composable
fun LegalScreen(onBack: () -> Unit, onOpen: (LegalDocument) -> Unit) {
    Scaffold(topBar = { BackTopBar(stringResource(R.string.legal_title), onBack) }) { padding ->
        LazyColumn(Modifier.fillMaxSize().padding(padding)) {
            items(LegalDocument.entries, key = { it.id }) { document ->
                ListItem(
                    headlineContent = { Text(stringResource(document.title)) },
                    modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp).clickable(role = Role.Button) { onOpen(document) },
                )
            }
        }
    }
}

@Composable
fun LegalDocumentScreen(id: String, onBack: () -> Unit) {
    val document = LegalDocument.fromId(id)
    val title = stringResource(document?.title ?: R.string.legal_title)
    Scaffold(topBar = { BackTopBar(title, onBack) }) { padding ->
        LazyColumn(Modifier.fillMaxSize().padding(padding).padding(24.dp)) {
            item {
                Text(stringResource(R.string.legal_unavailable), style = MaterialTheme.typography.bodyLarge)
            }
            item {
                Text(
                    stringResource(R.string.legal_unavailable_notice),
                    style = MaterialTheme.typography.bodyMedium,
                    modifier = Modifier.padding(top = 12.dp),
                )
            }
        }
    }
}
