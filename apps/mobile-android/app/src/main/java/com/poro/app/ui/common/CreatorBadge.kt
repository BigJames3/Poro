package com.poro.app.ui.common

import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import com.poro.app.R
import com.poro.app.ui.theme.PoroSize
import com.poro.app.ui.theme.PoroSpace

/** Badge non interactif : ne pas le rendre focusable. */
@Composable
fun CreatorBadge(modifier: Modifier = Modifier) {
    Surface(
        modifier = modifier,
        color = MaterialTheme.colorScheme.tertiaryContainer,
        contentColor = MaterialTheme.colorScheme.onTertiaryContainer,
        shape = RoundedCornerShape(PoroSize.radiusCard),
    ) {
        Text(
            stringResource(R.string.profile_creator_badge),
            style = MaterialTheme.typography.labelLarge,
            modifier = Modifier.padding(horizontal = PoroSpace.sm, vertical = PoroSpace.xxs),
        )
    }
}
