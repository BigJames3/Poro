package com.poro.app.core.account

/**
 * Règles de saisie du compte email, alignées sur le contrat Auth (docs/api/auth.openapi.yaml) :
 * le serveur reste juge, ces contrôles évitent seulement un aller-retour voué à l'échec.
 */
object AccountRules {
    const val EMAIL_MAX = 254
    const val PASSWORD_MIN = 8
    const val PASSWORD_MAX = 128
    const val FULL_NAME_MIN = 2
    const val FULL_NAME_MAX = 100
    const val DISPLAY_NAME_MAX = 50
    const val BIO_MAX = 160
    const val USERNAME_MAX = 30

    private val EMAIL = Regex("^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$")
    private val USERNAME = Regex("^[a-z0-9._]{3,30}$")

    /** Le serveur enregistre l'adresse sans espaces autour et en minuscules : on envoie la même. */
    fun normalizeEmail(input: String): String = input.trim().lowercase()

    fun isValidEmail(input: String): Boolean {
        val email = normalizeEmail(input)
        return email.length <= EMAIL_MAX && EMAIL.matches(email)
    }

    /** Nouveau mot de passe (inscription, réinitialisation). À la connexion, seul le maximum compte. */
    fun isValidNewPassword(password: String): Boolean = password.length in PASSWORD_MIN..PASSWORD_MAX

    fun isValidFullName(input: String): Boolean = input.trim().length in FULL_NAME_MIN..FULL_NAME_MAX

    /** Le serveur stocke les pseudos en minuscules ; un « @ » tapé en tête est ignoré. */
    fun normalizeUsername(input: String): String = input.trim().removePrefix("@").lowercase()

    /** 3 à 30 caractères parmi a-z, 0-9, « . » et « _ » (le serveur juge aussi les pseudos réservés). */
    fun isValidUsername(input: String): Boolean = USERNAME.matches(normalizeUsername(input))
}

/**
 * Compte et coupe en runes Unicode (points de code), comme le backend
 * (`utf8.RuneCountInString`) : un emoji compte pour 1, pas pour 2 unités UTF-16.
 */
object TextLimits {
    fun runeCount(value: String): Int = value.codePointCount(0, value.length)

    fun takeRunes(value: String, max: Int): String {
        if (max <= 0 || value.isEmpty()) return ""
        if (runeCount(value) <= max) return value
        var end = 0
        var count = 0
        while (end < value.length && count < max) {
            end += Character.charCount(value.codePointAt(end))
            count++
        }
        return value.substring(0, end)
    }
}
