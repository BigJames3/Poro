package com.poro.app.ui.auth.email

import androidx.lifecycle.SavedStateHandle
import com.poro.app.R
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.data.auth.CodeSent
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
class PasswordResetViewModelsTest {

    @get:Rule
    val main = MainDispatcherRule()

    private val auth = FakeAuthRepository()

    private fun TestScope.forgot(initialEmail: String = "") =
        ForgotPasswordViewModel(initialEmail, auth, SavedStateHandle(), clock = { testScheduler.currentTime })

    private fun TestScope.reset() =
        ResetPasswordViewModel(EMAIL, auth, SavedStateHandle(), clock = { testScheduler.currentTime })

    @Test
    fun `forgot password starts from the login address and sends the normalized one`() = runTest(main.dispatcher) {
        auth.emailCodeResults += ApiResult.Success(CodeSent(expiresIn = 900))
        val vm = forgot(initialEmail = " Awa@Poro.Africa")
        assertTrue(vm.state.value.canSubmit)

        vm.submit()

        assertEquals(ForgotPasswordEvent.CodeSent(EMAIL), vm.events.first())
        assertEquals(listOf("forgot:$EMAIL"), auth.emailCalls)
    }

    @Test
    fun `forgot password waits when the address is throttled`() = runTest(main.dispatcher) {
        auth.emailCodeResults += failure(429, ApiErrorCodes.EMAIL_THROTTLED, retryAfter = 45)
        val vm = forgot(EMAIL)

        vm.submit()
        runCurrent()

        assertEquals(45, vm.state.value.retryInSeconds)
        assertFalse(vm.state.value.canSubmit)
    }

    @Test
    fun `reset needs six digits and a valid new password`() = runTest(main.dispatcher) {
        auth.resetResults += ApiResult.Success(Unit)
        val vm = reset()

        vm.onCodeChange("123456")
        assertFalse("pas de soumission sans mot de passe", vm.state.value.canSubmit)
        assertTrue(auth.emailCalls.isEmpty())
        vm.onPasswordChange("1234567")
        assertFalse(vm.state.value.canSubmit)
        vm.onPasswordChange("new-pass-1")

        vm.submit()

        assertEquals(ResetPasswordEvent.PasswordChanged, vm.events.first())
        assertEquals(listOf("reset:$EMAIL:123456"), auth.emailCalls)
    }

    @Test
    fun `a wrong code is cleared, too many attempts requires a new code`() = runTest(main.dispatcher) {
        auth.resetResults += failure(422, ApiErrorCodes.CODE_INVALID)
        auth.resetResults += failure(429, ApiErrorCodes.CODE_TOO_MANY_ATTEMPTS)
        auth.emailCodeResults += ApiResult.Success(CodeSent(expiresIn = 900))
        val vm = reset()
        vm.onPasswordChange("new-pass-1")

        vm.onCodeChange("000000")
        vm.submit()
        runCurrent()
        assertEquals("", vm.state.value.code)
        assertEquals("new-pass-1", vm.state.value.newPassword)
        assertEquals(R.string.error_code_invalid, vm.state.value.error?.text)

        vm.onCodeChange("111111")
        vm.submit()
        runCurrent()
        assertTrue(vm.state.value.needsNewCode)

        advanceTimeBy(60_000)
        runCurrent()
        vm.resend()
        runCurrent()
        assertFalse(vm.state.value.needsNewCode)
        assertEquals("forgot:$EMAIL", auth.emailCalls.last())
    }

    private companion object {
        const val EMAIL = "awa@poro.africa"
    }
}
