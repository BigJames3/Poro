package com.poro.app.core.phone

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class PhoneNumberTest {

    @Test
    fun `national numbers of the four launch countries become E164`() {
        assertEquals("+2250700000000", PhoneNumber.parse("07 00 00 00 00", LaunchCountry.CI)?.e164)
        assertEquals("+221771234567", PhoneNumber.parse("77 123 45 67", LaunchCountry.SN)?.e164)
        assertEquals("+237690123456", PhoneNumber.parse("6 90 12 34 56", LaunchCountry.CM)?.e164)
        assertEquals("+2348031234567", PhoneNumber.parse("803 123 4567", LaunchCountry.NG)?.e164)
    }

    @Test
    fun `the Nigerian trunk zero is dropped but the Ivorian leading zero is kept`() {
        assertEquals("+2348031234567", PhoneNumber.parse("0803-123-4567", LaunchCountry.NG)?.e164)
        assertEquals("+2348031234567", PhoneNumber.parse("+234 0803 123 4567", LaunchCountry.CI)?.e164)
        assertEquals("+2250102030405", PhoneNumber.parse("01.02.03.04.05", LaunchCountry.CI)?.e164)
        assertNull("le 0 n'est pas un préfixe au Sénégal", PhoneNumber.parse("0771234567", LaunchCountry.SN))
    }

    @Test
    fun `an international input picks its own launch country`() {
        val pasted = PhoneNumber.parse("+221 77 123 45 67", LaunchCountry.CI)
        assertEquals(LaunchCountry.SN, pasted?.country)
        assertEquals("+221771234567", pasted?.e164)
        assertEquals(LaunchCountry.CM, PhoneNumber.parse("00237 690123456", LaunchCountry.CI)?.country)
    }

    @Test
    fun `countries outside the launch list are refused`() {
        assertNull(PhoneNumber.parse("+33 6 12 34 56 78", LaunchCountry.CI))
        assertNull(PhoneNumber.parse("+1 415 555 0100", LaunchCountry.CI))
    }

    @Test
    fun `wrong lengths and characters are refused`() {
        assertNull(PhoneNumber.parse("", LaunchCountry.CI))
        assertNull(PhoneNumber.parse("07 00 00 00", LaunchCountry.CI))
        assertNull(PhoneNumber.parse("07 00 00 00 00 0", LaunchCountry.CI))
        assertNull(PhoneNumber.parse("07 00 AB 00 00", LaunchCountry.CI))
        assertNull(PhoneNumber.parse("77 123 45 678", LaunchCountry.SN))
        assertNull(PhoneNumber.parse("+", LaunchCountry.CI))
    }

    @Test
    fun `masked display keeps the first and last two digits`() {
        assertEquals("+225 07 •••••• 00", PhoneNumber.parse("0700000000", LaunchCountry.CI)?.masked())
        assertEquals("+221 77 ••••• 67", PhoneNumber.parse("771234567", LaunchCountry.SN)?.masked())
    }

    @Test
    fun `E164 round trip`() {
        val phone = PhoneNumber.fromE164("+2348031234567")
        assertEquals(PhoneNumber(LaunchCountry.NG, "8031234567"), phone)
        assertNull(PhoneNumber.fromE164("08031234567"))
    }
}
