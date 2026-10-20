package com.poro.app.ui.auth.otp

import androidx.lifecycle.SavedStateHandle
import com.poro.app.R
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.data.auth.CodeSent
import com.poro.app.testing.FakeAuthRepository
import com.poro.app.testing.FakeAuthRepository.Companion.failure
import com.poro.app.testing.FakeAuthRepository.Companion.signedIn
import com.poro.app.testing.MainDispatcherRule
import java.io.IOException
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test

class OtpVerifyViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    private val auth = FakeAuthRepository()

    private fun TestScope.viewModel(saved: SavedStateHandle = SavedStateHandle()) =
        OtpVerifyViewModel(PHONE, expiresInSeconds = 300, auth, saved, clock = { testScheduler.currentTime })

    @Test
    fun `the resend countdown starts at 60 seconds and the code expiry at its lifetime`() = runTest(main.dispatcher) {
        val vm = viewModel()
        runCurrent()
        assertEquals(60, vm.state.value.resendInSeconds)
        assertEquals(300, vm.state.value.expiresInSeconds)
        assertFalse(vm.state.value.canResend)

        advanceTimeBy(60_000)
        runCurrent()
        assertEquals(0, vm.state.value.resendInSeconds)
        assertEquals(240, vm.state.value.expiresInSeconds)
        assertTrue(vm.state.value.canResend)
    }

    @Test
    fun `the sixth digit submits the code automatically and signs in`() = runTest(main.dispatcher) {
        auth.verifyResults += signedIn(deletionCancelled = true)
        val vm = viewModel()

        vm.onCodeChange("12345")
        assertTrue(auth.verified.isEmpty())
        vm.onCodeChange("123456")
        assertTrue(vm.state.value.verifying)

        assertEquals(OtpEvent.SignedIn(deletionCancelled = true), vm.events.first())
        assertEquals(listOf(PHONE to "123456"), auth.verified)
    }

    @Test
    fun `a pasted or autofilled code keeps only six digits`() = runTest(main.dispatcher) {
        auth.verifyResults += signedIn()
        val vm = viewModel()

        vm.onCodeChange("Code : 654 321 99")
        advanceUntilIdle()

        assertEquals(listOf(PHONE to "654321"), auth.verified)
    }

    @Test
    fun `a wrong code clears the field and can be retyped`() = runTest(main.dispatcher) {
        auth.verifyResults += failure(401, ApiErrorCodes.OTP_INVALID)
        auth.verifyResults += signedIn()
        val vm = viewModel()

        vm.onCodeChange("000000")
        advanceUntilIdle()
        assertEquals("", vm.state.value.code)
        assertEquals(R.string.error_otp_invalid, vm.state.value.error?.text)
        assertFalse(vm.state.value.needsNewCode)

        vm.onCodeChange("123456")
        assertEquals(OtpEvent.SignedIn(deletionCancelled = false), vm.events.first())
    }

    @Test
    fun `too many attempts requires a new code before verifying again`() = runTest(main.dispatcher) {
        auth.verifyResults += failure(429, ApiErrorCodes.OTP_TOO_MANY_ATTEMPTS)
        auth.requestResults += ApiResult.Success(CodeSent(expiresIn = 300))
        val vm = viewModel()

        vm.onCodeChange("111111")
        runCurrent()
        assertTrue(vm.state.value.needsNewCode)
        assertEquals(R.string.error_otp_too_many_attempts, vm.state.value.error?.text)

        vm.onCodeChange("222222")
        runCurrent()
        assertEquals("pas de nouvel essai avec un code épuisé", 1, auth.verified.size)

        advanceTimeBy(60_000)
        runCurrent()
        vm.resend()
        runCurrent()
        assertFalse(vm.state.value.needsNewCode)
        assertTrue(vm.state.value.codeResent)
        assertEquals("", vm.state.value.code)
        assertEquals(60, vm.state.value.resendInSeconds)
        assertEquals(listOf(PHONE), auth.requested)
    }

    @Test
    fun `an expired code also requires a new one`() = runTest(main.dispatcher) {
        auth.verifyResults += failure(401, ApiErrorCodes.OTP_EXPIRED)
        val vm = viewModel()

        vm.onCodeChange("123456")
        runCurrent()

        assertTrue(vm.state.value.needsNewCode)
        assertEquals(R.string.error_otp_expired, vm.state.value.error?.text)
    }

    @Test
    fun `resend is ignored during the cooldown`() = runTest(main.dispatcher) {
        val vm = viewModel()
        runCurrent()

        vm.resend()
        runCurrent()

        assertTrue(auth.requested.isEmpty())
    }

    @Test
    fun `a throttled resend waits for Retry-After`() = runTest(main.dispatcher) {
        auth.requestResults += failure(429, ApiErrorCodes.OTP_THROTTLED, retryAfter = 120)
        val vm = viewModel()
        advanceTimeBy(60_000)
        runCurrent()

        vm.resend()
        runCurrent()

        assertEquals(120, vm.state.value.resendInSeconds)
        assertEquals(R.string.error_otp_throttled, vm.state.value.error?.text)
    }

    @Test
    fun `offline verification keeps the code for a manual retry`() = runTest(main.dispatcher) {
        auth.verifyResults += ApiResult.NetworkError(IOException("offline"))
        auth.verifyResults += signedIn()
        val vm = viewModel()

        vm.onCodeChange("123456")
        runCurrent()
        assertEquals("123456", vm.state.value.code)
        assertEquals(R.string.error_network, vm.state.value.error?.text)
        assertTrue(vm.state.value.canVerify)

        vm.verify()
        assertEquals(OtpEvent.SignedIn(deletionCancelled = false), vm.events.first())
    }

    @Test
    fun `deadlines survive process death`() = runTest(main.dispatcher) {
        val saved = SavedStateHandle()
        viewModel(saved)
        advanceTimeBy(45_000)
        runCurrent()

        val restored = viewModel(saved)
        runCurrent()

        assertEquals(15, restored.state.value.resendInSeconds)
        assertEquals(255, restored.state.value.expiresInSeconds)
    }

    private companion object {
        const val PHONE = "+2250700000000"
    }
}
