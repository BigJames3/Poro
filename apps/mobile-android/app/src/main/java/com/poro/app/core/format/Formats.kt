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
}
