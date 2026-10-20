package com.poro.app.core.security

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

/** Implémentation en mémoire, pour les tests et les aperçus Compose. */
class InMemoryTokenStore(initial: SessionTokens? = null) : TokenStore {
    @Volatile
    private var tokens: SessionTokens? = initial
    private val signedIn = MutableStateFlow(initial != null)
    override val isSignedIn: StateFlow<Boolean> = signedIn.asStateFlow()

    override fun current(): SessionTokens? = tokens

    override fun save(tokens: SessionTokens) {
        this.tokens = tokens
        signedIn.value = true
    }

    override fun clear() {
        tokens = null
        signedIn.value = false
    }
}
