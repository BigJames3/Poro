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
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class EmailRegisterViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    private val auth = FakeAuthRepository()

    private fun TestScope.viewModel() = EmailRegisterViewModel(auth, SavedStateHandle(), clock = { testScheduler.currentTime })

    @Test
    fun `every field must be valid before submitting`() = runTest(main.dispatcher) {
        val vm = viewModel()
        vm.onFullNameChange("Awa Koné")
        vm.onEmailChange("awa@poro.africa")
        vm.onPasswordChange("1234567")
        assertFalse("7 caractères ne suffisent pas", vm.state.value.canSubmit)

        vm.onPasswordChange("12345678")
        assertTrue(vm.state.value.canSubmit)
        vm.onFullNameChange("A")
        assertFalse(vm.state.value.canSubmit)
    }

    @Test
    fun `registers with trimmed name and normalized address`() = runTest(main.dispatcher) {
        auth.signInResults += signedIn()
        val vm = viewModel()
        vm.onFullNameChange("  Awa Koné ")
        vm.onEmailChange("Awa@Poro.Africa")
        vm.onPasswordChange("s3cret-pass")

        vm.submit()

        assertEquals(EmailRegisterEvent.Registered, vm.events.first())
        assertEquals(listOf("register:awa@poro.africa:Awa Koné"), auth.emailCalls)
    }

    @Test
    fun `a taken address is reported`() = runTest(main.dispatcher) {
        auth.signInResults += failure(409, ApiErrorCodes.EMAIL_TAKEN)
        val vm = viewModel()
        vm.onFullNameChange("Awa Koné")
        vm.onEmailChange("awa@poro.africa")
        vm.onPasswordChange("s3cret-pass")

        vm.submit()
        runCurrent()

        assertEquals(R.string.error_email_taken, vm.state.value.error?.text)
        assertFalse(vm.state.value.registering)
    }
}
