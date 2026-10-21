package com.poro.app.ui.profile

import androidx.lifecycle.SavedStateHandle
import com.poro.app.R
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.data.auth.CodeSent
import com.poro.app.data.auth.DeletionScheduled
import com.poro.app.data.auth.UserDto
import com.poro.app.testing.FakeAccountRepository
import com.poro.app.testing.FakeAuthRepository
import com.poro.app.testing.FakeAuthRepository.Companion.failure
import com.poro.app.testing.MainDispatcherRule
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class DeleteAccountViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    private val accounts = FakeAccountRepository()
    private val auth = FakeAuthRepository()

    private fun TestScope.viewModel(): DeleteAccountViewModel {
        val vm = DeleteAccountViewModel(accounts, auth, SavedStateHandle(), clock = { testScheduler.currentTime })
        runCurrent()
        return vm
    }

    @Test
    fun `an email account confirms with its password and an explicit acknowledgement`() = runTest(main.dispatcher) {
        accounts.deleteResults += ApiResult.Success(DeletionScheduled("2026-11-20T08:00:00Z"))
        val vm = viewModel()
        assertEquals(DeletionProof.Password, vm.state.value.proof)

        vm.onPasswordChange("s3cret-pass")
        assertFalse("jamais sans la case cochée", vm.state.value.canDelete)
        vm.onAcknowledgedChange(true)
        vm.delete()

        assertEquals(DeleteAccountEvent.Scheduled("2026-11-20T08:00:00Z"), vm.events.first())
        assertEquals(listOf("s3cret-pass" to null), accounts.deletions)
    }

    @Test
    fun `a phone account requests an SMS code first`() = runTest(main.dispatcher) {
        accounts.account = UserDto(id = "u1", phone = "+2250700000000")
        auth.requestResults += ApiResult.Success(CodeSent(expiresIn = 300))
        accounts.deleteResults += failure(401, ApiErrorCodes.OTP_INVALID)
        val vm = viewModel()
        assertEquals(DeletionProof.SmsCode("+2250700000000"), vm.state.value.proof)

        vm.sendCode()
        runCurrent()
        assertTrue(vm.state.value.codeSent)
        assertEquals(60, vm.state.value.resendInSeconds)
        assertEquals(listOf("+2250700000000"), auth.requested)

        vm.onCodeChange("123456")
        vm.onAcknowledgedChange(true)
        vm.delete()
        runCurrent()
        assertEquals(listOf(null to "123456"), accounts.deletions)
        assertEquals("", vm.state.value.code)
        assertEquals(R.string.error_otp_invalid, vm.state.value.error?.text)
    }

    @Test
    fun `a locked account waits before another attempt`() = runTest(main.dispatcher) {
        accounts.deleteResults += failure(429, ApiErrorCodes.LOGIN_THROTTLED, retryAfter = 90)
        val vm = viewModel()
        vm.onPasswordChange("bad")
        vm.onAcknowledgedChange(true)

        vm.delete()
        runCurrent()

        assertEquals(90, vm.state.value.retryInSeconds)
        assertFalse(vm.state.value.canDelete)
    }
}
