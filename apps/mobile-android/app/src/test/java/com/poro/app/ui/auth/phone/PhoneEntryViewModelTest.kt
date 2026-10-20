package com.poro.app.ui.auth.phone

import androidx.lifecycle.SavedStateHandle
import com.poro.app.R
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.core.phone.LaunchCountry
import com.poro.app.data.auth.CodeSent
import com.poro.app.testing.FakeAuthRepository
import com.poro.app.testing.FakeAuthRepository.Companion.failure
import com.poro.app.testing.MainDispatcherRule
import java.io.IOException
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.advanceUntilIdle
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class PhoneEntryViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    private val auth = FakeAuthRepository()

    private fun TestScope.viewModel(saved: SavedStateHandle = SavedStateHandle()) =
        PhoneEntryViewModel(auth, saved, clock = { testScheduler.currentTime })

    @Test
    fun `submit is enabled only for a valid number of the selected country`() = runTest(main.dispatcher) {
        val vm = viewModel()
        assertEquals(LaunchCountry.CI, vm.state.value.country)

        vm.onInputChange("07 00 00 00")
        assertFalse(vm.state.value.canSubmit)
        vm.onInputChange("07 00 00 00 00")
        assertTrue(vm.state.value.canSubmit)
        vm.onCountryChange(LaunchCountry.SN)
        assertFalse("10 chiffres ne forment pas un numéro sénégalais", vm.state.value.canSubmit)
    }

    @Test
    fun `letters are filtered out of the input`() = runTest(main.dispatcher) {
        val vm = viewModel()
        vm.onInputChange("07a00 b00-00.00")
        assertEquals("0700 00-00.00", vm.state.value.input)
    }

    @Test
    fun `a sent code emits the E164 number and its lifetime`() = runTest(main.dispatcher) {
        auth.requestResults += ApiResult.Success(CodeSent(expiresIn = 300))
        val vm = viewModel()
        vm.onInputChange("07 00 00 00 00")

        vm.submit()
        assertTrue(vm.state.value.sending)
        val event = vm.events.first()

        assertEquals(PhoneEntryEvent.CodeSent("+2250700000000", 300), event)
        assertEquals(listOf("+2250700000000"), auth.requested)
        assertFalse(vm.state.value.sending)
    }

    @Test
    fun `a pasted international number switches to its country`() = runTest(main.dispatcher) {
        auth.requestResults += ApiResult.Success(CodeSent(expiresIn = 300))
        val vm = viewModel()
        vm.onInputChange("+221 77 123 45 67")

        vm.submit()
        advanceUntilIdle()

        assertEquals(LaunchCountry.SN, vm.state.value.country)
        assertEquals(listOf("+221771234567"), auth.requested)
    }

    @Test
    fun `a throttled number blocks the button for Retry-After seconds`() = runTest(main.dispatcher) {
        auth.requestResults += failure(429, ApiErrorCodes.OTP_THROTTLED, retryAfter = 30)
        val vm = viewModel()
        vm.onInputChange("0700000000")

        vm.submit()
        runCurrent()
        assertEquals(30, vm.state.value.retryInSeconds)
        assertEquals(R.string.error_otp_throttled, vm.state.value.error?.text)
        assertFalse(vm.state.value.canSubmit)

        advanceTimeBy(29_000)
        runCurrent()
        assertEquals(1, vm.state.value.retryInSeconds)
        advanceTimeBy(1_000)
        runCurrent()
        assertEquals(0, vm.state.value.retryInSeconds)
        assertTrue(vm.state.value.canSubmit)
    }

    @Test
    fun `rate limiting without Retry-After backs off exponentially`() = runTest(main.dispatcher) {
        auth.requestResults += failure(429, ApiErrorCodes.RATE_LIMITED)
        auth.requestResults += failure(429, ApiErrorCodes.RATE_LIMITED)
        val vm = viewModel()
        vm.onInputChange("0700000000")

        vm.submit()
        runCurrent()
        assertEquals(5, vm.state.value.retryInSeconds)
        advanceUntilIdle()
        vm.submit()
        runCurrent()
        assertEquals(10, vm.state.value.retryInSeconds)
    }

    @Test
    fun `server and network errors are translated`() = runTest(main.dispatcher) {
        auth.requestResults += failure(503, ApiErrorCodes.SMS_UNAVAILABLE)
        auth.requestResults += ApiResult.NetworkError(IOException("offline"))
        auth.requestResults += failure(500, ApiErrorCodes.INTERNAL_ERROR)
        val vm = viewModel()
        vm.onInputChange("0700000000")

        vm.submit()
        advanceUntilIdle()
        assertEquals(R.string.error_sms_unavailable, vm.state.value.error?.text)
        assertEquals(0, vm.state.value.retryInSeconds)

        vm.submit()
        advanceUntilIdle()
        assertEquals(R.string.error_network, vm.state.value.error?.text)

        vm.submit()
        advanceUntilIdle()
        assertEquals(R.string.error_unexpected, vm.state.value.error?.text)
        assertEquals("la référence n'accompagne que les erreurs inattendues", "req-1", vm.state.value.error?.requestId)

        vm.onInputChange("070000000")
        assertNull("une nouvelle saisie efface l'erreur", vm.state.value.error)
    }

    @Test
    fun `country, input and wait survive process death`() = runTest(main.dispatcher) {
        auth.requestResults += failure(429, ApiErrorCodes.OTP_THROTTLED, retryAfter = 30)
        val saved = SavedStateHandle()
        val first = viewModel(saved)
        first.onCountryChange(LaunchCountry.CM)
        first.onInputChange("690123456")
        first.submit()
        advanceTimeBy(10_000)
        runCurrent()

        val restored = viewModel(saved)
        runCurrent()

        assertEquals(LaunchCountry.CM, restored.state.value.country)
        assertEquals("690123456", restored.state.value.input)
        assertEquals(20, restored.state.value.retryInSeconds)
    }
}
