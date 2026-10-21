package com.poro.app.ui.common

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.Dp
import coil3.compose.AsyncImage
import com.poro.app.R
import com.poro.app.ui.theme.PoroSize

/** Photo de profil ronde ; à défaut, l'initiale du nom sur fond de couleur. Décorative pour TalkBack. */
@Composable
fun Avatar(url: String?, name: String?, modifier: Modifier = Modifier, size: Dp = PoroSize.avatarM) {
    Box(
        modifier = modifier.size(size).clip(CircleShape).background(MaterialTheme.colorScheme.primaryContainer),
        contentAlignment = Alignment.Center,
    ) {
        if (url != null) {
            AsyncImage(model = url, contentDescription = null, contentScale = ContentScale.Crop, modifier = Modifier.size(size))
        } else {
            Text(
                text = name?.trim()?.firstOrNull()?.uppercase() ?: "?",
                style = if (size >= PoroSize.avatarL) MaterialTheme.typography.headlineMedium else MaterialTheme.typography.titleMedium,
                color = MaterialTheme.colorScheme.onPrimaryContainer,
            )
        }
    }
}

/** Barre de titre d'un écran secondaire, avec retour. */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun BackTopBar(title: String, onBack: () -> Unit, actions: @Composable () -> Unit = {}) {
    TopAppBar(
        title = { Text(title, modifier = Modifier.semantics { heading() }) },
        navigationIcon = {
            IconButton(onClick = onBack) {
                Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = stringResource(R.string.action_back))
            }
        },
        actions = { actions() },
    )
}

/** Nom à afficher d'un compte : nom affiché, sinon @pseudo, sinon un libellé neutre. */
@Composable
fun displayNameOf(displayName: String?, username: String?): String =
    displayName?.takeIf { it.isNotBlank() } ?: username?.let { "@$it" } ?: stringResource(R.string.member_fallback)
