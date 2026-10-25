package com.poro.app.ui.upload

import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PageDto
import com.poro.app.data.video.VideoDto
import com.poro.app.data.video.VideoRepository
import com.poro.app.testing.MainDispatcherRule
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Rule
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class MyVideosViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    private class FakeVideos : VideoRepository {
        var mine = listOf(VideoDto(id = "old", status = VideoDto.STATUS_READY))
        var requests = 0
        var down = false

        override suspend fun video(id: String): ApiResult<VideoDto> = error("unused")

        override suspend fun mine(cursor: String?): ApiResult<PageDto<VideoDto>> {
            requests++
            if (down) return ApiResult.Failure(503, ApiErrorCodes.UNAVAILABLE, "down", null, null)
            return ApiResult.Success(PageDto(mine, null))
        }

        override suspend fun byUser(userId: String, cursor: String?): ApiResult<PageDto<VideoDto>> = error("unused")

        override suspend fun delete(id: String): ApiResult<Unit> = ApiResult.Success(Unit)
    }

    private val videos = FakeVideos()

    @Test
    fun `a reload shows a new video without emptying the grid first`() = runTest(main.dispatcher) {
        val vm = MyVideosViewModel(videos)
        runCurrent()
        assertEquals(listOf("old"), vm.state.value.videos.map { it.id })

        videos.mine = listOf(VideoDto(id = "new", status = VideoDto.STATUS_PROCESSING)) + videos.mine
        vm.reload()
        assertEquals("the grid stays while the page loads", listOf("old"), vm.state.value.videos.map { it.id })
        runCurrent()
        assertEquals(listOf("new", "old"), vm.state.value.videos.map { it.id })
        assertFalse(vm.state.value.loading)
    }

    @Test
    fun `a reload while loading is ignored`() = runTest(main.dispatcher) {
        val vm = MyVideosViewModel(videos)
        vm.reload()
        runCurrent()
        assertEquals("the first load is still running: no second request", 1, videos.requests)
    }

    @Test
    fun `a failed first page can be retried`() = runTest(main.dispatcher) {
        videos.down = true
        val vm = MyVideosViewModel(videos)
        runCurrent()
        assertNotNull(vm.state.value.error)

        videos.down = false
        vm.loadMore()
        runCurrent()

        assertEquals(listOf("old"), vm.state.value.videos.map { it.id })
        assertNull(vm.state.value.error)
    }
}
