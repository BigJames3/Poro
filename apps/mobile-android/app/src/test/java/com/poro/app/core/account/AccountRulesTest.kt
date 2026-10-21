package com.poro.app.core.account

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class AccountRulesTest {

    @Test
    fun `email is trimmed and lowercased like on the server`() {
        assertEquals("awa@poro.africa", AccountRules.normalizeEmail("  Awa@Poro.Africa "))
    }

    @Test
    fun `plausible addresses only`() {
        assertTrue(AccountRules.isValidEmail("awa@poro.africa"))
        assertTrue(AccountRules.isValidEmail(" Awa.Kone+test@mail.example.ci "))
        assertFalse(AccountRules.isValidEmail("awa@poro"))
        assertFalse(AccountRules.isValidEmail("awa poro@mail.ci"))
        assertFalse(AccountRules.isValidEmail("@mail.ci"))
        assertFalse(AccountRules.isValidEmail(""))
        assertFalse(AccountRules.isValidEmail("a".repeat(250) + "@m.ci"))
    }

    @Test
    fun `new passwords hold between 8 and 128 characters`() {
        assertFalse(AccountRules.isValidNewPassword("1234567"))
        assertTrue(AccountRules.isValidNewPassword("12345678"))
        assertTrue(AccountRules.isValidNewPassword("x".repeat(128)))
        assertFalse(AccountRules.isValidNewPassword("x".repeat(129)))
    }

    @Test
    fun `full names hold between 2 and 100 characters once trimmed`() {
        assertFalse(AccountRules.isValidFullName(" A "))
        assertTrue(AccountRules.isValidFullName("Awa Koné"))
        assertFalse(AccountRules.isValidFullName("x".repeat(101)))
    }

    @Test
    fun `display name and bio match the user service limits`() {
        assertEquals(50, AccountRules.DISPLAY_NAME_MAX)
        assertEquals(160, AccountRules.BIO_MAX)
    }
}
