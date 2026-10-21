package com.poro.app.ui.legal

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNull
import org.junit.Test

class LegalDocumentTest {

    @Test
    fun `the three store documents are listed and unknown ids stay unavailable`() {
        assertEquals(listOf("tos", "privacy", "mentions"), LegalDocument.entries.map { it.id })
        assertEquals(LegalDocument.Privacy, LegalDocument.fromId("privacy"))
        assertNull(LegalDocument.fromId("cookies"))
    }
}
