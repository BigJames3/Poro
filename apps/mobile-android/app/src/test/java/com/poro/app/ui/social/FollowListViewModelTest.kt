package com.poro.app.ui.social

import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PageDto
import com.poro.app.data.social.FollowEntryDto
import com.poro.app.data.user.ProfileDto
import com.poro.app.testing.FakeProfileRepository
import com.poro.app.testing.FakeSocialRepository
import com.poro.app.testing.MainDispatcherRule
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class FollowListViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    private val social = FakeSocialRepository()
    private val profiles = FakeProfileRepository()

    @Test
    fun `followers are paged without duplicates`() = runTest(main.dispatcher) {
        social.followPages += ApiResult.Success(PageDto(listOf(FollowEntryDto("u2"), FollowEntryDto("u3")), "n2"))
        social.followPages += ApiResult.Success(PageDto(listOf(FollowEntryDto("u3"), FollowEntryDto("u4"))))
        val vm = FollowListViewModel("u1", FollowListKind.Followers, social, profiles)
        runCurrent()

        assertEquals(listOf("u2", "u3"), vm.state.value.items.map { it.userId })
        vm.loadMore()
        runCurrent()
        assertEquals(listOf("u2", "u3", "u4"), vm.state.value.items.map { it.userId })
        assertTrue(vm.state.value.endReached)
        assertTrue("followers:u1:null" in social.calls)
        assertTrue("followers:u1:n2" in social.calls)
    }

    @Test
    fun `names and avatars are read in one batch per page, deleted accounts left out`() = runTest(main.dispatcher) {
        profiles.publicProfiles["u2"] = ProfileDto(userId = "u2", username = "awa", displayName = "Awa", avatarUrl = "https://cdn/a.jpg")
        social.followPages += ApiResult.Success(PageDto(listOf(FollowEntryDto("u2"), FollowEntryDto("gone")), "n2"))
        social.followPages += ApiResult.Success(PageDto(listOf(FollowEntryDto("u2"), FollowEntryDto("u5"))))
        val vm = FollowListViewModel("u1", FollowListKind.Following, social, profiles)
        runCurrent()

        assertEquals("Awa", vm.state.value.profiles["u2"]?.displayName)
        assertEquals("https://cdn/a.jpg", vm.state.value.profiles["u2"]?.avatarUrl)
        assertEquals(null, vm.state.value.profiles["gone"])
        assertTrue("following:u1:null" in social.calls)

        vm.loadMore()
        runCurrent()
        // Les comptes déjà demandés ne sont pas relus.
        assertEquals(listOf("profiles:u2,gone", "profiles:u5"), profiles.calls)
    }
}
