package com.poro.app.core.security

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

/** Implémentation en mémoire, pour les tests et les aperçus Compose. */
class InMemoryTokenStore(initial: SessionTokens? = null) : TokenStore {
    @Volatile
    private var tokens: SessionTokens? = initial
    private val state = MutableStateFlow(if (initial != null) SessionState.SignedIn else SessionState.SignedOut)
    override val session: StateFlow<SessionState> = state.asStateFlow()

    override fun current(): SessionTokens? = tokens

    override fun save(tokens: SessionTokens) {
        this.tokens = tokens
        state.value = SessionState.SignedIn
    }

    override fun clear() {
        tokens = null
        state.value = SessionState.SignedOut
    }
}
