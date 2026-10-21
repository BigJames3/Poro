package com.poro.app.ui.social

import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.testing.FakeAuthRepository.Companion.failure
import com.poro.app.testing.FakeProfileRepository
import com.poro.app.testing.FakeSocialRepository
import com.poro.app.testing.MainDispatcherRule
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class PublicProfileViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    private val profiles = FakeProfileRepository()
    private val social = FakeSocialRepository()

    @Test
    fun `follow shows at once and takes the server counters`() = runTest(main.dispatcher) {
        social.followResults += ApiResult.Success(social.userStats.copy(followers = 6, followedByMe = true))
        val vm = PublicProfileViewModel("u2", "kofi", profiles, social)
        runCurrent()
        assertFalse(vm.state.value.isMe)

        vm.toggleFollow()
        assertTrue(vm.state.value.following)
        runCurrent()

        assertEquals(6L, vm.state.value.stats?.followers)
        assertEquals(listOf("follow:u2:true"), social.calls)
    }

    @Test
    fun `my own profile has no follow button`() = runTest(main.dispatcher) {
        val vm = PublicProfileViewModel("u1", "awa", profiles, social)
        runCurrent()

        assertTrue(vm.state.value.isMe)
        vm.toggleFollow()
        runCurrent()
        assertTrue(social.calls.isEmpty())
    }

    @Test
    fun `a refused follow is rolled back`() = runTest(main.dispatcher) {
        social.followResults += failure(400, ApiErrorCodes.CANNOT_FOLLOW_SELF)
        val vm = PublicProfileViewModel("u9", null, profiles, social)
        runCurrent()

        vm.toggleFollow()
        runCurrent()

        assertFalse(vm.state.value.following)
        assertTrue("le serveur dit que c'est soi : plus de bouton", vm.state.value.isMe)
    }
}
