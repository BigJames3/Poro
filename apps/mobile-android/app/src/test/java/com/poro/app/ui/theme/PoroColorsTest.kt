package com.poro.app.ui.theme

import androidx.compose.ui.graphics.Color
import org.junit.Assert.assertEquals
import org.junit.Test

class PoroColorsTest {

    @Test
    fun `brand tokens match the published palette`() {
        assertEquals(Color(0xFFFF6A00), PoroOrange)
        assertEquals(Color(0xFFFF8A3D), PoroOrangeDark)
        assertEquals(Color(0xFF1B7F5A), PoroGreen)
        assertEquals(Color(0xFF4CC497), PoroGreenDark)
        assertEquals(Color(0xFFFF3B5C), PoroLike)
        assertEquals(Color(0xFFFFE4D1), PoroPrimaryContainerLight)
        assertEquals(Color(0xFFD4EFE4), PoroSecondaryContainerLight)
    }
}
