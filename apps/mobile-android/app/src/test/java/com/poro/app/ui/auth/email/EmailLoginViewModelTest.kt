package com.poro.app.ui.auth.email

import androidx.lifecycle.SavedStateHandle
import com.poro.app.R
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.testing.FakeAuthRepository
import com.poro.app.testing.FakeAuthRepository.Companion.failure
import com.poro.app.testing.FakeAuthRepository.Companion.signedIn
import com.poro.app.testing.MainDispatcherRule
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class EmailLoginViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    private val auth = FakeAuthRepository()

    private fun TestScope.viewModel(saved: SavedStateHandle = SavedStateHandle()) =
        EmailLoginViewModel(auth, saved, clock = { testScheduler.currentTime })

    @Test
    fun `signs in with the normalized address`() = runTest(main.dispatcher) {
        auth.signInResults += signedIn(deletionCancelled = true)
        val vm = viewModel()
        assertFalse(vm.state.value.canSubmit)

        vm.onEmailChange("  Awa@Poro.Africa ")
        vm.onPasswordChange("s3cret-pass")
        vm.submit()

        assertEquals(EmailLoginEvent.SignedIn(deletionCancelled = true), vm.events.first())
        assertEquals(listOf("login:awa@poro.africa"), auth.emailCalls)
    }

    @Test
    fun `a wrong password is cleared but the address stays`() = runTest(main.dispatcher) {
        auth.signInResults += failure(401, ApiErrorCodes.INVALID_CREDENTIALS)
        val vm = viewModel()
        vm.onEmailChange("awa@poro.africa")
        vm.onPasswordChange("bad")

        vm.submit()
        runCurrent()

        assertEquals("", vm.state.value.password)
        assertEquals("awa@poro.africa", vm.state.value.email)
        assertEquals(R.string.error_invalid_credentials, vm.state.value.error?.text)
    }

    @Test
    fun `a locked account waits for Retry-After`() = runTest(main.dispatcher) {
        auth.signInResults += failure(429, ApiErrorCodes.LOGIN_THROTTLED, retryAfter = 60)
        val vm = viewModel()
        vm.onEmailChange("awa@poro.africa")
        vm.onPasswordChange("pass")

        vm.submit()
        runCurrent()
        assertEquals(60, vm.state.value.retryInSeconds)
        assertFalse(vm.state.value.canSubmit)
        assertEquals(R.string.error_login_throttled, vm.state.value.error?.text)

        advanceTimeBy(60_000)
        runCurrent()
        assertTrue(vm.state.value.canSubmit)
    }

    @Test
    fun `the address survives process death, never the password`() = runTest(main.dispatcher) {
        val saved = SavedStateHandle()
        val first = viewModel(saved)
        first.onEmailChange("awa@poro.africa")
        first.onPasswordChange("s3cret-pass")

        val restored = viewModel(saved)

        assertEquals("awa@poro.africa", restored.state.value.email)
        assertEquals("", restored.state.value.password)
        assertTrue("aucune valeur enregistrée ne contient le mot de passe", saved.keys().none { saved.get<Any>(it) == "s3cret-pass" })
        assertNull(restored.state.value.error)
    }
}
