package com.poro.app.ui.player

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class PlaybackRulesTest {

    @Test
    fun `mute silences the player without changing the URL`() {
        assertEquals(1f, PlaybackRules.volume(muted = false), 0f)
        assertEquals(0f, PlaybackRules.volume(muted = true), 0f)
    }

    @Test
    fun `a new URL or a previous error forces prepare`() {
        assertTrue(PlaybackRules.shouldPrepare("b", "a", hasError = false))
        assertTrue(PlaybackRules.shouldPrepare("a", "a", hasError = true))
        assertFalse(PlaybackRules.shouldPrepare("a", "a", hasError = false))
        assertTrue(PlaybackRules.shouldPrepare("a", null, hasError = false))
    }
}
