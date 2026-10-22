package com.poro.app.core.phone

/**
 * Pays de lancement (ADR-0004) : le service Auth refuse tout autre indicatif
 * (`phone_country_not_supported`).
 *
 * [nationalLength] est le nombre de chiffres après l'indicatif. [trunkZero] : le 0 initial du
 * format national disparaît au format international (Nigeria : 0803… → +234 803…). En Côte
 * d'Ivoire, le 0 fait partie du numéro à 10 chiffres et reste (+225 07…).
 */
enum class LaunchCountry(
    val dialCode: String,
    val flag: String,
    val nationalLength: Int,
    val trunkZero: Boolean = false,
) {
    CI("225", "🇨🇮", nationalLength = 10),
    SN("221", "🇸🇳", nationalLength = 9),
    CM("237", "🇨🇲", nationalLength = 9),
    NG("234", "🇳🇬", nationalLength = 10, trunkZero = true),
    ;

    companion object {
        val DEFAULT = CI
    }
}

/** Numéro validé d'un pays de lancement. */
data class PhoneNumber(val country: LaunchCountry, val national: String) {

    /** Format envoyé à l'API, par exemple `+2250700000000`. */
    val e164: String get() = "+${country.dialCode}$national"

    /**
     * Affichage partiellement masqué, par exemple `+225 07 •••••• 00` : le numéro complet
     * n'apparaît pas sur un écran qui peut être vu ou capturé.
     */
    fun masked(): String {
        val hidden = "•".repeat(national.length - VISIBLE * 2)
        return "+${country.dialCode} ${national.take(VISIBLE)} $hidden ${national.takeLast(VISIBLE)}"
    }

    companion object {
        private const val VISIBLE = 2
        private val SEPARATORS = Regex("[\\s.\\-()]")

        /**
         * Lit une saisie libre : espaces, tirets, points et parenthèses sont ignorés. Une saisie
         * internationale (`+234…`, `00221…`) choisit son pays, à condition qu'il soit un pays de
         * lancement ; sinon [country] s'applique. Renvoie null si le numéro est invalide.
         */
        fun parse(input: String, country: LaunchCountry): PhoneNumber? {
            val compact = input.trim().replace(SEPARATORS, "")
            val international = when {
                compact.startsWith("+") -> compact.drop(1)
                compact.startsWith("00") -> compact.drop(2)
                else -> null
            }
            val (target, digits) = if (international != null) {
                val match = LaunchCountry.entries.firstOrNull { international.startsWith(it.dialCode) } ?: return null
                match to international.drop(match.dialCode.length)
            } else {
                country to compact
            }
            if (digits.isEmpty() || !digits.all { it in '0'..'9' }) return null
            val national = if (target.trunkZero && digits.length == target.nationalLength + 1 && digits.startsWith("0")) {
                digits.drop(1)
            } else {
                digits
            }
            return if (national.length == target.nationalLength) PhoneNumber(target, national) else null
        }

        /** Relit un numéro déjà au format E.164 (argument de navigation). */
        fun fromE164(e164: String): PhoneNumber? =
            if (e164.startsWith("+")) parse(e164, LaunchCountry.DEFAULT) else null
    }
}
