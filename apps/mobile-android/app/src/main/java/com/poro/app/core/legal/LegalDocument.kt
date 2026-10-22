package com.poro.app.core.legal

/** Documents légaux de l'application (liste proposée, à confirmer par le juridique). */
enum class LegalDocumentId(val slug: String) {
    Terms("terms"),
    Privacy("privacy"),
    Community("community"),
    Marketplace("marketplace"),
    ;

    companion object {
        fun fromSlug(slug: String): LegalDocumentId? = entries.firstOrNull { it.slug == slug }
    }
}

sealed interface LegalBlock {
    data class Heading(val text: String) : LegalBlock
    data class Paragraph(val text: String) : LegalBlock

    /** Encadré (`> …`), par exemple « Texte provisoire — À valider ». */
    data class Notice(val text: String) : LegalBlock
}

/**
 * Un document légal. [validated] false : texte de travail, affiché avec un bandeau permanent
 * « À valider », sans valeur juridique. [effectiveDate] null tant qu'aucune date n'est fixée.
 */
data class LegalDocument(
    val id: LegalDocumentId,
    val version: String,
    val effectiveDate: String?,
    val validated: Boolean,
    val title: String,
    val blocks: List<LegalBlock>,
)

/** Texte brut d'un document embarqué dans l'app ; null s'il n'existe pas encore. */
fun interface LegalTextSource {
    fun read(id: LegalDocumentId): String?
}

/**
 * Lit le format des documents de `res/raw/legal_*.md` : un en-tête `---` (version, effective,
 * status), puis `# titre`, `## section`, `> encadré` et des paragraphes séparés par une ligne vide.
 * Un document mal formé est refusé (null) plutôt qu'affiché à moitié.
 */
object LegalParser {
    fun parse(id: LegalDocumentId, text: String): LegalDocument? {
        val lines = text.replace("\r\n", "\n").lines()
        if (lines.firstOrNull()?.trim() != "---") return null
        val end = lines.drop(1).indexOfFirst { it.trim() == "---" }.takeIf { it >= 0 }?.plus(1) ?: return null
        val header = lines.subList(1, end).mapNotNull { line ->
            line.substringBefore('#').split(':', limit = 2).takeIf { it.size == 2 }?.let { (k, v) -> k.trim() to v.trim() }
        }.toMap()
        val version = header["version"]?.takeIf { it.isNotEmpty() } ?: return null
        val status = header["status"]
        if (status != "draft" && status != "validated") return null
        val effective = header["effective"]?.takeIf { it.isNotEmpty() && it != "null" }

        var title: String? = null
        val blocks = mutableListOf<LegalBlock>()
        val paragraph = StringBuilder()
        fun flush() {
            if (paragraph.isNotEmpty()) blocks += LegalBlock.Paragraph(paragraph.toString())
            paragraph.clear()
        }
        for (raw in lines.drop(end + 1)) {
            val line = raw.trim()
            when {
                line.isEmpty() -> flush()
                line.startsWith("## ") -> { flush(); blocks += LegalBlock.Heading(line.removePrefix("## ").trim()) }
                line.startsWith("# ") -> { flush(); title = title ?: line.removePrefix("# ").trim() }
                line.startsWith(">") -> { flush(); blocks += LegalBlock.Notice(line.removePrefix(">").trim()) }
                else -> paragraph.append(if (paragraph.isEmpty()) line else " $line")
            }
        }
        flush()
        return LegalDocument(id, version, effective, status == "validated", title ?: return null, blocks)
    }
}
