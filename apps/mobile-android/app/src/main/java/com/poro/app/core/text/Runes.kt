package com.poro.app.core.text

/**
 * Règles de texte du service video (services/video/internal/text/text.go) : les longueurs se
 * comptent en points de code Unicode (« runes » Go), pas en unités UTF-16 de Kotlin, et les
 * caractères de contrôle (Cc) ou de format (Cf) sont refusés. Exceptions : les sauts de ligne
 * dans une description, et le joint U+200D entre deux émojis (émojis composés).
 */
object Runes {
    /** Nombre de points de code : un émoji simple compte 1, comme côté serveur. */
    fun count(text: String): Int = text.codePointCount(0, text.length)

    /** Les [max] premiers points de code, sans couper une paire de substitution. */
    fun take(text: String, max: Int): String =
        if (count(text) <= max) text else text.substring(0, text.offsetByCodePoints(0, max))

    /**
     * Caractère refusé par le serveur : contrôle ou format, hors sauts de ligne si [allowNewlines]
     * et hors joint entre deux émojis (famille, cœur en feu, métier avec teinte de peau).
     */
    fun hasForbidden(text: String, allowNewlines: Boolean): Boolean {
        val cps = text.codePoints().toArray()
        return cps.indices.any { i ->
            val cp = cps[i]
            when {
                cp == '\n'.code || cp == '\r'.code -> !allowNewlines
                cp == ZERO_WIDTH_JOINER && joinsEmoji(cps, i) -> false
                else -> Character.getType(cp).let { it == Character.CONTROL.toInt() || it == Character.FORMAT.toInt() }
            }
        }
    }

    /** Le joint en [i] relie deux émojis : émoji, teinte de peau ou sélecteur avant, émoji après. */
    private fun joinsEmoji(cps: IntArray, i: Int): Boolean {
        if (i == 0 || i == cps.lastIndex) return false
        val before = cps[i - 1]
        return (isEmoji(before) || isSkinTone(before) || before == VARIATION_SELECTOR) && isEmoji(cps[i + 1])
    }

    private fun isEmoji(cp: Int) = Character.getType(cp) == Character.OTHER_SYMBOL.toInt() && !isSkinTone(cp)

    private fun isSkinTone(cp: Int) = cp in 0x1F3FB..0x1F3FF

    private const val ZERO_WIDTH_JOINER = 0x200D
    private const val VARIATION_SELECTOR = 0xFE0F
}
