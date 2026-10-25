package com.poro.app.ui.social

import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.SemanticsProperties
import androidx.compose.ui.test.SemanticsMatcher
import androidx.compose.ui.test.assert
import androidx.compose.ui.test.assertHeightIsAtLeast
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.createComposeRule
import androidx.compose.ui.test.onNodeWithText
import androidx.compose.ui.test.performClick
import androidx.compose.ui.unit.dp
import androidx.test.ext.junit.runners.AndroidJUnit4
import androidx.test.platform.app.InstrumentationRegistry
import com.poro.app.R
import com.poro.app.core.format.Formats
import com.poro.app.data.social.FollowEntryDto
import com.poro.app.data.social.UserStatsDto
import com.poro.app.data.user.ProfileDto
import com.poro.app.ui.theme.PoroTheme
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test
import org.junit.runner.RunWith

/** Compteurs d'abonnés cliquables (cible ≥ 48 dp, rôle bouton) et liste avec noms résolus. */
@RunWith(AndroidJUnit4::class)
class FollowUiTest {

    @get:Rule
    val compose = createComposeRule()

    private val context = InstrumentationRegistry.getInstrumentation().targetContext

    @Test
    fun eachCounterIsAButtonOfAtLeast48dpThatOpensItsList() {
        val opened = mutableListOf<String>()
        compose.setContent {
            PoroTheme { FollowCounts(UserStatsDto(followers = 1200, following = 85), { opened += "followers" }, { opened += "following" }) }
        }
        val followers = context.getString(R.string.profile_followers, Formats.count(1200))
        val following = context.getString(R.string.profile_following, Formats.count(85))
        val isButton = SemanticsMatcher.expectValue(SemanticsProperties.Role, Role.Button)

        compose.onNodeWithText(followers).assertHeightIsAtLeast(48.dp).assert(isButton).performClick()
        compose.onNodeWithText(following).assertHeightIsAtLeast(48.dp).assert(isButton).performClick()

        assertEquals(listOf("followers", "following"), opened)
    }

    @Test
    fun theListShowsResolvedNamesAndAFallbackForDeletedAccounts() {
        val opened = mutableListOf<Pair<String, String?>>()
        val state = FollowListUiState(
            items = listOf(FollowEntryDto("u2"), FollowEntryDto("gone")),
            profiles = mapOf("u2" to ProfileDto(userId = "u2", username = "awa", displayName = "Awa")),
            started = true,
        )
        compose.setContent {
            PoroTheme { FollowListScreen(FollowListKind.Followers, state, onBack = {}, onLoadMore = {}, onOpen = { e, u -> opened += e.userId to u }) }
        }

        compose.onNodeWithText("@awa").assertIsDisplayed()
        compose.onNodeWithText(context.getString(R.string.member_fallback)).assertIsDisplayed()
        compose.onNodeWithText("Awa").performClick()

        assertEquals(listOf("u2" to "awa"), opened)
    }
}
