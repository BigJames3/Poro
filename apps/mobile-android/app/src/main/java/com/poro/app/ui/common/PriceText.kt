package com.poro.app.ui.common

import androidx.compose.material3.LocalTextStyle
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.TextStyle
import com.poro.app.R
import com.poro.app.core.format.Formats

/** Prix toujours via [Formats.money] : jamais un float interpolé. */
@Composable
fun PriceText(
    amount: Long,
    currency: String,
    modifier: Modifier = Modifier,
    from: Boolean = false,
    style: TextStyle = LocalTextStyle.current,
) {
    val formatted = Formats.money(amount, currency)
    Text(
        text = if (from) stringResource(R.string.price_from, formatted) else formatted,
        modifier = modifier,
        style = style,
    )
}
