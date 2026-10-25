package com.poro.app.ui.theme

import androidx.compose.material3.ColorScheme
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.luminance
import org.junit.Assert.assertTrue
import org.junit.Test

/** Paires texte/fond de PoroTheme au niveau WCAG AA (4,5:1 pour du texte normal). */
class ThemeContrastTest {

    private fun contrast(a: Color, b: Color): Double {
        val (light, dark) = listOf(a.luminance(), b.luminance()).sortedDescending()
        return (light + 0.05) / (dark + 0.05)
    }

    private fun ColorScheme.pairs() = mapOf(
        "onPrimary/primary" to (onPrimary to primary),
        "onPrimaryContainer/primaryContainer (bouton de marque)" to (onPrimaryContainer to primaryContainer),
        "primary/background (liens, boutons texte)" to (primary to background),
        "primary/surface" to (primary to surface),
        "primary/surfaceVariant" to (primary to surfaceVariant),
        "onSurfaceVariant/surfaceVariant (textes secondaires)" to (onSurfaceVariant to surfaceVariant),
        "onSurfaceVariant/surface" to (onSurfaceVariant to surface),
        "onSecondary/secondary" to (onSecondary to secondary),
        "onSecondaryContainer/secondaryContainer (cartes d'information)" to (onSecondaryContainer to secondaryContainer),
        "secondary/background" to (secondary to background),
        "onBackground/background" to (onBackground to background),
        "onSurface/surface" to (onSurface to surface),
        "error/surface" to (error to surface),
        "onError/error" to (onError to error),
        "onErrorContainer/errorContainer" to (onErrorContainer to errorContainer),
        "tertiary/surface" to (tertiary to surface),
        "onTertiary/tertiary" to (onTertiary to tertiary),
        "onTertiaryContainer/tertiaryContainer" to (onTertiaryContainer to tertiaryContainer),
    )

    /** Contours et icônes : 3:1 (WCAG 1.4.11, composants d'interface). */
    private fun ColorScheme.componentPairs() = mapOf(
        "outline/surface (champs, cases du code)" to (outline to surface),
        "outline/background" to (outline to background),
    )

    @Test
    fun `light and dark themes meet AA for text`() {
        for ((name, scheme) in listOf("clair" to LightColors, "sombre" to DarkColors)) {
            for ((pair, colors) in scheme.pairs()) {
                val ratio = contrast(colors.first, colors.second)
                assertTrue("$name, $pair : ${"%.2f".format(ratio)}:1 < 4,5:1", ratio >= 4.5)
            }
            for ((pair, colors) in scheme.componentPairs()) {
                val ratio = contrast(colors.first, colors.second)
                assertTrue("$name, $pair : ${"%.2f".format(ratio)}:1 < 3:1", ratio >= 3.0)
            }
        }
    }

    @Test
    fun `the like red stands out on a darkened video`() {
        val darkenedVideo = Color(0xFF333333)
        val ratio = contrast(PoroExtras.like, darkenedVideo)
        assertTrue("${"%.2f".format(ratio)}:1 < 3:1", ratio >= 3.0)
    }
}
