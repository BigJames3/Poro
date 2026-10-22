package com.poro.app.ui.legal

import androidx.annotation.StringRes
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.selection.SelectionContainer
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import com.poro.app.R
import com.poro.app.core.format.Formats
import com.poro.app.core.legal.LegalBlock
import com.poro.app.core.legal.LegalDocument
import com.poro.app.core.legal.LegalDocumentId
import com.poro.app.ui.common.BackTopBar

@StringRes
fun LegalDocumentId.titleRes(): Int = when (this) {
    LegalDocumentId.Terms -> R.string.legal_terms
    LegalDocumentId.Privacy -> R.string.legal_privacy
    LegalDocumentId.Community -> R.string.legal_community
    LegalDocumentId.Marketplace -> R.string.legal_marketplace
}

@Composable
fun LegalIndexScreen(entries: List<LegalEntry>?, onBack: () -> Unit, onOpen: (LegalDocumentId) -> Unit) {
    Scaffold(topBar = { BackTopBar(stringResource(R.string.legal_title), onBack) }) { padding ->
        if (entries == null) {
            Box(Modifier.fillMaxSize().padding(padding), contentAlignment = Alignment.Center) { CircularProgressIndicator() }
            return@Scaffold
        }
        Column(Modifier.fillMaxSize().padding(padding).verticalScroll(rememberScrollState())) {
            entries.forEach { entry ->
                val status = when {
                    entry.document == null -> R.string.legal_status_unavailable
                    entry.document.validated -> null
                    else -> R.string.legal_status_draft
                }
                ListItem(
                    headlineContent = { Text(stringResource(entry.id.titleRes())) },
                    supportingContent = status?.let { { Text(stringResource(it)) } },
                    modifier = Modifier.heightIn(min = 56.dp).clickable(role = Role.Button) { onOpen(entry.id) },
                )
                HorizontalDivider()
            }
        }
    }
}

@Composable
fun LegalDocumentScreen(id: LegalDocumentId, state: LegalDocumentState, onBack: () -> Unit) {
    Scaffold(topBar = { BackTopBar(stringResource(id.titleRes()), onBack) }) { padding ->
        when (state) {
            LegalDocumentState.Loading ->
                Box(Modifier.fillMaxSize().padding(padding), contentAlignment = Alignment.Center) { CircularProgressIndicator() }
            LegalDocumentState.Unavailable ->
                Text(stringResource(R.string.legal_unavailable), Modifier.padding(padding).padding(24.dp))
            is LegalDocumentState.Ready -> LegalDocumentContent(state.document, Modifier.padding(padding))
        }
    }
}

@Composable
private fun LegalDocumentContent(document: LegalDocument, modifier: Modifier = Modifier) {
    Column(modifier.fillMaxSize().verticalScroll(rememberScrollState()).padding(24.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
        // Texte de travail : le bandeau ne peut pas être masqué.
        if (!document.validated) DraftBanner()
        Text(document.title, style = MaterialTheme.typography.headlineSmall, modifier = Modifier.semantics { heading() })
        Text(
            stringResource(
                R.string.legal_version,
                document.version,
                document.effectiveDate?.let { Formats.date(it) } ?: stringResource(R.string.legal_effective_unknown),
            ),
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        SelectionContainer {
            Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
                document.blocks.forEach { block ->
                    when (block) {
                        is LegalBlock.Heading ->
                            Text(block.text, style = MaterialTheme.typography.titleMedium, modifier = Modifier.semantics { heading() })
                        is LegalBlock.Paragraph -> Text(block.text, style = MaterialTheme.typography.bodyLarge)
                        is LegalBlock.Notice -> Text(
                            block.text,
                            style = MaterialTheme.typography.bodyMedium,
                            modifier = Modifier.fillMaxWidth()
                                .background(MaterialTheme.colorScheme.surfaceVariant, RoundedCornerShape(8.dp))
                                .padding(12.dp),
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun DraftBanner() {
    Text(
        stringResource(R.string.legal_draft_banner),
        style = MaterialTheme.typography.bodyMedium,
        color = MaterialTheme.colorScheme.onErrorContainer,
        modifier = Modifier.fillMaxWidth()
            .background(MaterialTheme.colorScheme.errorContainer, RoundedCornerShape(8.dp))
            .padding(12.dp),
    )
}

/** Mention des écrans de connexion : information, sans case à cocher (décision L1). */
@Composable
fun LegalMention(onOpen: (LegalDocumentId) -> Unit, modifier: Modifier = Modifier) {
    Column(modifier.fillMaxWidth(), horizontalAlignment = Alignment.CenterHorizontally) {
        Text(
            stringResource(R.string.legal_mention),
            style = MaterialTheme.typography.bodySmall,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        FlowRow(horizontalArrangement = Arrangement.Center) {
            TextButton(onClick = { onOpen(LegalDocumentId.Terms) }) { Text(stringResource(R.string.legal_terms)) }
            TextButton(onClick = { onOpen(LegalDocumentId.Privacy) }) { Text(stringResource(R.string.legal_privacy)) }
        }
    }
}
