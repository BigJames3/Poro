package com.poro.app.core.device

import android.os.Build

/**
 * Nom envoyé à la connexion (`device_name`, 60 caractères au plus) et affiché dans la liste des
 * sessions de l'utilisateur, par exemple « Tecno Spark 20 ».
 */
object DeviceName {
    private const val MAX_LENGTH = 60

    fun current(): String = format(Build.MANUFACTURER, Build.MODEL)

    internal fun format(manufacturer: String?, model: String?): String {
        val maker = manufacturer.orEmpty().trim().replaceFirstChar { it.uppercase() }
        val name = model.orEmpty().trim()
        val full = when {
            name.isEmpty() -> maker
            maker.isEmpty() || name.startsWith(maker, ignoreCase = true) -> name
            else -> "$maker $name"
        }.ifEmpty { "Android" }
        return full.take(MAX_LENGTH)
    }
}
