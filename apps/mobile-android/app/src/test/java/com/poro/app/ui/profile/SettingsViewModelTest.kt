package com.poro.app.ui.profile

import com.poro.app.data.auth.SessionDto
import com.poro.app.testing.FakeAccountRepository
import com.poro.app.testing.FakeAuthRepository
import com.poro.app.testing.MainDispatcherRule
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Rule
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class SettingsViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    private val accounts = FakeAccountRepository()
    private val auth = FakeAuthRepository()

    @Test
    fun `other devices can be signed out one by one or all at once`() = runTest(main.dispatcher) {
        accounts.sessions = accounts.sessions + SessionDto("s3", "Itel A70")
        val vm = SettingsViewModel(accounts, auth)
        runCurrent()
        assertEquals(listOf("s1", "s2", "s3"), vm.state.value.sessions.valueOrNull?.map { it.id })

        vm.revoke("s2")
        runCurrent()
        assertEquals(listOf("s1", "s3"), vm.state.value.sessions.valueOrNull?.map { it.id })

        vm.revokeOthers()
        runCurrent()
        assertEquals(listOf("s1"), vm.state.value.sessions.valueOrNull?.map { it.id })
    }

    @Test
    fun `logout goes through the auth repository once`() = runTest(main.dispatcher) {
        val vm = SettingsViewModel(accounts, auth)
        vm.logout()
        vm.logout()
        runCurrent()
        assertEquals(1, auth.loggedOut)
    }
}
