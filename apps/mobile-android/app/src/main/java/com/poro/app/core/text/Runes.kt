package com.poro.app.core.text

/**
 * Règles de texte du service video (services/video/internal/text/text.go) : les longueurs se
 * comptent en points de code Unicode (« runes » Go), pas en unités UTF-16 de Kotlin, et les
 * caractères de contrôle (Cc) ou de format (Cf, dont le joint U+200D des émojis composés) sont
 * refusés ; seuls les sauts de ligne sont permis dans une description.
 */
object Runes {
    /** Nombre de points de code : un émoji simple compte 1, comme côté serveur. */
    fun count(text: String): Int = text.codePointCount(0, text.length)

    /** Les [max] premiers points de code, sans couper une paire de substitution. */
    fun take(text: String, max: Int): String =
        if (count(text) <= max) text else text.substring(0, text.offsetByCodePoints(0, max))

    /** Caractère refusé par le serveur : contrôle ou format, hors sauts de ligne si [allowNewlines]. */
    fun hasForbidden(text: String, allowNewlines: Boolean): Boolean = text.codePoints().anyMatch { cp ->
        if (allowNewlines && (cp == '\n'.code || cp == '\r'.code)) return@anyMatch false
        val type = Character.getType(cp)
        type == Character.CONTROL.toInt() || type == Character.FORMAT.toInt()
    }
}
