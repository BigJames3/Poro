package com.poro.app.ui.theme

import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Shapes
import androidx.compose.material3.Typography
import androidx.compose.runtime.Composable
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp

/** Espacements de Poro : toute marge ou tout écart vient de cette échelle. */
object PoroSpacing {
    val XS = 4.dp
    val S = 8.dp
    val M = 12.dp
    val L = 16.dp
    val XL = 24.dp
    val XXL = 32.dp

    /** Largeur maximale d'un formulaire ou d'une page de texte, lisible sur tablette. */
    val ContentMaxWidth = 600.dp
}

/** Couleurs propres à Poro, hors des rôles Material. */
@Immutable
data class PoroExtraColors(
    /** « J'aime » actif, posé sur une vidéo : 3,6:1 au moins sur l'image assombrie. */
    val like: Color,
    /** Texte et icônes posés sur une vidéo ou une photo. */
    val onMedia: Color,
    /** Voile sous un texte posé sur un média. */
    val mediaScrim: Color,
)

internal val PoroExtras = PoroExtraColors(
    like = Color(0xFFFF3B5C),
    onMedia = Color.White,
    mediaScrim = Color.Black.copy(alpha = 0.6f),
)

internal val LocalPoroExtraColors = staticCompositionLocalOf { PoroExtras }

/** Couleurs propres à Poro du thème courant. */
val MaterialTheme.extraColors: PoroExtraColors
    @Composable
    @ReadOnlyComposable
    get() = LocalPoroExtraColors.current

/** Échelle typographique Material, titres un peu plus appuyés pour les écrans de contenu. */
internal val PoroTypography = Typography().run {
    copy(
        headlineSmall = headlineSmall.copy(fontWeight = FontWeight.SemiBold),
        titleLarge = titleLarge.copy(fontWeight = FontWeight.SemiBold),
        titleMedium = titleMedium.copy(fontWeight = FontWeight.SemiBold),
        labelLarge = labelLarge.copy(fontWeight = FontWeight.SemiBold),
    )
}

internal val PoroShapes = Shapes(
    extraSmall = RoundedCornerShape(4.dp),
    small = RoundedCornerShape(8.dp),
    medium = RoundedCornerShape(12.dp),
    large = RoundedCornerShape(16.dp),
    extraLarge = RoundedCornerShape(28.dp),
)
