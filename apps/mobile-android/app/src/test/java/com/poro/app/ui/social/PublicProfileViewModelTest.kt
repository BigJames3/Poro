package com.poro.app.ui.social

import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PageDto
import com.poro.app.data.video.VideoDto
import com.poro.app.data.video.VideoRepository
import com.poro.app.testing.FakeAuthRepository.Companion.failure
import com.poro.app.testing.FakeProfileRepository
import com.poro.app.testing.FakeSocialRepository
import com.poro.app.testing.MainDispatcherRule
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class PublicProfileViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    private val profiles = FakeProfileRepository()
    private val social = FakeSocialRepository()
    private val videos = FakeVideos()

    private class FakeVideos : VideoRepository {
        /** Réponses successives de [byUser], dans l'ordre. */
        val pages = ArrayDeque<ApiResult<PageDto<VideoDto>>>()
        val requests = mutableListOf<String>()

        override suspend fun video(id: String): ApiResult<VideoDto> = error("unused")
        override suspend fun mine(cursor: String?): ApiResult<PageDto<VideoDto>> = error("unused")
        override suspend fun byUser(userId: String, cursor: String?): ApiResult<PageDto<VideoDto>> {
            requests += "$userId:$cursor"
            return pages.removeFirstOrNull() ?: ApiResult.Success(PageDto())
        }
        override suspend fun delete(id: String): ApiResult<Unit> = error("unused")
    }

    @Test
    fun `follow shows at once and takes the server counters`() = runTest(main.dispatcher) {
        social.followResults += ApiResult.Success(social.userStats.copy(followers = 6, followedByMe = true))
        val vm = PublicProfileViewModel("u2", "kofi", profiles, social, videos)
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
        val vm = PublicProfileViewModel("u1", "awa", profiles, social, videos)
        runCurrent()

        assertTrue(vm.state.value.isMe)
        vm.toggleFollow()
        runCurrent()
        assertTrue(social.calls.isEmpty())
    }

    @Test
    fun `a refused follow is rolled back`() = runTest(main.dispatcher) {
        social.followResults += failure(400, ApiErrorCodes.CANNOT_FOLLOW_SELF)
        val vm = PublicProfileViewModel("u9", null, profiles, social, videos)
        runCurrent()

        vm.toggleFollow()
        runCurrent()

        assertFalse(vm.state.value.following)
        assertTrue("le serveur dit que c'est soi : plus de bouton", vm.state.value.isMe)
    }

    @Test
    fun `the public posts are listed page by page`() = runTest(main.dispatcher) {
        videos.pages += ApiResult.Success(PageDto(listOf(VideoDto("a"), VideoDto("b")), "next"))
        videos.pages += ApiResult.Success(PageDto(listOf(VideoDto("c")), null))
        val vm = PublicProfileViewModel("u2", "kofi", profiles, social, videos)
        runCurrent()
        assertEquals(listOf("a", "b"), vm.state.value.videos.map { it.id })
        assertTrue(vm.state.value.canLoadMoreVideos)

        vm.loadMoreVideos()
        runCurrent()

        assertEquals(listOf("a", "b", "c"), vm.state.value.videos.map { it.id })
        assertFalse(vm.state.value.canLoadMoreVideos)
        vm.loadMoreVideos()
        runCurrent()
        assertEquals("rien après la dernière page", listOf("u2:null", "u2:next"), videos.requests)
    }

    @Test
    fun `a failed first page is retried`() = runTest(main.dispatcher) {
        videos.pages += failure(503, ApiErrorCodes.UNAVAILABLE)
        val vm = PublicProfileViewModel("u2", "kofi", profiles, social, videos)
        runCurrent()
        assertNotNull(vm.state.value.videosError)
        assertTrue(vm.state.value.videos.isEmpty())

        vm.loadMoreVideos()
        runCurrent()

        assertNull(vm.state.value.videosError)
        assertEquals(listOf("u2:null", "u2:null"), videos.requests)
    }

    @Test
    fun `an untitled video shows the first line of its description`() {
        assertEquals("Danse du soir", VideoDto("v", description = "\n  Danse du soir \n#wax").headline)
        assertEquals("Titre", VideoDto("v", title = "Titre", description = "Danse").headline)
    }
}
