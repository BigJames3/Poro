package com.poro.app.ui.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.ui.graphics.Color

// Palette provisoire, à remplacer par la charte graphique de Poro.
private val PoroOrange = Color(0xFFFF6A00)
private val PoroOrangeDark = Color(0xFFFF8A3D)
private val Night = Color(0xFF121212)

private val LightColors = lightColorScheme(
    primary = PoroOrange,
    onPrimary = Color.White,
    secondary = Color(0xFF1B7F5A),
    background = Color(0xFFFFFBF8),
    surface = Color.White,
)

private val DarkColors = darkColorScheme(
    primary = PoroOrangeDark,
    onPrimary = Color.Black,
    secondary = Color(0xFF4CC497),
    background = Night,
    surface = Color(0xFF1E1E1E),
)

@Composable
fun PoroTheme(darkTheme: Boolean = isSystemInDarkTheme(), content: @Composable () -> Unit) {
    MaterialTheme(
        colorScheme = if (darkTheme) DarkColors else LightColors,
        content = content,
    )
}
