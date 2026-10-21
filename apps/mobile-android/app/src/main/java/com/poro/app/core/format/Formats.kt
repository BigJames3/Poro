package com.poro.app.core.format

import java.text.ParseException
import java.text.SimpleDateFormat
import java.util.Locale
import java.util.TimeZone

/**
 * Formats d'affichage, sans java.time (absent avant Android 8 sans désucrage ; minSdk 24).
 * Les dates du backend sont en ISO 8601 UTC (`2026-11-20T08:15:00Z`, fractions possibles).
 */
object Formats {

    /** Instant en millisecondes, ou null si la date est illisible. */
    fun parseIsoMillis(iso: String?): Long? {
        if (iso.isNullOrBlank()) return null
        // Les fractions de seconde (précision variable) ne servent pas à l'affichage.
        val trimmed = iso.replace(Regex("\\.\\d+"), "")
        return try {
            SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ssXXX", Locale.ROOT).parse(trimmed)?.time
        } catch (e: ParseException) {
            null
        }
    }

    /** « 20/11/2026 », dans le fuseau de l'appareil ; la chaîne brute si elle est illisible. */
    fun date(iso: String?, timeZone: TimeZone = TimeZone.getDefault()): String {
        val millis = parseIsoMillis(iso) ?: return iso.orEmpty()
        return SimpleDateFormat("dd/MM/yyyy", Locale.FRANCE).apply { this.timeZone = timeZone }.format(millis)
    }

    /** Ancienneté courte : « à l'instant », « 5 min », « 3 h », « 2 j », puis la date. */
    fun relative(iso: String?, nowMillis: Long, timeZone: TimeZone = TimeZone.getDefault()): String {
        val millis = parseIsoMillis(iso) ?: return ""
        val seconds = ((nowMillis - millis) / 1000).coerceAtLeast(0)
        return when {
            seconds < 60 -> "à l’instant"
            seconds < 3_600 -> "${seconds / 60} min"
            seconds < 86_400 -> "${seconds / 3_600} h"
            seconds < 7 * 86_400 -> "${seconds / 86_400} j"
            else -> date(iso, timeZone)
        }
    }

    /** Compteur compact à la française : 999, « 1,2 k », « 12 k », « 3,4 M ». */
    fun count(value: Long): String = when {
        value < 1_000 -> value.toString()
        value < 1_000_000 -> compact(value, 1_000, "k")
        else -> compact(value, 1_000_000, "M")
    }

    private fun compact(value: Long, unit: Long, suffix: String): String {
        val tenths = value * 10 / unit
        return if (tenths < 100 && tenths % 10 != 0L) "${tenths / 10},${tenths % 10} $suffix" else "${value / unit} $suffix"
    }

    /** Taille de fichier en Mo, une décimale : « 12,4 Mo ». */
    fun megabytes(bytes: Long): String {
        val tenths = bytes * 10 / (1024 * 1024)
        return "${tenths / 10},${tenths % 10} Mo"
    }

    /** Durée de vidéo « 0:42 » ou « 2:05 ». */
    fun duration(millis: Long): String {
        val totalSeconds = millis / 1000
        return "${totalSeconds / 60}:${(totalSeconds % 60).toString().padStart(2, '0')}"
    }

    /**
     * Montant en unités mineures : XOF et XAF n'en ont pas (« 12 500 FCFA »), NGN est en kobo
     * (« ₦1 250,50 »). Séparateur de milliers : espace insécable.
     */
    fun money(amount: Long, currency: String): String = when (currency) {
        "NGN" -> {
            val naira = grouped(amount / 100)
            val kobo = amount % 100
            if (kobo == 0L) "₦$naira" else "₦$naira,${kobo.toString().padStart(2, '0')}"
        }
        "XOF", "XAF" -> "${grouped(amount)}\u00A0FCFA"
        else -> "${grouped(amount)}\u00A0$currency"
    }

    /**
     * Prix saisi par un vendeur, en unités mineures ; null s'il est invalide. Espaces ignorés ;
     * décimales (virgule ou point, 2 au plus) seulement en NGN.
     */
    fun parseMoney(input: String, currency: String): Long? {
        val compact = input.replace(Regex("[\\s\u00A0]"), "").replace(',', '.')
        if (compact.isEmpty()) return null
        return if (currency == "NGN") {
            val match = Regex("^(\\d{1,10})(?:\\.(\\d{1,2}))?$").matchEntire(compact) ?: return null
            val kobo = match.groupValues[2].padEnd(2, '0')
            match.groupValues[1].toLong() * 100 + kobo.toLong()
        } else {
            compact.takeIf { Regex("^\\d{1,12}$").matches(it) }?.toLong()
        }
    }

    /** Valeur à préremplir dans un champ de prix : l'inverse de [parseMoney], sans séparateurs. */
    fun moneyInput(amount: Long, currency: String): String = when (currency) {
        "NGN" -> if (amount % 100 == 0L) (amount / 100).toString() else "${amount / 100},${(amount % 100).toString().padStart(2, '0')}"
        else -> amount.toString()
    }

    private fun grouped(value: Long): String =
        value.toString().reversed().chunked(3).joinToString("\u00A0").reversed()
}
