package com.poro.app.ui.common

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.material3.MaterialTheme
import androidx.compose.ui.text.style.TextAlign
import com.poro.app.ui.theme.PoroSize
import com.poro.app.ui.theme.PoroSpace

@Composable
fun EmptyState(
    title: String,
    modifier: Modifier = Modifier,
    body: String? = null,
    actionLabel: String? = null,
    onAction: (() -> Unit)? = null,
    color: Color = MaterialTheme.colorScheme.onSurface,
) {
    Column(
        modifier = modifier.fillMaxWidth().padding(PoroSpace.xl),
        verticalArrangement = Arrangement.spacedBy(PoroSpace.md),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(
            title,
            style = MaterialTheme.typography.titleMedium,
            color = color,
            textAlign = TextAlign.Center,
            modifier = Modifier.semantics { heading() },
        )
        body?.let {
            Text(it, style = MaterialTheme.typography.bodyMedium, color = color.copy(alpha = 0.85f), textAlign = TextAlign.Center)
        }
        if (actionLabel != null && onAction != null) {
            OutlinedButton(onClick = onAction, modifier = Modifier.heightIn(min = PoroSize.touch)) {
                Text(actionLabel)
            }
        }
    }
}
