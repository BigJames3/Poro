package com.poro.app.core.text

import java.text.Normalizer

/**
 * Règle `cleanText` des services shop et order (`services/shop/src/common/text.ts`) : texte
 * normalisé NFC, fins de ligne unifiées puis espaces de bord retirés ; longueur en points de
 * code, nombre de lignes plafonné, et aucun caractère de contrôle, de largeur nulle ou de
 * direction (U+200B à U+200F, U+202A à U+202E, U+2066 à U+2069, U+FEFF). Vide = champ effacé.
 */
object MultilineText {
    // Saut de ligne (U+000A) permis : le serveur découpe les lignes avant de chercher ces caractères.
    private val FORBIDDEN = Regex("[\\u0000-\\u0009\\u000B-\\u001F\\u007F-\\u009F\\u200B-\\u200F\\u202A-\\u202E\\u2066-\\u2069\\uFEFF]")
    private val LINE_BREAK = Regex("\r\n?")

    /** Le texte tel que le serveur l'enregistre. */
    fun clean(text: String): String = Normalizer.normalize(text, Normalizer.Form.NFC).replace(LINE_BREAK, "\n").trim()

    /** Accepté par le serveur avec au plus [max] points de code et [maxLines] lignes. */
    fun fits(text: String, max: Int, maxLines: Int): Boolean {
        val value = clean(text)
        return value.isEmpty() ||
            (Runes.count(value) <= max && value.split('\n').size <= maxLines && !FORBIDDEN.containsMatchIn(value))
    }
}
