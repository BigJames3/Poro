package com.poro.app.core.text

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class MultilineTextTest {
    @Test
    fun `counts code points and lines like the shop and order services`() {
        assertTrue(MultilineText.fits("é".repeat(300), 300, 3))
        assertFalse(MultilineText.fits("a".repeat(301), 300, 3))
        assertTrue(MultilineText.fits("😀".repeat(300), 300, 3), "an emoji is one code point")
        assertTrue(MultilineText.fits("a\nb\nc", 300, 3))
        assertFalse(MultilineText.fits("a\nb\nc\nd", 300, 3))
        assertTrue(MultilineText.fits("a\r\nb\r\nc", 300, 3), "CRLF is one line break")
        assertTrue(MultilineText.fits("\n\na\nb\nc\n\n", 300, 3), "blank edges are trimmed first")
    }

    @Test
    fun `refuses the characters the server refuses`() {
        assertFalse(MultilineText.fits("a\tb", 300, 3))
        assertFalse(MultilineText.fits("a​b", 300, 3))
        assertFalse(MultilineText.fits("a‮b", 300, 3))
        assertFalse(MultilineText.fits("a﻿b", 300, 3))
    }

    @Test
    fun `empty text clears the field`() {
        assertTrue(MultilineText.fits("   \n ", 10, 1))
        assertEquals("", MultilineText.clean(" \r\n "))
        assertEquals("é", MultilineText.clean("é"))
    }

    private fun assertTrue(condition: Boolean, message: String) = org.junit.Assert.assertTrue(message, condition)
}
