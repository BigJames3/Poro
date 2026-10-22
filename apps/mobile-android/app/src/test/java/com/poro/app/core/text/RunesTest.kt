package com.poro.app.core.text

import com.poro.app.ui.upload.UploadFormState
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class RunesTest {

    @Test
    fun `lengths are counted in code points like the video service`() {
        // Deux points de code, quatre unités UTF-16.
        assertEquals(2, Runes.count("😀😀"))
        assertEquals(4, "😀😀".length)
        assertEquals("😀", Runes.take("😀😀", 1))
        assertEquals("abc", Runes.take("abc", 5))
    }

    @Test
    fun `invisible format and control characters are refused, newlines only in descriptions`() {
        assertTrue(Runes.hasForbidden("a\u0007b", allowNewlines = true))
        assertFalse(Runes.hasForbidden("ligne 1\nligne 2", allowNewlines = true))
        assertTrue(Runes.hasForbidden("ligne 1\nligne 2", allowNewlines = false))
        assertFalse(Runes.hasForbidden("Match 🏆 à Abidjan", allowNewlines = false))
    }

    @Test
    fun `composed emoji are allowed, a joiner elsewhere is not`() {
        val composed = listOf(
            "Famille 👩\u200D👩\u200D👧",
            "Coeur ❤\uFE0F\u200D🔥",
            "Dev 👩🏽\u200D💻",
            "Coureuse 🏃🏾\u200D♀\uFE0F",
        )
        composed.forEach { assertFalse(it, Runes.hasForbidden(it, allowNewlines = false)) }
        listOf(
            "a\u200Db",
            "\u200D👩",
            "👩\u200D",
            "👩\u200D\u200D👧",
            "👩\u200Db",
            "👩\u200B👧",
        ).forEach { assertTrue(it, Runes.hasForbidden(it, allowNewlines = false)) }
    }

    @Test
    fun `a description of 500 emoji is accepted, 501 is not`() {
        val file = com.poro.app.core.media.LocalMedia("content://v", "video/mp4", 10)
        val ok = UploadFormState(file = file, title = "Match", description = "⚽".repeat(500))
        val tooLong = ok.copy(description = "⚽".repeat(501))

        assertTrue(ok.canPublish)
        assertFalse(tooLong.canPublish)
        assertFalse(ok.copy(title = "Match\u200D").canPublish)
    }
}
