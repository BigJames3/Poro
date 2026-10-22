package com.poro.app.ui.social

import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.data.social.ShareChannel
import com.poro.app.data.social.VideoStatsDto
import com.poro.app.testing.FakeAuthRepository.Companion.failure
import com.poro.app.testing.FakeSocialRepository
import com.poro.app.testing.MainDispatcherRule
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class VideoInteractionsViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    private val social = FakeSocialRepository()
    private val fallback = VideoStatsDto("v1", likes = 10, comments = 2, shares = 1)

    @Test
    fun `stats of a shown video are read once`() = runTest(main.dispatcher) {
        social.stats = fallback.copy(likedByMe = true, likes = 11)
        val vm = VideoInteractionsViewModel(social)

        vm.onShown("v1")
        vm.onShown("v1")
        runCurrent()

        assertEquals(listOf("stats:v1"), social.calls)
        assertTrue(vm.state.value.stats.getValue("v1").likedByMe)
    }

    @Test
    fun `a feed page reads its counters in one request`() = runTest(main.dispatcher) {
        social.batchStats = listOf(fallback.copy(videoId = "v1", likedByMe = true), fallback.copy(videoId = "v2"))
        val vm = VideoInteractionsViewModel(social)

        vm.prefetch(listOf("v1", "v2"))
        runCurrent()
        vm.onShown("v1")
        vm.onShown("v2")
        runCurrent()

        assertEquals(listOf("batch:v1,v2"), social.calls)
        assertTrue(vm.state.value.stats.getValue("v1").likedByMe)
    }

    @Test
    fun `a failed batch falls back to one request per shown video`() = runTest(main.dispatcher) {
        social.batchStats = null
        val vm = VideoInteractionsViewModel(social)

        vm.prefetch(listOf("v1", "v2"))
        runCurrent()
        vm.onShown("v1")
        runCurrent()

        assertEquals(listOf("batch:v1,v2", "stats:v1"), social.calls)
    }

    @Test
    fun `a like shows at once and the server answer wins`() = runTest(main.dispatcher) {
        social.likeResults += ApiResult.Success(fallback.copy(likes = 15, likedByMe = true))
        val vm = VideoInteractionsViewModel(social)

        vm.toggleLike("v1", fallback)
        assertEquals("optimiste", 11L, vm.state.value.stats.getValue("v1").likes)
        runCurrent()

        assertEquals(15L, vm.state.value.stats.getValue("v1").likes)
        assertEquals(listOf("like:v1:true"), social.calls)
    }

    @Test
    fun `a refused like is rolled back with a message`() = runTest(main.dispatcher) {
        social.likeResults += failure(429, ApiErrorCodes.RATE_LIMITED, retryAfter = 30)
        val vm = VideoInteractionsViewModel(social)

        vm.toggleLike("v1", fallback)
        runCurrent()

        assertEquals(fallback, vm.state.value.stats.getValue("v1"))
        assertNotNull(vm.state.value.error)
    }

    @Test
    fun `shares and comments update the counters`() = runTest(main.dispatcher) {
        val vm = VideoInteractionsViewModel(social)

        vm.recordShare("v1", ShareChannel.WhatsApp, fallback)
        runCurrent()
        vm.onCommentsChanged("v1", +1, vm.state.value.stats.getValue("v1"))

        val stats = vm.state.value.stats.getValue("v1")
        assertEquals(2L, stats.shares)
        assertEquals(3L, stats.comments)
        assertEquals("share:v1:whatsapp", social.calls.single())
    }
}
