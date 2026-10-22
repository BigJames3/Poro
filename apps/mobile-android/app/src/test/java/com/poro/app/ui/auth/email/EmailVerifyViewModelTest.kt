package com.poro.app.ui.auth.email

import androidx.lifecycle.SavedStateHandle
import com.poro.app.R
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.data.auth.CodeSent
import com.poro.app.data.auth.UserDto
import com.poro.app.testing.FakeAuthRepository
import com.poro.app.testing.FakeAuthRepository.Companion.failure
import com.poro.app.testing.MainDispatcherRule
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class EmailVerifyViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    private val auth = FakeAuthRepository()

    private fun TestScope.viewModel(saved: SavedStateHandle = SavedStateHandle(), codeAlreadySent: Boolean = true) =
        EmailVerifyViewModel(codeAlreadySent, auth, saved, clock = { testScheduler.currentTime })

    @Test
    fun `the code sent at sign-up can be resent after a minute`() = runTest(main.dispatcher) {
        val vm = viewModel()
        runCurrent()
        assertEquals(60, vm.state.value.resendInSeconds)
        assertFalse(vm.state.value.canResend)

        advanceTimeBy(60_000)
        runCurrent()
        assertTrue(vm.state.value.canResend)
    }

    @Test
    fun `the sixth digit confirms the address`() = runTest(main.dispatcher) {
        auth.confirmResults += ApiResult.Success(UserDto(id = "u1", emailVerified = true))
        val vm = viewModel()

        vm.onCodeChange("123456")

        assertEquals(EmailVerifyEvent.Verified, vm.events.first())
        assertEquals(listOf("confirm:123456"), auth.emailCalls)
    }

    @Test
    fun `an expired code requires a new one, which restarts the cooldown`() = runTest(main.dispatcher) {
        auth.confirmResults += failure(422, ApiErrorCodes.CODE_EXPIRED)
        auth.emailCodeResults += ApiResult.Success(CodeSent(expiresIn = 900))
        val vm = viewModel()

        vm.onCodeChange("123456")
        runCurrent()
        assertTrue(vm.state.value.needsNewCode)
        assertEquals(R.string.error_code_expired, vm.state.value.error?.text)

        advanceTimeBy(60_000)
        runCurrent()
        vm.resend()
        runCurrent()
        assertFalse(vm.state.value.needsNewCode)
        assertTrue(vm.state.value.codeResent)
        assertEquals(60, vm.state.value.resendInSeconds)
    }

    @Test
    fun `opened from the profile, a code is requested at once`() = runTest(main.dispatcher) {
        auth.emailCodeResults += ApiResult.Success(CodeSent(expiresIn = 900))
        val vm = viewModel(codeAlreadySent = false)
        runCurrent()

        assertEquals(listOf("verification"), auth.emailCalls)
        assertTrue(vm.state.value.codeResent)
        assertEquals(60, vm.state.value.resendInSeconds)
    }

    @Test
    fun `an address verified meanwhile ends the step`() = runTest(main.dispatcher) {
        auth.emailCodeResults += failure(409, ApiErrorCodes.EMAIL_ALREADY_VERIFIED)
        val vm = viewModel()
        advanceTimeBy(60_000)
        runCurrent()

        vm.resend()

        assertEquals(EmailVerifyEvent.Verified, vm.events.first())
    }

    @Test
    fun `a throttled resend waits for Retry-After`() = runTest(main.dispatcher) {
        auth.emailCodeResults += failure(429, ApiErrorCodes.EMAIL_THROTTLED, retryAfter = 300)
        val vm = viewModel()
        advanceTimeBy(60_000)
        runCurrent()

        vm.resend()
        runCurrent()

        assertEquals(300, vm.state.value.resendInSeconds)
        assertEquals(R.string.error_email_throttled, vm.state.value.error?.text)
    }
}
