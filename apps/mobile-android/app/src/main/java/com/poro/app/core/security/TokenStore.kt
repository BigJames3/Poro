package com.poro.app.core.security

import java.io.IOException
import kotlinx.coroutines.flow.StateFlow

/** Jetons de la session de l'appareil (réponse `Tokens` de l'API Auth). */
data class SessionTokens(
    val accessToken: String,
    /** Opaque, change à chaque rafraîchissement : l'ancien ne doit plus jamais être renvoyé. */
    val refreshToken: String,
)

/** État de la session, observé par l'interface pour choisir le premier écran. */
enum class SessionState {
    /** Jetons en cours de lecture au démarrage : afficher un écran d'attente. */
    Unknown,
    SignedIn,
    SignedOut,
}

/**
 * Stockage des jetons. Lecture synchrone ([current]) car l'authenticator OkHttp tourne sur un
 * thread réseau ; [save] et [clear] persistent avant de rendre la main. Aucune de ces trois
 * méthodes ne doit être appelée depuis le thread principal.
 */
interface TokenStore {
    /** Jetons courants, ou null si l'utilisateur n'est pas connecté. */
    fun current(): SessionTokens?

    val session: StateFlow<SessionState>

    /** @throws TokenStorageException si les jetons ne peuvent pas être enregistrés. */
    fun save(tokens: SessionTokens)

    fun clear()
}

/**
 * Les jetons n'ont pas pu être chiffrés ou écrits (Keystore défaillant sur certains appareils,
 * disque plein). Hérite d'[IOException] : levée pendant un rafraîchissement dans OkHttp,
 * elle fait échouer la requête au lieu de faire planter l'application.
 */
class TokenStorageException(cause: Throwable) : IOException("session tokens could not be stored", cause)
