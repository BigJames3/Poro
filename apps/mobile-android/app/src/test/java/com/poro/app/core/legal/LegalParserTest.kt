package com.poro.app.core.legal

import java.io.File
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Test

class LegalParserTest {

    @Test
    fun `a document is read with its header, title, sections and notices`() {
        val doc = LegalParser.parse(
            LegalDocumentId.Privacy,
            """
            ---
            version: 1.0
            effective: 2026-11-01   # date d'effet
            status: validated
            ---
            # Politique de confidentialité

            > Résumé en une ligne.

            ## Données collectées
            Première ligne
            suite du paragraphe.

            Second paragraphe.
            """.trimIndent(),
        )!!

        assertEquals("1.0", doc.version)
        assertEquals("2026-11-01", doc.effectiveDate)
        assertTrue(doc.validated)
        assertEquals("Politique de confidentialité", doc.title)
        assertEquals(
            listOf(
                LegalBlock.Notice("Résumé en une ligne."),
                LegalBlock.Heading("Données collectées"),
                LegalBlock.Paragraph("Première ligne suite du paragraphe."),
                LegalBlock.Paragraph("Second paragraphe."),
            ),
            doc.blocks,
        )
    }

    @Test
    fun `a malformed document is refused rather than shown half`() {
        assertNull(LegalParser.parse(LegalDocumentId.Terms, "# Sans en-tête"))
        assertNull(LegalParser.parse(LegalDocumentId.Terms, "---\nversion: 1\nstatus: final\n---\n# Titre"))
        assertNull(LegalParser.parse(LegalDocumentId.Terms, "---\nversion: 1\nstatus: draft\n---\nPas de titre"))
    }

    @Test
    fun `every bundled document is a draft to validate`() {
        // Aucun texte de travail ne doit pouvoir passer pour définitif sans décision du juridique.
        for (id in LegalDocumentId.entries) {
            val file = listOf("src/main/res/raw", "app/src/main/res/raw").map { File(it, "legal_${id.slug}.md") }.firstOrNull { it.exists() }
            assertNotNull("legal_${id.slug}.md manquant", file)
            val doc = LegalParser.parse(id, file!!.readText())
            assertNotNull("legal_${id.slug}.md illisible", doc)
            assertFalse("legal_${id.slug}.md ne doit pas être marqué validé", doc!!.validated)
            assertNull(doc.effectiveDate)
        }
    }
}
