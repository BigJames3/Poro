package com.poro.app.core.account

import org.junit.Assert.assertEquals
import org.junit.Test

class TextLimitsTest {

    @Test
    fun `an emoji counts as one rune and is not split`() {
        val text = "ab👍cd"
        assertEquals(5, TextLimits.runeCount(text))
        assertEquals("ab👍", TextLimits.takeRunes(text, 3))
        assertEquals(text, TextLimits.takeRunes(text, 5))
        assertEquals("", TextLimits.takeRunes(text, 0))
    }
}
