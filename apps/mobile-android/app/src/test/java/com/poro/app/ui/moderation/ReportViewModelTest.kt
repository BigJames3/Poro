package com.poro.app.ui.moderation

import androidx.lifecycle.SavedStateHandle
import com.poro.app.R
import com.poro.app.core.network.ApiErrorCodes
import com.poro.app.core.network.ApiResult
import com.poro.app.data.moderation.ModerationRepository
import com.poro.app.data.moderation.ReportDto
import com.poro.app.data.moderation.ReportReason
import com.poro.app.data.moderation.ReportTargetType
import com.poro.app.testing.MainDispatcherRule
import java.io.IOException
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class ReportViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    private val calls = mutableListOf<String>()
    private var result: ApiResult<ReportDto> = ApiResult.Success(ReportDto("r1", "c1", created = true))
    private val repo = object : ModerationRepository {
        override suspend fun report(type: ReportTargetType, targetId: String, reason: ReportReason, comment: String?): ApiResult<ReportDto> {
            calls += "${type.wire}:$targetId:${reason.wire}:$comment"
            return result
        }
    }
    private val target = ReportTarget(ReportTargetType.Video, "v1")

    @Test
    fun `a reason is required before sending`() = runTest {
        val vm = ReportViewModel(target, repo, SavedStateHandle())

        vm.send()
        runCurrent()

        assertFalse(vm.state.value.canSend)
        assertTrue(calls.isEmpty())
    }

    @Test
    fun `sending reports the target and tells whether it is new`() = runTest {
        val vm = ReportViewModel(target, repo, SavedStateHandle())
        vm.selectReason(ReportReason.Spam)
        vm.onDetailChange("Publicité")

        vm.send()

        assertEquals(ReportEvent.Sent(created = true), vm.events.first())
        assertEquals(listOf("video:v1:spam:Publicité"), calls)

        result = ApiResult.Success(ReportDto("r1", "c1", created = false))
        vm.send()
        assertEquals(ReportEvent.Sent(created = false), vm.events.first())
    }

    @Test
    fun `the detail is capped and the input survives process death`() = runTest {
        val saved = SavedStateHandle()
        val first = ReportViewModel(target, repo, saved)
        first.selectReason(ReportReason.Fraud)
        first.onDetailChange("x".repeat(600))

        val restored = ReportViewModel(target, repo, saved)

        assertEquals(ReportReason.Fraud, restored.state.value.reason)
        assertEquals(500, restored.state.value.detail.length)
    }

    @Test
    fun `failures keep the input and explain the limit`() = runTest {
        val vm = ReportViewModel(target, repo, SavedStateHandle())
        vm.selectReason(ReportReason.Hate)

        result = ApiResult.NetworkError(IOException("offline"))
        vm.send()
        runCurrent()
        assertEquals(R.string.error_network, vm.state.value.error?.text)
        assertEquals(ReportReason.Hate, vm.state.value.reason)

        result = ApiResult.Failure(429, ApiErrorCodes.RATE_LIMITED, "slow down", null, 600)
        vm.send()
        runCurrent()
        assertEquals(10L, vm.state.value.retryInMinutes)

        result = ApiResult.Failure(422, ApiErrorCodes.CANNOT_REPORT_SELF, "self", null, null)
        vm.send()
        assertEquals(ReportEvent.OwnContent, vm.events.first())
    }
}
