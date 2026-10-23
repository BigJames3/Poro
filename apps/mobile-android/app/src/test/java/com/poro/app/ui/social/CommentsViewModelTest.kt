package com.poro.app.ui.social

import com.poro.app.core.network.ApiResult
import com.poro.app.core.network.PageDto
import com.poro.app.data.social.CommentDto
import com.poro.app.testing.FakeProfileRepository
import com.poro.app.testing.FakeSocialRepository
import com.poro.app.testing.MainDispatcherRule
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Rule
import org.junit.Test

@OptIn(ExperimentalCoroutinesApi::class)
class CommentsViewModelTest {

    @get:Rule
    val main = MainDispatcherRule()

    private val social = FakeSocialRepository()
    private val profiles = FakeProfileRepository()

    private fun comment(id: String, user: String = "u2", parent: String? = null, replies: Long = 0) =
        CommentDto(id = id, videoId = "v1", userId = user, parentId = parent, content = "texte $id", repliesCount = replies)

    private fun page(vararg comments: CommentDto, next: String? = null) = ApiResult.Success(PageDto(comments.toList(), next))

    @Test
    fun `comments are paged and my own are recognised`() = runTest(main.dispatcher) {
        social.commentPages += page(comment("c1"), comment("c2", user = "u1"), next = "n2")
        social.commentPages += page(comment("c3"))
        val vm = CommentsViewModel("v1", social, profiles)
        runCurrent()

        assertEquals("u1", vm.state.value.myUserId)
        assertEquals(listOf("c1", "c2"), vm.state.value.comments.map { it.id })
        vm.loadMore()
        runCurrent()
        assertEquals(listOf("c1", "c2", "c3"), vm.state.value.comments.map { it.id })
        assertTrue(vm.state.value.endReached)
    }

    @Test
    fun `a new comment goes on top and is counted`() = runTest(main.dispatcher) {
        social.commentPages += page(comment("c1"))
        social.postResults += ApiResult.Success(comment("c9", user = "u1"))
        val vm = CommentsViewModel("v1", social, profiles)
        runCurrent()

        vm.onDraftChange("  Trop beau 🔥 ")
        vm.post()

        assertEquals(CommentsEvent.CountChanged(+1), vm.events.first())
        assertEquals(listOf("c9", "c1"), vm.state.value.comments.map { it.id })
        assertEquals("", vm.state.value.draft)
        assertEquals("post:  Trop beau 🔥 :null", social.calls.last())
    }

    @Test
    fun `a reply to a reply goes to its top-level comment`() = runTest(main.dispatcher) {
        social.commentPages += page(comment("c1", replies = 1))
        social.replyPages += page(comment("r1", parent = "c1"))
        social.postResults += ApiResult.Success(comment("r2", user = "u1", parent = "c1"))
        val vm = CommentsViewModel("v1", social, profiles)
        runCurrent()

        vm.toggleReplies("c1")
        runCurrent()
        vm.replyTo(vm.state.value.replies.getValue("c1").items.single())
        vm.onDraftChange("d'accord")
        vm.post()
        runCurrent()

        assertEquals("post:d'accord:c1", social.calls.last())
        assertEquals(listOf("r1", "r2"), vm.state.value.replies.getValue("c1").items.map { it.id })
        assertEquals(2L, vm.state.value.comments.single().repliesCount)
    }

    @Test
    fun `an own comment can be edited then marked edited`() = runTest(main.dispatcher) {
        val created = "2026-10-22T04:00:00Z"
        social.commentPages += page(comment("c1", user = "u1").copy(createdAt = created))
        social.editResults += ApiResult.Success(comment("c1", user = "u1").copy(content = "corrigé", edited = true, createdAt = created))
        val vm = CommentsViewModel("v1", social, profiles)
        runCurrent()

        vm.startEdit(vm.state.value.comments.single(), nowMs = com.poro.app.core.format.Formats.parseIsoMillis(created)!! + 1_000)
        vm.onEditDraftChange("corrigé")
        vm.saveEdit()
        runCurrent()

        assertEquals("edit:c1:corrigé", social.calls.last())
        assertTrue(vm.state.value.comments.single().edited)
        assertEquals(null, vm.state.value.editing)
    }

    @Test
    fun `a deleted comment with replies stays as an empty shell`() = runTest(main.dispatcher) {
        social.commentPages += page(comment("c1", user = "u1", replies = 2), comment("c2", user = "u1"))
        val vm = CommentsViewModel("v1", social, profiles)
        runCurrent()

        vm.delete(vm.state.value.comments[0])
        vm.delete(vm.state.value.comments[1])
        runCurrent()

        val remaining = vm.state.value.comments.single()
        assertEquals("c1", remaining.id)
        assertTrue(remaining.deleted)
        assertEquals(null, remaining.content)
    }
}
