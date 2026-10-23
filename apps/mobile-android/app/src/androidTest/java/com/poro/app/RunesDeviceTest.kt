package com.poro.app

import androidx.test.ext.junit.runners.AndroidJUnit4
import com.poro.app.core.text.Runes
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test
import org.junit.runner.RunWith

/**
 * Règle des caractères invisibles avec les tables Unicode de l'appareil (ICU d'Android), qui
 * peuvent différer de celles de la JVM des tests unitaires.
 */
@RunWith(AndroidJUnit4::class)
class RunesDeviceTest {

    private fun text(vararg codePoints: Int) = String(codePoints, 0, codePoints.size)

    private val woman = 0x1F469
    private val girl = 0x1F467
    private val joiner = 0x200D

    @Test
    fun composedEmojiAreAllowed() {
        assertFalse("famille", Runes.hasForbidden(text(woman, joiner, woman, joiner, girl), allowNewlines = false))
        assertFalse("teinte de peau puis joiner", Runes.hasForbidden(text(woman, 0x1F3FD, joiner, 0x1F33E), allowNewlines = false))
        assertFalse("drapeau arc-en-ciel", Runes.hasForbidden(text(0x1F3F3, 0xFE0F, joiner, 0x1F308), allowNewlines = false))
    }

    @Test
    fun aJoinerOutsideEmojiIsRefused() {
        assertTrue("entre deux lettres", Runes.hasForbidden(text('a'.code, joiner, 'b'.code), allowNewlines = false))
        assertTrue("en fin de texte", Runes.hasForbidden(text(woman, joiner), allowNewlines = false))
        assertTrue("autre caractère de format", Runes.hasForbidden(text('a'.code, 0x200B, 'b'.code), allowNewlines = false))
    }
}
