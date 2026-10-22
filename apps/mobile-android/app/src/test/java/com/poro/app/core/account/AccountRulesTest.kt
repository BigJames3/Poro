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
    fun `profile limits follow the user service, in code points`() {
        assertEquals(50, AccountRules.displayNameLength("  " + "é".repeat(50) + "  "))
        assertEquals("espaces regroupés : « Awa Koné »", 8, AccountRules.displayNameLength("Awa   Koné"))
        assertTrue(AccountRules.isValidBio("😀".repeat(160)))
        assertFalse(AccountRules.isValidBio("😀".repeat(161)))
        assertTrue(AccountRules.isValidBio("1\n2\n3\n4\n5"))
        assertFalse(AccountRules.isValidBio("1\n2\n3\n4\n5\n6"))
    }
}
