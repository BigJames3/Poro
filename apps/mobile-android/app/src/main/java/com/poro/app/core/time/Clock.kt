package com.poro.app.core.time

import android.os.SystemClock

/**
 * Temps monotone en millisecondes, injectable pour les tests. Les échéances (renvoi du code,
 * expiration) sont des instants absolus de cette horloge : elles restent justes après une mort
 * du processus, contrairement à un compteur qui repartirait de zéro.
 */
fun interface Clock {
    fun elapsedMillis(): Long
}

/** Horloge de l'appareil depuis le démarrage, insensible aux changements d'heure. */
object SystemElapsedClock : Clock {
    override fun elapsedMillis(): Long = SystemClock.elapsedRealtime()
}
