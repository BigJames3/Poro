package com.poro.app.core.security

import android.content.Context
import android.util.Log
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import java.util.concurrent.CountDownLatch
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking

private val Context.sessionDataStore: DataStore<Preferences> by preferencesDataStore(name = "session")

/**
 * Jetons chiffrés par [KeystoreCipher] puis écrits dans DataStore.
 *
 * Le chargement initial (DataStore puis Keystore, lent sur les appareils d'entrée de gamme) tourne
 * dans [scope], hors du thread principal : [session] vaut [SessionState.Unknown] jusqu'à sa fin.
 * Une copie en mémoire sert ensuite les lectures, fréquentes et synchrones (intercepteurs OkHttp).
 * [current], [save] et [clear] attendent la fin du chargement : jamais depuis le thread principal.
 */
class EncryptedTokenStore(
    context: Context,
    scope: CoroutineScope,
    private val cipher: KeystoreCipher = KeystoreCipher(),
) : TokenStore {

    private val dataStore = context.applicationContext.sessionDataStore
    private val lock = Any()
    private val loaded = CountDownLatch(1)

    @Volatile
    private var cached: SessionTokens? = null

    private val state = MutableStateFlow(SessionState.Unknown)
    override val session: StateFlow<SessionState> = state.asStateFlow()

    init {
        scope.launch {
            try {
                val tokens = try {
                    load()
                } catch (e: Exception) {
                    // DataStore illisible : on repart d'une session vide plutôt que de bloquer l'application.
                    Log.w(TAG, "session could not be loaded: ${e.javaClass.simpleName}")
                    null
                }
                cached = tokens
                state.value = if (tokens != null) SessionState.SignedIn else SessionState.SignedOut
            } finally {
                // Toujours libérer les appels en attente, sinon le réseau resterait bloqué.
                loaded.countDown()
            }
        }
    }

    override fun current(): SessionTokens? {
        loaded.await()
        return cached
    }

    override fun save(tokens: SessionTokens) {
        loaded.await()
        synchronized(lock) {
            try {
                val access = cipher.encrypt(tokens.accessToken)
                val refresh = cipher.encrypt(tokens.refreshToken)
                runBlocking {
                    dataStore.edit {
                        it[ACCESS] = access
                        it[REFRESH] = refresh
                    }
                }
            } catch (e: Exception) {
                // Le Keystore lève des exceptions variées selon les constructeurs (ProviderException,
                // KeyStoreException…) : toutes deviennent un échec propre, jamais un plantage.
                throw TokenStorageException(e)
            }
            cached = tokens
            state.value = SessionState.SignedIn
        }
    }

    override fun clear() {
        loaded.await()
        synchronized(lock) {
            cached = null
            state.value = SessionState.SignedOut
            try {
                runBlocking { dataStore.edit { it.clear() } }
            } catch (e: Exception) {
                // La session est déjà fermée en mémoire ; l'écriture sera retentée à la prochaine déconnexion.
                Log.w(TAG, "session could not be cleared on disk: ${e.javaClass.simpleName}")
            }
        }
    }

    private suspend fun load(): SessionTokens? {
        val prefs = dataStore.data.first()
        val access = prefs[ACCESS]?.let(cipher::decrypt)
        val refresh = prefs[REFRESH]?.let(cipher::decrypt)
        if (access != null && refresh != null) return SessionTokens(access, refresh)
        // Clé perdue ou données incomplètes : on repart d'une session vide.
        if (prefs[ACCESS] != null || prefs[REFRESH] != null) dataStore.edit { it.clear() }
        return null
    }

    private companion object {
        const val TAG = "PoroSession"
        val ACCESS = stringPreferencesKey("access_token")
        val REFRESH = stringPreferencesKey("refresh_token")
    }
}
