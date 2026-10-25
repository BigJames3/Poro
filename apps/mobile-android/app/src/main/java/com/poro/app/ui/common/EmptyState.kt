package com.poro.app.ui.common

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.unit.dp
import com.poro.app.ui.theme.PoroSpacing

/**
 * Liste ou écran vide : un message centré, une icône facultative (décorative) et, au besoin,
 * l'action qui permet d'en sortir. [color] change la teinte sur un fond de média.
 */
@Composable
fun EmptyState(
    message: String,
    modifier: Modifier = Modifier,
    icon: ImageVector? = null,
    color: Color = MaterialTheme.colorScheme.onSurfaceVariant,
    action: (@Composable () -> Unit)? = null,
) {
    Column(
        modifier.fillMaxWidth().padding(PoroSpacing.XL),
        verticalArrangement = Arrangement.spacedBy(PoroSpacing.M),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        icon?.let { Icon(it, contentDescription = null, tint = color, modifier = Modifier.size(48.dp)) }
        Text(message, color = color, textAlign = TextAlign.Center, style = MaterialTheme.typography.bodyLarge)
        action?.invoke()
    }
}
