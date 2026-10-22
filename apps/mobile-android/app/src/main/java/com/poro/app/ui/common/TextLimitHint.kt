package com.poro.app.ui.common

import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.res.stringResource
import com.poro.app.R

/** Sous un champ multiligne : « Facultatif », ou les limites du serveur quand le texte les dépasse. */
@Composable
fun TextLimitHint(fits: Boolean, max: Int, maxLines: Int) {
    Text(if (fits) stringResource(R.string.optional) else stringResource(R.string.text_limit, max, maxLines))
}
