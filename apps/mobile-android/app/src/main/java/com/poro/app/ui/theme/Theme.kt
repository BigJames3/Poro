package com.poro.app.ui.theme

import android.app.Activity
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.ui.platform.LocalView
import androidx.core.view.WindowCompat

private val LightColors = lightColorScheme(
    primary = PoroOrange,
    onPrimary = androidx.compose.ui.graphics.Color.White,
    primaryContainer = PoroPrimaryContainerLight,
    onPrimaryContainer = PoroOnPrimaryContainerLight,
    secondary = PoroGreen,
    onSecondary = androidx.compose.ui.graphics.Color.White,
    secondaryContainer = PoroSecondaryContainerLight,
    onSecondaryContainer = PoroOnSecondaryContainerLight,
    tertiary = PoroTertiaryLight,
    onTertiary = PoroOnTertiaryLight,
    tertiaryContainer = PoroTertiaryContainerLight,
    onTertiaryContainer = PoroOnTertiaryContainerLight,
    error = PoroErrorLight,
    onError = PoroOnErrorLight,
    errorContainer = PoroErrorContainerLight,
    onErrorContainer = PoroOnErrorContainerLight,
    background = PoroBackgroundLight,
    onBackground = PoroOnPrimaryContainerLight,
    surface = androidx.compose.ui.graphics.Color.White,
    onSurface = PoroOnPrimaryContainerLight,
    surfaceVariant = PoroSurfaceVariantLight,
    onSurfaceVariant = PoroOnSurfaceVariantLight,
    outline = PoroOutlineLight,
    outlineVariant = PoroOutlineVariantLight,
)

private val DarkColors = darkColorScheme(
    primary = PoroOrangeDark,
    onPrimary = androidx.compose.ui.graphics.Color.Black,
    primaryContainer = PoroPrimaryContainerDark,
    onPrimaryContainer = PoroOnPrimaryContainerDark,
    secondary = PoroGreenDark,
    onSecondary = androidx.compose.ui.graphics.Color.Black,
    secondaryContainer = PoroSecondaryContainerDark,
    onSecondaryContainer = PoroOnSecondaryContainerDark,
    tertiary = PoroTertiaryDark,
    onTertiary = PoroOnTertiaryDark,
    tertiaryContainer = PoroTertiaryContainerDark,
    onTertiaryContainer = PoroOnTertiaryContainerDark,
    error = PoroErrorDark,
    onError = PoroOnErrorDark,
    errorContainer = PoroErrorContainerDark,
    onErrorContainer = PoroOnErrorContainerDark,
    background = PoroNight,
    onBackground = PoroOnPrimaryContainerDark,
    surface = PoroSurfaceDark,
    onSurface = PoroOnPrimaryContainerDark,
    surfaceVariant = PoroSurfaceVariantDark,
    onSurfaceVariant = PoroOnSurfaceVariantDark,
    outline = PoroOutlineDark,
    outlineVariant = PoroOutlineVariantDark,
)

@Composable
fun PoroTheme(darkTheme: Boolean = isSystemInDarkTheme(), content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = if (darkTheme) DarkColors else LightColors,
        content = content,
    )
}

/** Icônes de barre système claires : le fil est toujours un chrome sombre, thème clair ou non. */
@Composable
fun OverlaySystemBars() {
    val view = LocalView.current
    if (view.isInEditMode) return
    DisposableEffect(view) {
        val window = (view.context as Activity).window
        val controller = WindowCompat.getInsetsController(window, view)
        val wasLightStatus = controller.isAppearanceLightStatusBars
        val wasLightNav = controller.isAppearanceLightNavigationBars
        controller.isAppearanceLightStatusBars = false
        controller.isAppearanceLightNavigationBars = false
        onDispose {
            controller.isAppearanceLightStatusBars = wasLightStatus
            controller.isAppearanceLightNavigationBars = wasLightNav
        }
    }
}
