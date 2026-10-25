package com.poro.app.ui.upload

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
import org.junit.Rule
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class MyVideosViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    private class FakeVideos : VideoRepository {
        var mine = listOf(VideoDto(id = "old", status = VideoDto.STATUS_READY))
        var requests = 0

        override suspend fun video(id: String): ApiResult<VideoDto> = error("unused")

        override suspend fun mine(cursor: String?): ApiResult<PageDto<VideoDto>> {
            requests++
            return ApiResult.Success(PageDto(mine, null))
        }

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
}
