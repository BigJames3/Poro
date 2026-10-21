package com.poro.app.ui.moderation

import androidx.lifecycle.SavedStateHandle
import com.poro.app.R
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.data.moderation.ReportDto
import com.poro.app.testing.FakeModerationRepository
import com.poro.app.testing.MainDispatcherRule
import java.io.IOException
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
class ReportViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    private val reports = FakeModerationRepository()

    private fun TestScope.viewModel(
        type: String = "video",
        id: String = "v1",
        saved: SavedStateHandle = SavedStateHandle(),
    ) = ReportViewModel(type, id, reports, saved, clock = { testScheduler.currentTime })

    @Test
    fun `only the seven contract reasons exist`() {
        assertEquals(listOf("spam", "nudity", "violence", "harassment", "hate", "fraud", "other"), ReportReason.entries.map { it.wire })
    }

    @Test
    fun `submit is disabled until a reason is chosen`() = runTest(main.dispatcher) {
        val vm = viewModel()
        assertFalse(vm.state.value.canSubmit)
        vm.submit()
        runCurrent()
        assertTrue(reports.calls.isEmpty())

        vm.onReason(ReportReason.Spam)
        assertTrue(vm.state.value.canSubmit)
        vm.submit()
        assertEquals(ReportEvent.Submitted, vm.events.first())
        assertEquals(listOf("report:video:v1:spam:"), reports.calls)
    }

    @Test
    fun `a duplicate report is not an error`() = runTest(main.dispatcher) {
        reports.result = ApiResult.Success(ReportDto("r1", "c1", created = false))
        val vm = viewModel("comment", "c9")
        vm.onReason(ReportReason.Harassment)
        vm.onCommentChange("détail")
        vm.submit()
        assertEquals(ReportEvent.AlreadyReported, vm.events.first())
        assertNull(vm.state.value.error)
        assertEquals(listOf("report:comment:c9:harassment:détail"), reports.calls)
    }

    @Test
    fun `reporting yourself shows the dedicated message`() = runTest(main.dispatcher) {
        reports.result = ApiResult.Failure(422, ApiErrorCodes.CANNOT_REPORT_SELF, "self", null, null)
        val vm = viewModel("user", "me")
        vm.onReason(ReportReason.Other)
        vm.submit()
        runCurrent()
        assertEquals(R.string.error_cannot_report_self, vm.state.value.error?.text)
        assertTrue(vm.state.value.canSubmit)
    }

    @Test
    fun `detail cannot exceed 500 characters`() = runTest(main.dispatcher) {
        val vm = viewModel()
        vm.onCommentChange("x".repeat(501))
        assertEquals(500, vm.state.value.comment.length)
    }

    @Test
    fun `a rate limit waits for Retry-After`() = runTest(main.dispatcher) {
        reports.result = ApiResult.Failure(429, ApiErrorCodes.RATE_LIMITED, "wait", null, 5)
        val vm = viewModel()
        vm.onReason(ReportReason.Fraud)
        vm.submit()
        runCurrent()
        assertEquals(5, vm.state.value.retryInSeconds)
        assertFalse(vm.state.value.canSubmit)
        assertEquals(R.string.error_rate_limited, vm.state.value.error?.text)

        advanceTimeBy(5_000)
        runCurrent()
        assertTrue(vm.state.value.canSubmit)
    }

    @Test
    fun `a network error keeps the chosen reason`() = runTest(main.dispatcher) {
        reports.result = ApiResult.NetworkError(IOException())
        val vm = viewModel()
        vm.onReason(ReportReason.Nudity)
        vm.submit()
        runCurrent()
        assertEquals(ReportReason.Nudity, vm.state.value.reason)
        assertEquals(R.string.error_network, vm.state.value.error?.text)
        assertTrue(vm.state.value.canSubmit)
    }
}
