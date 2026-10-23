package com.poro.app.ui.social

import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PageDto
import com.poro.app.data.social.FollowEntryDto
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

    @Test
    fun `followers are paged without duplicates`() = runTest(main.dispatcher) {
        social.followPages += ApiResult.Success(PageDto(listOf(FollowEntryDto("u2"), FollowEntryDto("u3")), "n2"))
        social.followPages += ApiResult.Success(PageDto(listOf(FollowEntryDto("u3"), FollowEntryDto("u4"))))
        val vm = FollowListViewModel("u1", FollowListKind.Followers, social)
        runCurrent()

        assertEquals(listOf("u2", "u3"), vm.state.value.items.map { it.userId })
        vm.loadMore()
        runCurrent()
        assertEquals(listOf("u2", "u3", "u4"), vm.state.value.items.map { it.userId })
        assertTrue(vm.state.value.endReached)
        assertTrue("followers:u1:null" in social.calls)
        assertTrue("followers:u1:n2" in social.calls)
    }
}
