package com.poro.app.ui.profile

import androidx.lifecycle.SavedStateHandle
import com.poro.app.R
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.data.user.UsernameAvailability
import com.poro.app.testing.FakeAuthRepository.Companion.failure
import com.poro.app.testing.FakeMediaReader
import com.poro.app.testing.FakeProfileRepository
import com.poro.app.testing.MainDispatcherRule
import kotlinx.coroutines.ExperimentalCoroutinesApi
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

@OptIn(ExperimentalCoroutinesApi::class)
class EditProfileViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    private val profiles = FakeProfileRepository()
    private val media = FakeMediaReader()

    private fun TestScope.viewModel(): EditProfileViewModel {
        val vm = EditProfileViewModel(profiles, media, SavedStateHandle())
        runCurrent()
        return vm
    }

    @Test
    fun `fields start from the current profile`() = runTest(main.dispatcher) {
        val vm = viewModel()
        assertEquals("awa", vm.state.value.username)
        assertEquals("Awa", vm.state.value.displayName)
        assertEquals(UsernameStatus.Unchanged, vm.state.value.usernameStatus)
        assertTrue(vm.state.value.canSave)
    }

    @Test
    fun `a new username is checked once typing pauses`() = runTest(main.dispatcher) {
        profiles.availability["awa.kone"] = ApiResult.Success(UsernameAvailability("awa.kone", available = false, reason = "taken"))
        val vm = viewModel()

        vm.onUsernameChange("@Awa.k")
        vm.onUsernameChange("@Awa.Kone")
        assertEquals("awa.kone", vm.state.value.username)
        assertEquals(UsernameStatus.Checking, vm.state.value.usernameStatus)
        advanceTimeBy(500)
        runCurrent()

        assertEquals("une seule vérification après la pause", listOf("availability:awa.kone"), profiles.calls)
        assertEquals(UsernameStatus.Taken, vm.state.value.usernameStatus)
        assertFalse(vm.state.value.canSave)
    }

    @Test
    fun `an invalid username is refused without asking the server`() = runTest(main.dispatcher) {
        val vm = viewModel()
        vm.onUsernameChange("a")
        advanceUntilIdle()

        assertEquals(UsernameStatus.Invalid, vm.state.value.usernameStatus)
        assertTrue(profiles.calls.isEmpty())
    }

    @Test
    fun `saving reports a username taken in the meantime`() = runTest(main.dispatcher) {
        profiles.updateResults += failure(409, ApiErrorCodes.USERNAME_TAKEN)
        profiles.updateResults += ApiResult.Success(profiles.profile.copy(username = "awa2"))
        val vm = viewModel()
        vm.onUsernameChange("awa2")
        advanceUntilIdle()

        vm.save()
        runCurrent()
        assertEquals(UsernameStatus.Taken, vm.state.value.usernameStatus)

        profiles.availability["awa2"] = ApiResult.Success(UsernameAvailability("awa2", available = true))
        vm.onUsernameChange("awa2")
        advanceUntilIdle()
        vm.onBioChange("Danseuse")
        vm.save()
        assertEquals(EditProfileEvent.Saved, vm.events.first())
        assertEquals("update:awa2:Awa:Danseuse", profiles.calls.last())
    }

    @Test
    fun `avatars are checked locally before upload`() = runTest(main.dispatcher) {
        media.add("content://big", "image/jpeg", ByteArray(6 * 1024 * 1024))
        media.add("content://gif", "image/gif", ByteArray(10))
        media.add("content://ok", "image/png", ByteArray(10))
        profiles.avatarResults += ApiResult.Success(profiles.profile.copy(avatarUrl = "https://cdn/a.webp"))
        val vm = viewModel()

        vm.onAvatarPicked("content://big")
        runCurrent()
        assertEquals(R.string.error_avatar_too_large, vm.state.value.error?.text)
        vm.onAvatarPicked("content://gif")
        runCurrent()
        assertEquals(R.string.error_avatar_invalid, vm.state.value.error?.text)
        assertTrue(profiles.calls.none { it.startsWith("avatar") })

        vm.onAvatarPicked("content://ok")
        runCurrent()
        assertEquals("avatar:10:image/png", profiles.calls.last())
        assertEquals("https://cdn/a.webp", vm.state.value.avatarUrl)
        assertFalse(vm.state.value.uploadingAvatar)
    }
}
