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

    @Test
    fun `money in CFA francs has no decimals, naira is in kobo`() {
        assertEquals("12\u00A0500\u00A0FCFA", Formats.money(12_500, "XOF"))
        assertEquals("750\u00A0FCFA", Formats.money(750, "XAF"))
        assertEquals("₦1\u00A0250,50", Formats.money(125_050, "NGN"))
        assertEquals("₦1\u00A0250", Formats.money(125_000, "NGN"))
    }

    @Test
    fun `seller prices are read in the shop currency`() {
        assertEquals(12_500L, Formats.parseMoney("12 500", "XOF"))
        assertEquals(null, Formats.parseMoney("12,5", "XOF"))
        assertEquals(125_050L, Formats.parseMoney("1 250,5", "NGN"))
        assertEquals(125_000L, Formats.parseMoney("1250", "NGN"))
        assertEquals(null, Formats.parseMoney("1250,505", "NGN"))
        assertEquals(null, Formats.parseMoney("", "NGN"))
        assertEquals("1250,50", Formats.moneyInput(125_050, "NGN"))
        assertEquals("12500", Formats.moneyInput(12_500, "XOF"))
    }
}
