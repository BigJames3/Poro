package com.poro.app.core.format

import java.util.TimeZone
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class FormatsTest {

    private val abidjan = TimeZone.getTimeZone("Africa/Abidjan")
    private val lagos = TimeZone.getTimeZone("Africa/Lagos")

    @Test
    fun `ISO dates are parsed with or without fractions and offsets`() {
        assertEquals(1_795_756_800_000L, Formats.parseIsoMillis("2026-11-27T05:20:00Z"))
        assertEquals(1_795_756_800_000L, Formats.parseIsoMillis("2026-11-27T05:20:00.123456Z"))
        assertEquals(1_795_756_800_000L, Formats.parseIsoMillis("2026-11-27T06:20:00+01:00"))
        assertNull(Formats.parseIsoMillis("hier"))
        assertNull(Formats.parseIsoMillis(null))
    }

    @Test
    fun `dates are shown in the device time zone`() {
        assertEquals("27/11/2026", Formats.date("2026-11-27T23:30:00Z", abidjan))
        assertEquals("28/11/2026", Formats.date("2026-11-27T23:30:00Z", lagos))
    }

    @Test
    fun `relative age`() {
        val now = Formats.parseIsoMillis("2026-11-27T12:00:00Z")!!
        assertEquals("à l’instant", Formats.relative("2026-11-27T11:59:30Z", now, abidjan))
        assertEquals("5 min", Formats.relative("2026-11-27T11:55:00Z", now, abidjan))
        assertEquals("3 h", Formats.relative("2026-11-27T09:00:00Z", now, abidjan))
        assertEquals("2 j", Formats.relative("2026-11-25T12:00:00Z", now, abidjan))
        assertEquals("01/11/2026", Formats.relative("2026-11-01T12:00:00Z", now, abidjan))
    }

    @Test
    fun `compact counters`() {
        assertEquals("999", Formats.count(999))
        assertEquals("1 k", Formats.count(1_000))
        assertEquals("1,2 k", Formats.count(1_250))
        assertEquals("12 k", Formats.count(12_400))
        assertEquals("3,4 M", Formats.count(3_400_000))
    }

    @Test
    fun `video durations`() {
        assertEquals("0:42", Formats.duration(42_300))
        assertEquals("2:05", Formats.duration(125_000))
    }
}
