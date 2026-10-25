package com.poro.app.ui.theme

import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.ButtonColors
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.darkColorScheme
import androidx.compose.material3.lightColorScheme
import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.ui.graphics.Color

/*
 * Palette Poro (décision DS1 : texte foncé sur l'orange). Contrastes WCAG 2.x vérifiés, AA :
 * - l'orange de marque ne porte jamais de texte blanc : boutons pleins en orange avec texte
 *   foncé (6,7:1), via primaryContainer ([brandButtonColors]) ;
 * - sur fond clair, l'orange de marque est trop pâle pour du texte (2,8:1) : textes, liens,
 *   icônes et contours utilisent l'orange foncé de `primary` (5,1:1 au pire, sur surfaceVariant).
 * docs/product/mobile-ux-phase-2/06-design-system-extension.md
 */
private val PoroOrange = Color(0xFFFF6A00)
private val PoroOrangeText = Color(0xFFA84100)
private val PoroOrangeDark = Color(0xFFFF8A3D)
private val InkOnOrange = Color(0xFF1A0A00)
private val Night = Color(0xFF121212)

internal val LightColors = lightColorScheme(
    primary = PoroOrangeText,
    onPrimary = Color.White,
    primaryContainer = PoroOrange,
    onPrimaryContainer = InkOnOrange,
    secondary = Color(0xFF1B7F5A),
    onSecondary = Color.White,
    // Vert de marque très clair (cartes d'information) : la valeur par défaut de Material est lavande.
    secondaryContainer = Color(0xFFD3F2E3),
    onSecondaryContainer = Color(0xFF00281A),
    background = Color(0xFFFFFBF8),
    surface = Color.White,
    // Gris chauds accordés à l'orange (les valeurs par défaut de Material tirent sur le violet).
    surfaceVariant = Color(0xFFF2E8E2),
    onSurfaceVariant = Color(0xFF52443C),
    // Ocre (badges, mises en avant secondaires) et contours chauds, au lieu des violets de Material.
    tertiary = Color(0xFF7A5900),
    onTertiary = Color.White,
    tertiaryContainer = Color(0xFFFFDF9E),
    onTertiaryContainer = Color(0xFF261A00),
    error = Color(0xFFB3261E),
    onError = Color.White,
    errorContainer = Color(0xFFF9DEDC),
    onErrorContainer = Color(0xFF410E0B),
    outline = Color(0xFF85736A),
    outlineVariant = Color(0xFFD7C2B8),
)

internal val DarkColors = darkColorScheme(
    primary = PoroOrangeDark,
    onPrimary = Color.Black,
    primaryContainer = PoroOrangeDark,
    onPrimaryContainer = InkOnOrange,
    secondary = Color(0xFF4CC497),
    onSecondary = Color.Black,
    secondaryContainer = Color(0xFF0F4D36),
    onSecondaryContainer = Color(0xFFC6F2DF),
    background = Night,
    surface = Color(0xFF1E1E1E),
    surfaceVariant = Color(0xFF2B2B2B),
    onSurfaceVariant = Color(0xFFD8C2B8),
    tertiary = Color(0xFFF0C048),
    onTertiary = Color(0xFF3F2E00),
    tertiaryContainer = Color(0xFF5C4300),
    onTertiaryContainer = Color(0xFFFFDF9E),
    error = Color(0xFFF2B8B5),
    onError = Color(0xFF601410),
    errorContainer = Color(0xFF8C1D18),
    onErrorContainer = Color(0xFFF9DEDC),
    outline = Color(0xFFA08D84),
    outlineVariant = Color(0xFF52443C),
)

@Composable
fun PoroTheme(darkTheme: Boolean = isSystemInDarkTheme(), content: @Composable () -> Unit) {
    CompositionLocalProvider(LocalPoroExtraColors provides PoroExtras) {
        MaterialTheme(
            colorScheme = if (darkTheme) DarkColors else LightColors,
            typography = PoroTypography,
            shapes = PoroShapes,
            content = content,
        )
    }
}

/** Bouton plein de marque : orange avec texte foncé, en clair comme en sombre. */
@Composable
fun brandButtonColors(): ButtonColors = ButtonDefaults.buttonColors(
    containerColor = MaterialTheme.colorScheme.primaryContainer,
    contentColor = MaterialTheme.colorScheme.onPrimaryContainer,
)
