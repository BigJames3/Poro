package com.poro.app.ui.moderation

import androidx.compose.foundation.layout.Box
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.res.stringResource
import com.poro.app.R
import com.poro.app.ui.common.PoroIcons

data class OverflowAction(val label: String, val onClick: () -> Unit)

/** Menu ⋮ (48 dp) : une action « Signaler » aujourd'hui, d'autres plus tard sans changer les accroches. */
@Composable
fun OverflowMenu(
    items: List<OverflowAction>,
    modifier: Modifier = Modifier,
    tint: Color = Color.Unspecified,
    contentDescription: String = stringResource(R.string.report_menu),
) {
    var expanded by rememberSaveable { mutableStateOf(false) }
    Box(modifier) {
        IconButton(onClick = { expanded = true }) {
            Icon(PoroIcons.MoreVert, contentDescription = contentDescription, tint = tint)
        }
        DropdownMenu(expanded = expanded, onDismissRequest = { expanded = false }) {
            items.forEach { item ->
                DropdownMenuItem(
                    text = { Text(item.label) },
                    onClick = {
                        expanded = false
                        item.onClick()
                    },
                )
            }
        }
    }
}
