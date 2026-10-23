package com.poro.app

import androidx.annotation.StringRes
import androidx.compose.ui.test.assertCountEquals
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createAndroidComposeRule
import androidx.compose.ui.test.onAllNodesWithText
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.test.espresso.Espresso
import androidx.test.ext.junit.runners.AndroidJUnit4
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

/**
 * Premier lancement, sans session : l'app ouvre le fil invité (aucun backend n'est requis, le fil
 * peut afficher une erreur réseau) et « Se connecter » mène à la connexion, fil invité dessous.
 */
@RunWith(AndroidJUnit4::class)
class GuestStartTest {

    @get:Rule
    val compose = createAndroidComposeRule<MainActivity>()

    private fun string(@StringRes id: Int) = compose.activity.getString(id)

    private fun waitFor(text: String) {
        compose.waitUntil(TIMEOUT_MS) { compose.onAllNodesWithText(text).fetchSemanticsNodes().isNotEmpty() }
    }

    @Test
    fun aGuestLandsOnThePublicFeedAndCanSignIn() {
        val signIn = string(R.string.action_sign_in)
        waitFor(signIn)
        compose.onNodeWithText(string(R.string.feed_for_you)).assertIsDisplayed()
        // Abonnements reste réservé aux comptes.
        compose.onAllNodesWithText(string(R.string.feed_following)).assertCountEquals(0)

        compose.onNodeWithText(signIn).performClick()
        waitFor(string(R.string.phone_subtitle))

        // « Retour » ramène au fil invité au lieu de fermer l'app.
        Espresso.closeSoftKeyboard()
        Espresso.pressBack()
        waitFor(signIn)
    }

    private companion object {
        const val TIMEOUT_MS = 10_000L
    }
}
