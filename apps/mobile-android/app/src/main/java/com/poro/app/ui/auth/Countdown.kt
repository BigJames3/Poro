package com.poro.app.ui.auth

import androidx.lifecycle.SavedStateHandle
import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.retryDelaySeconds
import com.poro.app.core.time.Clock
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch

/**
 * Compte à rebours vers une échéance gardée dans [savedState] sous [key], en temps absolu de
 * [clock] : recréé après une mort du processus, il reprend là où il en était. [onTick] reçoit les
 * secondes restantes, une fois par seconde, jusqu'à 0 inclus.
 */
class Countdown(
    private val scope: CoroutineScope,
    private val savedState: SavedStateHandle,
    private val key: String,
    private val clock: Clock,
    private val onTick: (Int) -> Unit,
) {
    private var job: Job? = null

    /** Une échéance a déjà été fixée (par cette instance ou avant la mort du processus). */
    val isSet: Boolean get() = savedState.get<Long>(key) != null

    /** Reprend l'échéance enregistrée, s'il y en a une. */
    fun restore() {
        savedState.get<Long>(key)?.let(::runUntil)
    }

    fun start(seconds: Long) = runUntil(clock.elapsedMillis() + seconds * TICK_MS)

    private fun runUntil(deadline: Long) {
        // L'échéance passée reste enregistrée : [isSet] distingue « terminé » de « jamais lancé ».
        savedState[key] = deadline
        job?.cancel()
        job = scope.launch {
            while (true) {
                val remaining = ((deadline - clock.elapsedMillis() + TICK_MS - 1) / TICK_MS).coerceAtLeast(0).toInt()
                onTick(remaining)
                if (remaining == 0) break
                delay(TICK_MS)
            }
        }
    }

    private companion object {
        const val TICK_MS = 1000L
    }
}

/** Compte les 429 consécutifs pour le recul exponentiel (voir [retryDelaySeconds]). */
class RateLimitBackoff {
    private var consecutive = 0

    /** Délai imposé par ce résultat en secondes, ou null s'il n'y a pas à attendre. */
    fun delayAfter(result: ApiResult<*>): Long? {
        if (result !is ApiResult.Failure) {
            if (result is ApiResult.Success) consecutive = 0
            return null
        }
        if (result.httpStatus == 429) consecutive++ else consecutive = 0
        return result.retryDelaySeconds(consecutive)
    }
}
