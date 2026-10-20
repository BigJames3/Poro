package com.poro.app.core.security

import kotlinx.coroutines.flow.StateFlow

/** Jetons de la session de l'appareil (réponse `Tokens` de l'API Auth). */
data class SessionTokens(
    val accessToken: String,
    /** Opaque, change à chaque rafraîchissement : l'ancien ne doit plus jamais être renvoyé. */
    val refreshToken: String,
)

/**
 * Stockage des jetons. Lecture synchrone ([current]) car l'authenticator OkHttp tourne sur un
 * thread réseau ; [save] et [clear] persistent avant de rendre la main.
 */
interface TokenStore {
    /** Jetons courants, ou null si l'utilisateur n'est pas connecté. */
    fun current(): SessionTokens?

    /** `true` tant qu'une session est enregistrée : l'interface observe ce flux pour router. */
    val isSignedIn: StateFlow<Boolean>

    fun save(tokens: SessionTokens)

    fun clear()
}
