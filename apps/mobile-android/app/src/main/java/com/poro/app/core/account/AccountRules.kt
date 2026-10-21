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

    private val EMAIL = Regex("^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$")

    /** Le serveur enregistre l'adresse sans espaces autour et en minuscules : on envoie la même. */
    fun normalizeEmail(input: String): String = input.trim().lowercase()

    fun isValidEmail(input: String): Boolean {
        val email = normalizeEmail(input)
        return email.length <= EMAIL_MAX && EMAIL.matches(email)
    }

    /** Nouveau mot de passe (inscription, réinitialisation). À la connexion, seul le maximum compte. */
    fun isValidNewPassword(password: String): Boolean = password.length in PASSWORD_MIN..PASSWORD_MAX

    fun isValidFullName(input: String): Boolean = input.trim().length in FULL_NAME_MIN..FULL_NAME_MAX
}
