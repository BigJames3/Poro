package com.poro.app.core.security

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.runBlocking

private val Context.sessionDataStore: DataStore<Preferences> by preferencesDataStore(name = "session")

/**
 * Jetons chiffrés par [KeystoreCipher] puis écrits dans DataStore.
 * Une copie en mémoire sert les lectures, fréquentes et synchrones (intercepteurs OkHttp).
 * Les écritures sont bloquantes : elles ne sont jamais appelées depuis le thread principal
 * (couche réseau et ViewModels sur Dispatchers.IO).
 */
class EncryptedTokenStore(
    context: Context,
    private val cipher: KeystoreCipher = KeystoreCipher(),
) : TokenStore {

    private val dataStore = context.applicationContext.sessionDataStore
    private val lock = Any()

    @Volatile
    private var cached: SessionTokens? = load()

    private val signedIn = MutableStateFlow(cached != null)
    override val isSignedIn: StateFlow<Boolean> = signedIn.asStateFlow()

    override fun current(): SessionTokens? = cached

    override fun save(tokens: SessionTokens) = synchronized(lock) {
        runBlocking {
            dataStore.edit {
                it[ACCESS] = cipher.encrypt(tokens.accessToken)
                it[REFRESH] = cipher.encrypt(tokens.refreshToken)
            }
        }
        cached = tokens
        signedIn.value = true
    }

    override fun clear() = synchronized(lock) {
        runBlocking { dataStore.edit { it.clear() } }
        cached = null
        signedIn.value = false
    }

    private fun load(): SessionTokens? = runBlocking {
        val prefs = dataStore.data.first()
        val access = prefs[ACCESS]?.let(cipher::decrypt)
        val refresh = prefs[REFRESH]?.let(cipher::decrypt)
        if (access != null && refresh != null) {
            SessionTokens(access, refresh)
        } else {
            // Clé perdue ou données incomplètes : on repart d'une session vide.
            if (prefs[ACCESS] != null || prefs[REFRESH] != null) dataStore.edit { it.clear() }
            null
        }
    }

    private companion object {
        val ACCESS = stringPreferencesKey("access_token")
        val REFRESH = stringPreferencesKey("refresh_token")
    }
}
