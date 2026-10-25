package com.poro.app.ui

import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PageDto
import com.poro.app.data.notification.NotificationDto
import com.poro.app.data.notification.NotificationRepository
import com.poro.app.data.notification.PreferencesDto
import com.poro.app.data.notification.ReadAll
import com.poro.app.data.notification.ReadReceipt
import com.poro.app.data.order.CartDto
import com.poro.app.testing.FakeOrderRepository
import com.poro.app.testing.MainDispatcherRule
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class TabBadgesViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    private val orders = FakeOrderRepository()

    private class FakeNotifications : NotificationRepository {
        var unread: ApiResult<Int> = ApiResult.Success(0)
        override suspend fun page(cursor: String?): ApiResult<PageDto<NotificationDto>> = error("unused")
        override suspend fun unreadCount() = unread
        override suspend fun markRead(id: String): ApiResult<ReadReceipt> = error("unused")
        override suspend fun readAll(): ApiResult<ReadAll> = error("unused")
        override suspend fun preferences(): ApiResult<PreferencesDto> = error("unused")
        override suspend fun updatePreferences(preferences: PreferencesDto): ApiResult<PreferencesDto> = error("unused")
    }

    private val notifications = FakeNotifications()

    @Test
    fun `badges show unread notifications and cart items`() = runTest(main.dispatcher) {
        notifications.unread = ApiResult.Success(3)
        orders.cart = ApiResult.Success(CartDto(itemCount = 2))
        val vm = TabBadgesViewModel(notifications, orders)

        vm.refresh()
        runCurrent()

        assertEquals(TabBadges(unreadNotifications = 3, cartItems = 2), vm.badges.value)
    }

    @Test
    fun `a failed refresh keeps the last known counts`() = runTest(main.dispatcher) {
        notifications.unread = ApiResult.Success(5)
        orders.cart = ApiResult.Success(CartDto(itemCount = 1))
        val vm = TabBadgesViewModel(notifications, orders)
        vm.refresh()
        runCurrent()

        notifications.unread = ApiResult.Failure(503, ApiErrorCodes.UNAVAILABLE, "down", null, null)
        orders.cart = ApiResult.Failure(503, ApiErrorCodes.UNAVAILABLE, "down", null, null)
        vm.refresh()
        runCurrent()

        assertEquals(TabBadges(unreadNotifications = 5, cartItems = 1), vm.badges.value)
    }

    @Test
    fun `a badge caps its text at 99+`() {
        assertEquals("7", badgeText(7))
        assertEquals("99", badgeText(99))
        assertEquals("99+", badgeText(100))
    }
}
