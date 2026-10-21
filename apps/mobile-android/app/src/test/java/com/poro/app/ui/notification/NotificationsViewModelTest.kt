package com.poro.app.ui.notification

import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PageDto
import com.poro.app.data.notification.NotificationDto
import com.poro.app.data.notification.NotificationRepository
import com.poro.app.data.notification.PreferencesDto
import com.poro.app.data.notification.ReadAll
import com.poro.app.data.notification.ReadReceipt
import com.poro.app.testing.FakeAuthRepository.Companion.failure
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
class NotificationsViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    private class FakeNotifications : NotificationRepository {
        val pages = ArrayDeque<ApiResult<PageDto<NotificationDto>>>()
        val calls = mutableListOf<String>()
        var prefsUpdate: ApiResult<PreferencesDto> = ApiResult.Success(PreferencesDto())

        override suspend fun page(cursor: String?) = pages.removeFirst()
        override suspend fun unreadCount(): ApiResult<Int> = ApiResult.Success(3)
        override suspend fun markRead(id: String): ApiResult<ReadReceipt> {
            calls += "read:$id"
            return ApiResult.Success(ReadReceipt(id))
        }
        override suspend fun readAll(): ApiResult<ReadAll> {
            calls += "readAll"
            return ApiResult.Success(ReadAll(2))
        }
        override suspend fun preferences(): ApiResult<PreferencesDto> = ApiResult.Success(PreferencesDto())
        override suspend fun updatePreferences(preferences: PreferencesDto): ApiResult<PreferencesDto> {
            calls += "prefs:${preferences.likes}"
            return prefsUpdate
        }
    }

    private val repo = FakeNotifications()

    private fun n(id: String, read: Boolean = false) = NotificationDto(id = id, type = "like", read = read)

    @Test
    fun `pages skip a notification that moved up meanwhile`() = runTest(main.dispatcher) {
        repo.pages += ApiResult.Success(PageDto(listOf(n("a"), n("b")), "c2"))
        repo.pages += ApiResult.Success(PageDto(listOf(n("b"), n("c")), null))
        val vm = NotificationsViewModel(repo)
        runCurrent()
        vm.loadMore()
        runCurrent()

        assertEquals(listOf("a", "b", "c"), vm.state.value.items.map { it.id })
        assertFalse(vm.state.value.canLoadMore)
    }

    @Test
    fun `opening marks read, read all marks everything`() = runTest(main.dispatcher) {
        repo.pages += ApiResult.Success(PageDto(listOf(n("a"), n("b"), n("c", read = true)), null))
        val vm = NotificationsViewModel(repo)
        runCurrent()

        vm.open(vm.state.value.items[0])
        vm.open(vm.state.value.items[2])
        runCurrent()
        assertEquals(listOf("read:a"), repo.calls)
        assertTrue(vm.state.value.hasUnread)

        vm.readAll()
        runCurrent()
        assertFalse(vm.state.value.hasUnread)
        assertEquals("readAll", repo.calls.last())
    }

    @Test
    fun `a refused preference goes back to its previous value`() = runTest(main.dispatcher) {
        repo.prefsUpdate = failure(503, ApiErrorCodes.UNAVAILABLE)
        val vm = NotificationPreferencesViewModel(repo)
        runCurrent()

        vm.update { it.copy(likes = false) }
        assertFalse("affiché aussitôt", vm.state.value.preferences.valueOrNull!!.likes)
        runCurrent()

        assertTrue(vm.state.value.preferences.valueOrNull!!.likes)
        assertEquals(listOf("prefs:false"), repo.calls)
    }
}
