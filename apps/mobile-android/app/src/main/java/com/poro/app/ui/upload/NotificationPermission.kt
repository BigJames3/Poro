package com.poro.app.ui.upload

import android.Manifest
import android.content.pm.PackageManager
import android.os.Build
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.ui.platform.LocalContext
import androidx.core.content.ContextCompat

/**
 * Avant un envoi : Android 13+ demande l'autorisation d'afficher la notification de progression
 * (l'envoi continue app fermée). La réponse, quelle qu'elle soit, lance [then] : un refus ne bloque
 * jamais la publication, la notification ne s'affiche simplement pas.
 */
@Composable
fun rememberNotificationPermission(then: () -> Unit): () -> Unit {
    val context = LocalContext.current
    val currentThen by rememberUpdatedState(then)
    val launcher = rememberLauncherForActivityResult(ActivityResultContracts.RequestPermission()) { currentThen() }
    return remember(launcher) {
        {
            val needed = Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
                ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
            if (needed) launcher.launch(Manifest.permission.POST_NOTIFICATIONS) else currentThen()
        }
    }
}
