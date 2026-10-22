package com.poro.app.ui.feed

import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PageDto
import com.poro.app.data.feed.FeedItem
import com.poro.app.data.feed.FeedKind
import com.poro.app.data.feed.FeedRepository
import com.poro.app.testing.FakeAuthRepository.Companion.failure
import com.poro.app.testing.MainDispatcherRule
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class FeedViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    /** Réponses programmées par fil ; enregistre les curseurs demandés. */
    private class FakeFeed : FeedRepository {
        val pages = FeedKind.entries.associateWith { ArrayDeque<ApiResult<PageDto<FeedItem>>>() }
        val requests = mutableListOf<Pair<FeedKind, String?>>()

        override suspend fun page(kind: FeedKind, cursor: String?): ApiResult<PageDto<FeedItem>> {
            requests += kind to cursor
            return pages.getValue(kind).removeFirst()
        }
    }

    private val feed = FakeFeed()

    private fun page(vararg ids: String, next: String?) = ApiResult.Success(PageDto(ids.map { FeedItem(videoId = it) }, next))

    @Test
    fun `the next page is loaded a few videos before the end`() = runTest(main.dispatcher) {
        feed.pages.getValue(FeedKind.ForYou) += page("v1", "v2", "v3", "v4", "v5", next = "c2")
        feed.pages.getValue(FeedKind.ForYou) += page("v6", "v7", next = null)
        val vm = FeedViewModel(feed)
        runCurrent()

        vm.onItemShown(1)
        runCurrent()
        assertEquals("pas encore", 1, feed.requests.size)

        vm.onItemShown(2)
        runCurrent()
        assertEquals(listOf(FeedKind.ForYou to null, FeedKind.ForYou to "c2"), feed.requests)
        assertEquals(7, vm.state.value.current.items.size)
        assertTrue(vm.state.value.current.endReached)

        vm.onItemShown(6)
        runCurrent()
        assertEquals("fin du fil : plus de requête", 2, feed.requests.size)
    }

    @Test
    fun `a restarted For You session does not repeat videos`() = runTest(main.dispatcher) {
        feed.pages.getValue(FeedKind.ForYou) += page("v1", "v2", next = "c2")
        feed.pages.getValue(FeedKind.ForYou) += page("v2", "v3", next = "c3")
        val vm = FeedViewModel(feed)
        runCurrent()

        vm.loadMore(FeedKind.ForYou)
        runCurrent()

        assertEquals(listOf("v1", "v2", "v3"), vm.state.value.current.items.map { it.videoId })
    }

    @Test
    fun `each feed keeps its own pages and is loaded on first selection`() = runTest(main.dispatcher) {
        feed.pages.getValue(FeedKind.ForYou) += page("v1", next = null)
        feed.pages.getValue(FeedKind.Following) += page(next = null)
        val vm = FeedViewModel(feed)
        runCurrent()

        vm.select(FeedKind.Following)
        runCurrent()
        assertTrue(vm.state.value.current.isEmpty)

        vm.select(FeedKind.ForYou)
        runCurrent()
        assertEquals(listOf("v1"), vm.state.value.current.items.map { it.videoId })
        assertEquals(2, feed.requests.size)
    }

    @Test
    fun `an expired cursor restarts the feed from the first page`() = runTest(main.dispatcher) {
        feed.pages.getValue(FeedKind.Trending) += page("v1", next = "c2")
        feed.pages.getValue(FeedKind.Trending) += failure(400, ApiErrorCodes.INVALID_CURSOR)
        feed.pages.getValue(FeedKind.Trending) += page("v9", next = null)
        feed.pages.getValue(FeedKind.ForYou) += page(next = null)
        val vm = FeedViewModel(feed)
        vm.select(FeedKind.Trending)
        runCurrent()

        vm.loadMore(FeedKind.Trending)
        runCurrent()

        assertEquals(FeedKind.Trending to null, feed.requests.last())
        assertEquals(listOf("v9"), vm.state.value.current.items.map { it.videoId })
    }
}
